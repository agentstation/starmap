package starmap

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestNewRejectsCorruptConfiguredLocalCatalog(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(path, "providers.yaml"),
		[]byte("- id: invalid\n  name: [unterminated\n"),
		constants.FilePermissions,
	); err != nil {
		t.Fatalf("Write corrupt catalog: %v", err)
	}
	client, err := New(WithLocalPath(path))
	if err == nil || client != nil {
		t.Fatalf("New = (%v, %v), want nil client and error", client, err)
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("New error = %T: %v, want *errors.ParseError", err, err)
	}
}

func TestEmbeddedCatalogTakesPrecedenceOverLocalPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog")
	local := catalogs.NewEmpty()
	if err := local.SetProvider(catalogs.Provider{ID: "local-only", Name: "Local only"}); err != nil {
		t.Fatalf("Seed local provider: %v", err)
	}
	if err := local.Save(save.WithPath(path)); err != nil {
		t.Fatalf("Save local catalog: %v", err)
	}

	client, err := New(
		WithLocalPath(path),
		WithEmbeddedCatalog(),
	)
	if err != nil {
		t.Fatalf("New with explicit embedded catalog: %v", err)
	}
	if client == nil {
		t.Fatal("New returned a nil client")
	}
	catalog := client.Catalog()
	if _, err := catalog.Provider("local-only"); err == nil {
		t.Fatal("Explicit embedded catalog merged the configured local path")
	}
}

func TestCurrentGenerationIDTracksBootstrapAndDurablePublication(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	bootstrapID := client.Readiness().Embedded.GenerationID
	if bootstrapID == "" || client.CurrentGenerationID() != bootstrapID {
		t.Fatalf("bootstrap generation ID = %q, readiness = %q", client.CurrentGenerationID(), bootstrapID)
	}
	client.swapCatalogGeneration(client.Catalog(), "durable-generation")
	if got := client.CurrentGenerationID(); got != "durable-generation" {
		t.Fatalf("published generation ID = %q", got)
	}
}

func TestConfiguredLocalCatalogHasNoInventedGenerationID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog")
	if err := catalogs.NewEmpty().Save(save.WithPath(path)); err != nil {
		t.Fatalf("Save local catalog: %v", err)
	}
	client, err := New(WithLocalPath(path))
	if err != nil {
		t.Fatalf("New local: %v", err)
	}
	if got := client.CurrentGenerationID(); got != "" {
		t.Fatalf("local catalog generation ID = %q, want empty unknown identity", got)
	}
}

func TestRemoteServerURLDoesNotForceRemoteOnlyUpdates(t *testing.T) {
	called := false
	opts, err := defaults().apply(
		WithCatalogStore(catalogstore.NewMemory()),
		WithRemoteServerURL("http://127.0.0.1:1"),
		WithUpdateFunc(func(_ context.Context, catalog *catalogs.Builder) (*catalogs.Builder, error) {
			called = true
			return catalog, nil
		}),
	)
	if err != nil {
		t.Fatalf("Apply options: %v", err)
	}

	client := &Client{options: opts, catalog: mustTestCatalog(t, catalogs.NewEmpty()), hooks: newHooks()}
	if err := client.Update(context.Background()); err != nil {
		t.Fatalf("Update with non-exclusive remote configuration: %v", err)
	}
	if !called {
		t.Fatal("Non-exclusive remote configuration bypassed the configured update module")
	}
}

func TestRemoteServerOnlyUsesRemoteCatalog(t *testing.T) {
	remoteBuilder := catalogs.NewEmpty()
	if err := remoteBuilder.SetProvider(catalogs.Provider{ID: "remote-provider", Name: "Remote Provider"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	remoteCatalog, err := remoteBuilder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	observedAt := time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)
	observation, err := sources.NewObservation(sources.LocalCatalogID, remoteCatalog, sources.ObservationMetadata{
		ObservedAt: observedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	generation, err := generationTestClient(observedAt).newGeneration(remoteCatalog, []sources.Observation{observation})
	if err != nil {
		t.Fatalf("newGeneration: %v", err)
	}
	manifest, err := catalogremote.MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatalf("MarshalManifest: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	called := false
	opts, err := defaults().apply(
		WithCatalogStore(catalogstore.NewMemory()),
		WithRemoteServerOnly(server.URL),
		WithUpdateFunc(func(_ context.Context, catalog *catalogs.Builder) (*catalogs.Builder, error) {
			called = true
			return catalog, nil
		}),
	)
	if err != nil {
		t.Fatalf("Apply options: %v", err)
	}

	client := &Client{options: opts, catalog: mustTestCatalog(t, catalogs.NewEmpty()), hooks: newHooks()}
	events := make(chan CatalogPublishedEvent, 2)
	client.OnCatalogPublished(func(event CatalogPublishedEvent) error {
		events <- event
		return nil
	})
	if err := client.Update(context.Background()); err != nil {
		t.Fatalf("Remote-only update: %v", err)
	}
	if called {
		t.Fatal("Remote-only update invoked the configured local update module")
	}
	if _, err := client.Catalog().Provider("remote-provider"); err != nil {
		t.Fatalf("remote provider not published: %v", err)
	}
	if got := client.CurrentGenerationID(); got != generation.Manifest.GenerationID {
		t.Fatalf("remote generation ID = %q, want %q", got, generation.Manifest.GenerationID)
	}
	firstState := client.CurrentCatalogState()
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first remote publication")
	}
	if err := client.Update(context.Background()); err != nil {
		t.Fatalf("Idempotent remote-only update: %v", err)
	}
	if state := client.CurrentCatalogState(); state.GenerationID != firstState.GenerationID || state.Sequence != firstState.Sequence {
		t.Fatalf("idempotent retry republished generation: before=%#v after=%#v", firstState, state)
	}
	assertNoCatalogEvent(t, events)
}
