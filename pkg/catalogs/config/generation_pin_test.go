package config_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/runtime"
)

func TestGenerationPinConfigurationSelectsRetainedCatalog(t *testing.T) {
	store := storage.NewMemory()
	writer, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pin-selected", "pin-newer"} {
		_, err := writer.Update(t.Context(), func(_ context.Context, catalog *catalogs.Catalog) (*starmap.Candidate, error) {
			return starmap.NewCandidate(catalog, starmap.CandidateEvidence{}, starmap.WithCandidateGenerationID(id))
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	selected, err := store.Get(t.Context(), "pin-selected")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"STARMAP_CATALOG_GENERATION_PIN": "pin-selected",
		config.SourceRefreshMode:         "manual",
		config.SourcePollInterval:        "0s",
		config.AcquisitionEnabled:        "false",
		config.StateDirectory:            filepath.Join(t.TempDir(), "runtime"),
	}
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	probe := &controlWatcherProbe{controlNetworkProbe: controlNetworkProbe{client: server.Client(), address: server.URL}, changes: make(chan struct{}, 1)}
	for attempt := range 4 {
		if attempt >= 2 {
			values[config.SourceRefreshMode] = "automatic"
			values[config.AcquisitionEnabled] = "true"
		}
		parsed, err := config.Load(func(name string) (string, bool) { value, found := values[name]; return value, found })
		if err != nil {
			t.Fatal(err)
		}
		options := append(parsed.Options(), runtime.WithClientOptions(starmap.WithCatalogStore(store)), runtime.WithSource(probe), runtime.WithAcquirer(probe))
		connected, err := runtime.Open(t.Context(), options...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := connected.Close(); err != nil {
				t.Error(err)
			}
		})
		state := connected.State()
		if state.GenerationID != selected.Manifest.GenerationID || state.PayloadChecksum != selected.Manifest.Payload.Checksum {
			t.Errorf("attempt %d selected %q, want retained generation %q", attempt, state.GenerationID, selected.Manifest.GenerationID)
		}
		if connected.Client().CurrentGenerationID() != state.GenerationID {
			t.Error("runtime and publication client disagree")
		}
		if _, err := connected.RefreshSource(t.Context()); err == nil {
			t.Error("pin allowed explicit source refresh")
		}
		called := false
		if _, err := connected.Client().Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
			called = true
			return nil, nil
		}); err == nil || called {
			t.Error("pin allowed direct candidate work")
		}
		if _, err := connected.Client().Rollback(t.Context(), "pin-newer"); err == nil {
			t.Error("direct rollback bypassed the pin")
		}
		if probe.subscriptions.Load() != 0 {
			t.Error("pin subscribed to source watcher")
		}
		if requests.Load() != 0 {
			t.Error("pin allowed source network access")
		}
		if err := connected.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGenerationPinDescriptorKeepsConfigurationAuthority(t *testing.T) {
	parsed, err := config.Parse(map[string]string{config.GenerationPin: ""})
	if err != nil {
		t.Fatal(err)
	}
	if value, present := parsed.Value(config.GenerationPin); !present || value != "" || parsed.GenerationPin != "" {
		t.Fatal("explicit unpin lost presence")
	}
	for _, descriptor := range config.Descriptors() {
		if descriptor.Name == config.GenerationPin {
			if descriptor.Scope != config.DeploymentScope || descriptor.Mutability != "runtime-replacement" || !descriptor.AllowEmpty {
				t.Fatal("pin has the wrong configuration authority or replacement contract")
			}
			return
		}
	}
	t.Fatal("pin descriptor is missing")
}
