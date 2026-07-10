package starmap

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestWritableStoreTriggerMatrixRejectsBeforeWork(t *testing.T) {
	t.Run("manual sync before provider fetch", func(t *testing.T) {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
		}))
		defer server.Close()

		outputPath := t.TempDir()
		seed := catalogs.NewEmpty()
		if err := seed.SetProvider(catalogs.Provider{
			ID:   "preflight-provider",
			Name: "Preflight Provider",
			Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				URL:  server.URL,
			}},
		}); err != nil {
			t.Fatalf("Seed provider: %v", err)
		}
		if err := seed.Save(save.WithPath(outputPath)); err != nil {
			t.Fatalf("Save seed catalog: %v", err)
		}

		client := newWritableStoreTestClient(t, defaults())
		err := syncError(client.Sync(
			context.Background(),
			pkgsync.WithOutputPath(outputPath),
			pkgsync.WithSources(sources.LocalCatalogID, sources.ProvidersID),
		))
		assertWritableStoreConfigError(t, err)
		if got := requests.Load(); got != 0 {
			t.Fatalf("provider requests = %d, want 0", got)
		}
	})

	t.Run("custom update before callback", func(t *testing.T) {
		var calls atomic.Int32
		opts := defaults()
		opts.updateFunc = func(_ context.Context, catalog *catalogs.Builder) (*catalogs.Builder, error) {
			calls.Add(1)
			return catalog, nil
		}
		client := newWritableStoreTestClient(t, opts)

		assertWritableStoreConfigError(t, client.Update(context.Background()))
		if got := calls.Load(); got != 0 {
			t.Fatalf("custom update calls = %d, want 0", got)
		}
	})

	t.Run("remote update before request", func(t *testing.T) {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		opts := defaults()
		opts.remoteServerOnly = true
		opts.remoteServerURL = &server.URL
		client := newWritableStoreTestClient(t, opts)

		assertWritableStoreConfigError(t, client.Update(context.Background()))
		if got := requests.Load(); got != 0 {
			t.Fatalf("remote requests = %d, want 0", got)
		}
	})

}

func TestWritableStoreAllowsConfiguredMutationTriggers(t *testing.T) {
	t.Run("manual sync", func(t *testing.T) {
		outputPath := t.TempDir()
		seed := catalogs.NewEmpty()
		if err := seed.SetProvider(catalogs.Provider{ID: "local-provider", Name: "Local Provider"}); err != nil {
			t.Fatalf("Seed provider: %v", err)
		}
		if err := seed.Save(save.WithPath(outputPath)); err != nil {
			t.Fatalf("Save seed catalog: %v", err)
		}

		opts := defaults()
		opts.catalogStore = catalogstore.NewMemory()
		client := newWritableStoreTestClient(t, opts)
		if _, err := client.Sync(
			context.Background(),
			pkgsync.WithOutputPath(outputPath),
			pkgsync.WithSources(sources.LocalCatalogID),
			pkgsync.WithReformat(true),
		); err != nil {
			t.Fatalf("Sync: %v", err)
		}
	})

	t.Run("custom update", func(t *testing.T) {
		var calls atomic.Int32
		opts := defaults()
		opts.catalogStore = catalogstore.NewMemory()
		opts.updateFunc = func(_ context.Context, catalog *catalogs.Builder) (*catalogs.Builder, error) {
			calls.Add(1)
			return catalog, nil
		}
		client := newWritableStoreTestClient(t, opts)
		if err := client.Update(context.Background()); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if got := calls.Load(); got != 1 {
			t.Fatalf("custom update calls = %d, want 1", got)
		}
	})

	t.Run("remote update", func(t *testing.T) {
		remote := mustTestCatalog(t, catalogs.NewEmpty())
		observedAt := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
		observation, err := sources.NewObservation(sources.LocalCatalogID, remote, sources.ObservationMetadata{
			ObservedAt: observedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		})
		if err != nil {
			t.Fatalf("NewObservation: %v", err)
		}
		generation, err := generationTestClient(observedAt).newGeneration(remote, []sources.Observation{observation})
		if err != nil {
			t.Fatalf("newGeneration: %v", err)
		}
		manifest, err := catalogremote.MarshalManifest(generation.Manifest)
		if err != nil {
			t.Fatalf("MarshalManifest: %v", err)
		}
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			switch r.URL.Path {
			case catalogremote.ManifestPath:
				w.Header().Set("Content-Type", catalogremote.ManifestMediaType)
				_, _ = w.Write(manifest)
			case catalogremote.SnapshotPath(generation.Manifest.GenerationID):
				w.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
				_, _ = w.Write(generation.Payload)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		opts := defaults()
		opts.catalogStore = catalogstore.NewMemory()
		opts.remoteServerOnly = true
		opts.remoteServerURL = &server.URL
		client := newWritableStoreTestClient(t, opts)
		if err := client.Update(context.Background()); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if got := requests.Load(); got != 2 {
			t.Fatalf("remote requests = %d, want 2", got)
		}
	})

}

func TestWithCatalogStoreRejectsNilImplementations(t *testing.T) {
	var typedNil *catalogstore.Memory
	for name, store := range map[string]catalogstore.Store{
		"nil interface": nil,
		"typed nil":     typedNil,
	} {
		t.Run(name, func(t *testing.T) {
			client, err := New(WithCatalogStore(store))
			if client != nil {
				t.Fatal("New returned client with nil catalog store")
			}
			assertWritableStoreConfigError(t, err)
		})
	}
}

func assertWritableStoreConfigError(t testing.TB, err error) {
	t.Helper()
	var configErr *pkgerrors.ConfigError
	if !stderrors.As(err, &configErr) {
		t.Fatalf("error = %T: %v, want *errors.ConfigError", err, err)
	}
	if configErr.Component != "catalog store" {
		t.Fatalf("ConfigError component = %q, want catalog store", configErr.Component)
	}
}

func syncError(_ *pkgsync.Result, err error) error {
	return err
}

func newWritableStoreTestClient(t testing.TB, opts *options) *Client {
	t.Helper()
	return &Client{
		options: opts,
		catalog: mustTestCatalog(t, catalogs.NewEmpty()),
		hooks:   newHooks(),
	}
}
