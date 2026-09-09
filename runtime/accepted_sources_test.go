package runtime

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcceptedSourcesSurviveFailureAndRespectRestartPolicy(t *testing.T) {
	directory := privateRuntimeDirectory(t)
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	calls := 0
	collector := sourceAcquirerFunc(func(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
		calls++
		return nil, &pkgerrors.ConfigError{Component: "fixture", Message: "unavailable"}
	})
	open := func(selected sources.ID) *Runtime {
		return openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionSources(selected), WithSourceAcquirer(collector), WithClientOptions(starmap.WithCatalogStore(store)))
	}
	check := func(r *Runtime, want ...sources.ID) {
		t.Helper()
		raw, err := json.Marshal(r.Status())
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Accepted []sources.ID `json:"accepted_acquisition_sources"`
		}
		if err := json.Unmarshal(raw, &report); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(report.Accepted, want) {
			t.Fatalf("accepted sources=%v, want %v", report.Accepted, want)
		}
	}
	first := open(sources.LocalCatalogID)
	check(first)
	observation := acquisitionSourceObservation(t, false)
	store.reject.Store(true)
	if _, err := first.PublishObservations(t.Context(), observation); err == nil {
		t.Fatal("rejected publication succeeded")
	}
	check(first)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	store.reject.Store(false)
	first = open(sources.LocalCatalogID)
	check(first)
	if _, err := first.PublishObservations(t.Context(), observation); err != nil {
		t.Fatal(err)
	}
	check(first, sources.LocalCatalogID)
	snapshot := first.Status()
	snapshot.AcceptedAcquisitionSources[0] = sources.ProvidersID
	check(first, sources.LocalCatalogID)
	generation := first.State().GenerationID
	if _, err := first.Sync(t.Context()); err == nil {
		t.Fatal("failed acquisition reported success")
	}
	check(first, sources.LocalCatalogID)
	if first.State().GenerationID != generation || calls != 1 {
		t.Fatal("failed refresh replaced accepted state")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	retained := open(sources.LocalCatalogID)
	check(retained, sources.LocalCatalogID)
	if calls != 1 {
		t.Fatal("status or startup contacted acquisition")
	}
	if err := retained.Close(); err != nil {
		t.Fatal(err)
	}
	revoked := open(sources.ProvidersID)
	check(revoked)
	if calls != 1 {
		t.Fatal("revoked source ran during startup")
	}
}
