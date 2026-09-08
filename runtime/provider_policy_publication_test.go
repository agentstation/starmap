package runtime

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/server"
)

func TestProviderPolicyStartupAlignsHTTPGeneration(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	directory := privateRuntimeDirectory(t)
	first := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)), WithProviderBindings(*layer.Receipt.ProviderBinding))
	if _, err := first.publishProviders(t.Context(), []ProviderLayer{layer}, first.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	next := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)), WithProviderBindings())
	srv, err := server.New(next.Client(), server.DefaultConfig(), server.WithRuntime(next))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/providers/provider", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("HTTP status = %d, want 404 for inactive provider", recorder.Code)
	}
	manifest := httptest.NewRecorder()
	srv.Handler().ServeHTTP(manifest, httptest.NewRequest(http.MethodGet, "/api/v1/catalog/manifest", nil))
	if manifest.Code != http.StatusOK {
		t.Fatalf("manifest status = %d", manifest.Code)
	}
	if manifest.Header().Get("X-Starmap-Generation-ID") != next.State().GenerationID {
		t.Error("HTTP manifest and runtime select different generations")
	}
	if next.Client().CurrentCatalogState().PayloadChecksum != next.State().PayloadChecksum {
		t.Error("client and runtime disagree after startup")
	}
	stored, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if stored.Manifest.GenerationID != next.State().GenerationID {
		t.Error("durable current still selects the prior policy")
	}
}

func TestProviderPolicyReactivationPreservesImmutableGeneration(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	store := storage.NewMemory()
	directory := privateRuntimeDirectory(t)
	common := []Option{WithStateDirectory(directory), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store))}
	first := openTestRuntime(t, append(common, WithProviderBindings(*layer.Receipt.ProviderBinding))...)
	if _, err := first.publishProviders(t.Context(), []ProviderLayer{layer}, first.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	original, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := openTestRuntime(t, append(common, WithProviderBindings())...)
	if second.State().GenerationID == original.Manifest.GenerationID {
		t.Error("removed binding kept the prior selected identity")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	restored := openTestRuntime(t, append(common, WithProviderBindings(*layer.Receipt.ProviderBinding))...)
	if restored.State().GenerationID != original.Manifest.GenerationID {
		t.Error("explicitly restored declarations changed the immutable identity")
	}
	current, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if current.Manifest.SyncRunID != original.Manifest.SyncRunID || current.Manifest.GeneratedAt != original.Manifest.GeneratedAt {
		t.Error("reactivation rewrote the retained manifest")
	}
}

func TestProviderPolicyGenerationIdentityBindsCompleteSelection(t *testing.T) {
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	first := *scopedProviderLayer(t, "a", "1", at).Receipt.ProviderBinding
	second := *scopedProviderLayer(t, "b", "1", at).Receipt.ProviderBinding
	identity := func(upstream, checksum string, bindings ...sources.ProviderAcquisitionBinding) string {
		t.Helper()
		config, err := defaults().apply(WithProviderBindings(bindings...))
		if err != nil {
			t.Fatal(err)
		}
		id, err := config.providerBindings.generationID(upstream, checksum)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	want := identity("source.local.original", "checksum", first, second)
	if identity("source.local.original", "checksum", second, first) != want {
		t.Error("declaration order changes the identity")
	}
	changed := first
	changed.Revision = "2"
	for _, got := range []string{identity("source", "checksum", first, second), identity("source.local.original", "different", first, second), identity("source.local.original", "checksum", changed, second), identity("source.local.original", "checksum")} {
		if got == want {
			t.Error("changed selection reused an identity")
		}
	}
}

type policyFailingStore struct{ *storage.Memory }

func (s *policyFailingStore) Commit(context.Context, catalogs.Generation, string) error {
	return fs.ErrPermission
}

func TestProviderPolicyFailedStartupPublicationReleasesDirectory(t *testing.T) {
	directory := privateRuntimeDirectory(t)
	failed, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithSourcePollInterval(0), WithAcquisitionEnabled(false), WithProviderBindings(), WithClientOptions(starmap.WithCatalogStore(&policyFailingStore{storage.NewMemory()})))
	if err == nil {
		failed.Close()
		t.Fatal("failed policy publication returned a usable runtime")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("failure lost its storage cause: %v", err)
	}
	openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithProviderBindings())
}

type policyMismatchedReadStore struct {
	*storage.Memory
	generation catalogs.Generation
}

func (s *policyMismatchedReadStore) Get(context.Context, string) (catalogs.Generation, error) {
	return s.generation.Copy(), nil
}

func TestProviderPolicyRejectsMismatchedStoredIdentity(t *testing.T) {
	generation, err := starmap.EmbeddedGeneration()
	if err != nil {
		t.Fatal(err)
	}
	store := &policyMismatchedReadStore{Memory: storage.NewMemory(), generation: generation}
	connected, err := Open(t.Context(), WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithSourcePollInterval(0), WithAcquisitionEnabled(false), WithProviderBindings(), WithClientOptions(starmap.WithCatalogStore(store)))
	if err == nil {
		connected.Close()
		t.Error("store returned another identity with matching bytes and startup accepted it")
	}
	if _, err := store.Current(t.Context()); !pkgerrors.IsNotFound(err) {
		t.Error("a mismatched addressed generation reached current publication")
	}
}
