package starmap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/internal/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestNewPrefersDurableCurrentOverCorruptLocalCompatibilityView(t *testing.T) {
	store := catalogstore.NewMemory()
	generation := rootRemoteGeneration(t)
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	localPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(localPath, "providers.yaml"), []byte("providers: [unterminated\n"), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Write corrupt local view: %v", err)
	}

	client, err := New(WithCatalogStore(store), WithCatalogPath(localPath))
	if err != nil {
		t.Fatalf("New rejected valid durable current because local view was corrupt: %v", err)
	}
	if got := client.CurrentGenerationID(); got != generation.Manifest.GenerationID {
		t.Fatalf("generation ID = %q, want %q", got, generation.Manifest.GenerationID)
	}
	if _, err := client.Catalog().Provider("remote-root"); err != nil {
		t.Fatalf("durable catalog was not published: %v", err)
	}
}

// TestF001CharacterizationNewPrefersDurableCurrentOverValidLocalWorkspace pins
// the current restart precedence. The P3 workspace lifecycle must invert this
// behavior through explicit edit detection/reconciliation and digest repair;
// it must not silently ignore a valid human workspace merely because a durable
// generation exists.
func TestF001CharacterizationNewPrefersDurableCurrentOverValidLocalWorkspace(t *testing.T) {
	store := catalogstore.NewMemory()
	generation := rootRemoteGeneration(t)
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	localPath := t.TempDir()
	local := catalogs.NewEmpty()
	if err := local.SetProvider(catalogs.Provider{ID: "workspace-only", Name: "Workspace Only"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := local.SaveTo(localPath); err != nil {
		t.Fatalf("Save local workspace: %v", err)
	}

	client, err := New(WithCatalogStore(store), WithCatalogPath(localPath))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := client.CurrentGenerationID(); got != generation.Manifest.GenerationID {
		t.Fatalf("generation ID = %q, want %q", got, generation.Manifest.GenerationID)
	}
	if _, err := client.Catalog().Provider("remote-root"); err != nil {
		t.Fatalf("durable provider missing: %v", err)
	}
	if _, err := client.Catalog().Provider("workspace-only"); err == nil {
		t.Fatal("F-001 characterization changed: valid local workspace affected restart catalog")
	}
}

func TestRemoteCatalogCorruptOrIncompatibleGenerationCannotReplaceCurrent(t *testing.T) {
	valid := rootRemoteGeneration(t)
	for _, test := range []struct {
		name       string
		generation catalogstore.Generation
		corrupt    bool
	}{
		{name: "corrupt payload", generation: valid, corrupt: true},
		{name: "incompatible manifest", generation: incompatibleRemoteGeneration(t, valid)},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := catalogstore.NewMemory()
			client, err := New(WithCatalogStore(store))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			generation := test.generation.Copy()
			if test.corrupt {
				generation.Payload[len(generation.Payload)-1] ^= 1
			}
			beforeCatalog := client.Catalog()
			beforeID := client.CurrentGenerationID()
			if _, err := client.Activate(context.Background(), generation); err == nil {
				t.Fatal("invalid remote generation replaced current catalog")
			}
			if client.Catalog() != beforeCatalog || client.CurrentGenerationID() != beforeID {
				t.Fatalf("current catalog changed to %q", client.CurrentGenerationID())
			}
			if _, err := store.Current(context.Background()); !pkgerrors.IsNotFound(err) {
				t.Fatalf("invalid remote generation reached durable store: %v", err)
			}
		})
	}
}

func rootRemoteGeneration(t *testing.T) catalogstore.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "remote-root", Name: "Remote Root"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	at := time.Date(2026, time.July, 11, 4, 0, 0, 0, time.UTC)
	observation, err := sources.NewObservation(sources.LocalCatalogID, catalog, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	generation, err := generationTestClient(at).newGeneration(
		catalog,
		[]catalogs.SourceObservationLink{observation.Link()},
	)
	if err != nil {
		t.Fatalf("newGeneration: %v", err)
	}
	return generation
}

func incompatibleRemoteGeneration(t *testing.T, generation catalogstore.Generation) catalogstore.Generation {
	t.Helper()
	incompatible := generation.Copy()
	future := catalogs.CurrentCatalogSchemaVersion + 1
	incompatible.Manifest.SchemaVersion = future
	incompatible.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{
		MinSchemaVersion: future,
		MaxSchemaVersion: future,
	}
	if err := incompatible.Validate(); err != nil {
		t.Fatalf("incompatible fixture must remain internally valid: %v", err)
	}
	return incompatible
}
