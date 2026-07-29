package starmap

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/internal/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
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
	client, err := New(WithCatalogPath(path))
	if err == nil || client != nil {
		t.Fatalf("New = (%v, %v), want nil client and error", client, err)
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("New error = %T: %v, want *errors.ParseError", err, err)
	}
}

func TestConfiguredCatalogPathLoadsHumanWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog")
	local := catalogs.NewEmpty()
	if err := local.SetProvider(catalogs.Provider{ID: "local-only", Name: "Local only"}); err != nil {
		t.Fatalf("Seed local provider: %v", err)
	}
	if err := local.SaveTo(path); err != nil {
		t.Fatalf("Save local catalog: %v", err)
	}

	client, err := New(WithCatalogPath(path))
	if err != nil {
		t.Fatalf("New with catalog path: %v", err)
	}
	if client == nil {
		t.Fatal("New returned a nil client")
	}
	catalog := client.Catalog()
	if _, err := catalog.Provider("local-only"); err != nil {
		t.Fatalf("configured workspace was not loaded: %v", err)
	}
}

func TestConfiguredWorkspaceLoadsSemanticHumanValuesWithoutEmbeddedPreMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog")
	human := catalogs.NewEmpty()
	if err := human.SetProvider(catalogs.Provider{
		ID:   "openai",
		Name: "Human OpenAI",
		Models: map[string]*catalogs.Model{
			"gpt-4o": {ID: "gpt-4o", Name: "Human GPT-4o"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	seedTestModelDefinitions(t, human)
	if err := human.SaveTo(path); err != nil {
		t.Fatalf("Save human workspace: %v", err)
	}

	client, err := New(WithCatalogPath(path))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	catalog := client.Catalog()
	if got := catalog.Providers().Len(); got != 1 {
		t.Fatalf("provider count = %d, want human workspace only", got)
	}
	model, err := catalog.FindModel("gpt-4o")
	if err != nil {
		t.Fatalf("FindModel: %v", err)
	}
	if model.Name != "Human GPT-4o" {
		t.Fatalf("model name = %q, want semantic human value", model.Name)
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
	client.swapCatalogGeneration(client.Catalog(), "durable-generation", time.Time{})
	if got := client.CurrentGenerationID(); got != "durable-generation" {
		t.Fatalf("published generation ID = %q", got)
	}
}

func TestConfiguredLocalCatalogHasNoInventedGenerationID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog")
	local := catalogs.NewEmpty()
	if err := local.SetProvider(catalogs.Provider{ID: "local", Name: "Local"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := local.SaveTo(path); err != nil {
		t.Fatalf("Save local catalog: %v", err)
	}
	client, err := New(WithCatalogPath(path))
	if err != nil {
		t.Fatalf("New local: %v", err)
	}
	if got := client.CurrentGenerationID(); got != "" {
		t.Fatalf("local catalog generation ID = %q, want empty unknown identity", got)
	}
}

func TestUpdateUsesExplicitCandidateFunction(t *testing.T) {
	called := false
	opts, err := defaults().apply(
		WithCatalogStore(catalogstore.NewMemory()),
	)
	if err != nil {
		t.Fatalf("Apply options: %v", err)
	}

	client := &Client{options: opts, catalog: mustTestCatalog(t, catalogs.NewEmpty()), hooks: newHooks()}
	if _, err := client.Update(context.Background(), func(
		_ context.Context,
		catalog *catalogs.Catalog,
	) (*Candidate, error) {
		called = true
		return NewCandidate(catalog)
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !called {
		t.Fatal("explicit update function was not called")
	}
}

func TestActivateUsesExactImmutableGeneration(t *testing.T) {
	generation := rootRemoteGeneration(t)
	client, err := New(WithCatalogStore(catalogstore.NewMemory()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	events := make(chan CatalogPublishedEvent, 2)
	client.OnCatalogPublished(func(event CatalogPublishedEvent) error {
		events <- event
		return nil
	})
	if _, err := client.Activate(context.Background(), generation); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if _, err := client.Catalog().Provider("remote-root"); err != nil {
		t.Fatalf("activated provider not published: %v", err)
	}
	if got := client.CurrentGenerationID(); got != generation.Manifest.GenerationID {
		t.Fatalf("remote generation ID = %q, want %q", got, generation.Manifest.GenerationID)
	}
	firstState := client.CurrentCatalogState()
	if !firstState.GeneratedAt.Equal(generation.Manifest.GeneratedAt) {
		t.Fatalf(
			"remote generation time = %s, want %s",
			firstState.GeneratedAt,
			generation.Manifest.GeneratedAt,
		)
	}
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for activation publication")
	}
	if publication, err := client.Activate(context.Background(), generation); err != nil {
		t.Fatalf("idempotent Activate: %v", err)
	} else if publication.Published {
		t.Fatal("idempotent activation reported a second publication")
	}
	if state := client.CurrentCatalogState(); state.GenerationID != firstState.GenerationID || state.Sequence != firstState.Sequence {
		t.Fatalf("idempotent retry republished generation: before=%#v after=%#v", firstState, state)
	}
	assertNoCatalogEvent(t, events)
}
