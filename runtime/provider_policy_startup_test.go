package runtime

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/server"
)

func TestStartupWithoutInputsRefusesWithdrawnProviderBinding(t *testing.T) {
	for _, opaque := range []bool{false, true} {
		t.Run(map[bool]string{false: "derived identity", true: "opaque identity"}[opaque], func(t *testing.T) {
			store, _ := acceptedScopedStartupStore(t)
			if opaque {
				generation, err := store.Current(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				expected := generation.Manifest.GenerationID
				generation.Manifest.GenerationID = "imported-scoped-catalog"
				if err := store.Commit(t.Context(), generation, expected); err != nil {
					t.Fatal(err)
				}
			}
			before, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			next, err := Open(t.Context(), WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)), WithAcquisitionEnabled(false), WithSourcePollInterval(0))
			if next != nil {
				if _, providerErr := next.State().Catalog.Provider("provider"); providerErr == nil {
					t.Error("withdrawn private provider remains available")
				}
				_ = next.Close()
			}
			var conflict *pkgerrors.ConflictError
			if !errors.As(err, &conflict) || next != nil {
				t.Fatalf("Open error=%v, want typed refusal without a runtime", err)
			}
			after, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if before.Manifest.GenerationID != after.Manifest.GenerationID || before.Manifest.Payload.Checksum != after.Manifest.Payload.Checksum {
				t.Fatal("refusal changed accepted stored state")
			}
		})
	}
}

func TestStartupRemovedBindingAlignsClientWithRetainedInputs(t *testing.T) {
	store, directory := acceptedScopedStartupStore(t)
	next := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)))
	if _, err := next.State().Catalog.Provider("provider"); err == nil {
		t.Error("withdrawn provider remains in runtime")
	}
	if _, err := next.Client().CurrentCatalogState().Catalog.Provider("provider"); err == nil {
		t.Error("withdrawn provider remains in client")
	}
	retained, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if retained.Manifest.GenerationID != next.State().GenerationID || retained.Manifest.Payload.Checksum != next.State().PayloadChecksum {
		t.Error("stored current differs from permitted runtime")
	}
	srv, err := server.New(next.Client(), server.DefaultConfig(), server.WithRuntime(next))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	srv.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/providers/provider", nil))
	if response.Code != http.StatusNotFound {
		t.Errorf("HTTP status=%d, want 404 for withdrawn provider", response.Code)
	}
}

func TestStartupExplicitEmptyBindingsDropsStoredScopeWithoutInputs(t *testing.T) {
	store, _ := acceptedScopedStartupStore(t)
	next := openTestRuntime(t, WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)), WithProviderBindings())
	if _, err := next.State().Catalog.Provider("provider"); err == nil {
		t.Fatal("explicit empty policy kept withdrawn provider")
	}
	if next.State().PayloadChecksum != next.Client().CurrentCatalogState().PayloadChecksum {
		t.Fatal("client differs from runtime")
	}
}

func TestStartupUnscopedStoreOnlyCatalogRemainsAvailable(t *testing.T) {
	store := storage.NewMemory()
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store))}
	first := openTestRuntime(t, options...)
	layer := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	if _, err := first.publishProviders(t.Context(), []ProviderLayer{layer}, first.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	accepted := first.State()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	next := openTestRuntime(t, append(options, WithStateDirectory(privateRuntimeDirectory(t)))...)
	if _, err := next.State().Catalog.Provider("provider"); err != nil {
		t.Fatal("permitted unscoped catalog disappeared")
	}
	if next.State().GenerationID != accepted.GenerationID || next.State().PayloadChecksum != accepted.PayloadChecksum {
		t.Fatal("permitted accepted state changed")
	}
}

func acceptedScopedStartupStore(t *testing.T) (*storage.Memory, string) {
	t.Helper()
	layer := scopedProviderLayer(t, "private-binding", "1", time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	store := storage.NewMemory()
	directory := privateRuntimeDirectory(t)
	first := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)), WithProviderBindings(*layer.Receipt.ProviderBinding))
	if _, err := first.publishProviders(t.Context(), []ProviderLayer{layer}, first.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	if _, err := first.State().Catalog.Provider("provider"); err != nil {
		t.Fatal("fixture did not publish private provider")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	return store, directory
}

func TestStartupRemovedBindingRefusesFailedPublication(t *testing.T) {
	store, directory := acceptedScopedStartupStore(t)
	before, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	failed, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithSourcePollInterval(0), WithAcquisitionEnabled(false), WithClientOptions(starmap.WithCatalogStore(&policyFailingStore{store})))
	if failed != nil {
		_ = failed.Close()
	}
	if !errors.Is(err, fs.ErrPermission) || failed != nil {
		t.Fatalf("Open error=%v, want publication refusal without a runtime", err)
	}
	after, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if after.Manifest.GenerationID != before.Manifest.GenerationID || after.Manifest.Payload.Checksum != before.Manifest.Payload.Checksum {
		t.Fatal("failed publication changed accepted state")
	}
	recovered := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)))
	if _, err := recovered.Client().CurrentCatalogState().Catalog.Provider("provider"); err == nil {
		t.Fatal("recovered publication retains withdrawn provider")
	}
}
