package acquisition

import (
	"context"
	stderrors "errors"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

func TestManualPreflightFailureReportsUnattemptedSources(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "read-only store"
		if canceled {
			name = "canceled before publication"
		}
		t.Run(name, func(t *testing.T) {
			var options []starmap.Option
			if canceled {
				options = append(options, starmap.WithCatalogStore(storage.NewMemory()))
			}
			client, err := starmap.New(options...)
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			syncer, err := New(client, WithProviderClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
				calls.Add(1)
				return testProviderClient{}, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if canceled {
				cancel()
			}
			before := client.Catalog()
			beforeID := client.CurrentGenerationID()
			result, err := syncer.Sync(ctx, pkgsync.WithSources(sources.ProvidersID), pkgsync.WithProvider("openai"))
			if result != nil || err == nil {
				t.Fatalf("preflight result=%+v, error=%v", result, err)
			}
			if canceled {
				if !stderrors.Is(err, context.Canceled) {
					t.Fatalf("cancellation identity lost: %v", err)
				}
			} else {
				var configErr *pkgerrors.ConfigError
				if !stderrors.As(err, &configErr) || configErr.Component != "catalog store" {
					t.Fatalf("store error identity lost: %v", err)
				}
			}
			if calls.Load() != 0 || client.Catalog() != before || client.CurrentGenerationID() != beforeID {
				t.Fatal("preflight failure acquired or published catalog data")
			}
			activity := sources.ActivityFromError(err)
			if len(activity) != 4 {
				t.Fatalf("preflight source activity=%+v, want four source rows", activity)
			}
			seen := make(map[sources.ID]bool)
			for _, row := range activity {
				if !row.Valid() || seen[row.Source] || !row.Supported || row.SelectionUnknown || row.Enabled != (row.Source == sources.ProvidersID) || row.Attempted || row.Eligibility != sources.EligibilityUnknown {
					t.Fatalf("invalid preflight activity=%+v", row)
				}
				seen[row.Source] = true
			}
			if len(sources.ProviderAttemptsFromError(err)) != 0 || sources.AcceptedSourcesFromError(err) != nil {
				t.Fatal("preflight failure invented provider attempts or retained runtime input")
			}
		})
	}
}

func TestManualRuntimePreflightReportsSelectedSources(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		selected  []sources.ID
		requested []sources.ID
		canceled  bool
	}{
		{name: "disabled"},
		{name: "excluded", selected: []sources.ID{sources.LocalCatalogID}, requested: []sources.ID{sources.ProvidersID}},
		{name: "canceled inherited selection", selected: []sources.ID{sources.LocalCatalogID}, canceled: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			connected, err := runtime.Open(t.Context(), runtime.WithCatalogSource("embedded"), runtime.WithAcquisitionEnabled(false), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionSources(scenario.selected...), runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = connected.Close() })
			var calls atomic.Int32
			syncer, err := NewForRuntime(connected, WithProviderClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
				calls.Add(1)
				return testProviderClient{}, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			before := connected.State()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if scenario.canceled {
				cancel()
			}
			result, err := syncer.Sync(ctx, pkgsync.WithSources(scenario.requested...))
			if result != nil || err == nil {
				t.Fatalf("preflight result=%+v, error=%v", result, err)
			}
			if scenario.canceled && !stderrors.Is(err, context.Canceled) {
				t.Fatalf("cancellation identity lost: %v", err)
			}
			activity := sources.ActivityFromError(err)
			if len(activity) != 4 {
				t.Fatalf("activity=%+v, want four rows", activity)
			}
			for _, row := range activity {
				wantEnabled := len(scenario.selected) != 0 && row.Source == sources.LocalCatalogID
				if !row.Valid() || !row.Supported || row.SelectionUnknown || row.Enabled != wantEnabled || row.Attempted || row.Eligibility != sources.EligibilityUnknown {
					t.Fatalf("preflight activity=%+v, enabled want %v", row, wantEnabled)
				}
			}
			accepted := sources.AcceptedSourcesFromError(err)
			if accepted == nil || accepted.GenerationID != before.GenerationID || len(accepted.Sources) != 0 {
				t.Fatalf("accepted input changed: %+v", accepted)
			}
			after := connected.State()
			if calls.Load() != 0 || after.Catalog != before.Catalog || after.GenerationID != before.GenerationID {
				t.Fatal("preflight failure acquired or published catalog data")
			}
		})
	}
}
