package starmap

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestFirstExplicitUpdateAtomicallySeedsAbsentWorkspaceFromEmbedded(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "catalog")
	store := storage.NewMemory()
	client, err := New(
		WithCatalogStore(store),
		WithCatalogPath(path),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := os.Lstat(path); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("construction created workspace: %v", err)
	}
	baselineLocalEvidence := localEvidenceKeys(client.Catalog())

	result, err := client.Sync(
		context.Background(),
		pkgsync.WithSources(sources.LocalCatalogID),
	)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if result.HasChanges() {
		t.Fatal("embedded-only first update unexpectedly changed catalog facts")
	}
	if result.GenerationID == "" || result.Projection == nil ||
		result.Projection.Status != pkgsync.ProjectionStatusApplied {
		t.Fatalf("first update result = %#v", result)
	}

	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Manifest.GenerationID != result.GenerationID {
		t.Fatalf("stored generation = %q, want %q", current.Manifest.GenerationID, result.GenerationID)
	}
	if len(current.Manifest.SourceObservations) != 1 ||
		current.Manifest.SourceObservations[0].Source != sources.EmbeddedCatalogID {
		t.Fatalf("first generation observations = %#v, want embedded only", current.Manifest.SourceObservations)
	}
	projected, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	if err := projected.LoadReport().Err(); err != nil {
		t.Fatalf("projected model files: %v", err)
	}
	projectedCatalog, err := projected.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	gotPayload, err := catalogs.EncodeCatalogPayload(projectedCatalog)
	if err != nil {
		t.Fatalf("Encode projected catalog: %v", err)
	}
	if got, want := catalogs.DescribeCatalogPayload(gotPayload).Checksum,
		result.Projection.WorkspaceChecksum; got != want {
		t.Fatalf("projected checksum = %q, want receipt %q", got, want)
	}
	if len(projectedCatalog.Providers().List()) == 0 || len(projectedCatalog.Definitions()) == 0 {
		t.Fatal("first workspace is not a complete embedded-seeded catalog")
	}
	if got, want := len(projectedCatalog.Providers().List()), len(client.Catalog().Providers().List()); got != want {
		t.Fatalf("projected providers = %d, want %d", got, want)
	}
	if got, want := len(projectedCatalog.Definitions()), len(client.Catalog().Definitions()); got != want {
		t.Fatalf("projected definitions = %d, want %d", got, want)
	}
	assertEveryProviderModelJoinsAuthoredDefinition(t, projectedCatalog)
	evidence := projectedCatalog.Provenance().Map()
	if len(evidence) == 0 {
		t.Fatal("first workspace omitted embedded source provenance")
	}
	for resource, history := range evidence {
		for _, entry := range history {
			if entry.Source == sources.LocalCatalogID {
				if _, existed := baselineLocalEvidence[resource]; !existed {
					t.Fatalf("absent workspace created new local provenance %q", resource)
				}
			}
		}
	}
	for _, forbidden := range []string{"definitions", "offerings", "overrides"} {
		if _, err := os.Lstat(filepath.Join(path, forbidden)); !stderrors.Is(err, os.ErrNotExist) {
			t.Fatalf("forbidden persisted tree %q exists: %v", forbidden, err)
		}
	}
	matches, err := filepath.Glob(filepath.Join(root, ".catalog.candidate-*"))
	if err != nil {
		t.Fatalf("Glob staging: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("first-run staging survived: %v", matches)
	}
}

func TestFirstExplicitUpdateCommitFailureLeavesWorkspaceAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog")
	store := newCommitGateStore(true)
	client, err := New(
		WithCatalogStore(store),
		WithCatalogPath(path),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	before := client.CurrentGenerationID()
	done := make(chan error, 1)
	go func() {
		_, syncErr := client.Sync(
			context.Background(),
			pkgsync.WithSources(sources.LocalCatalogID),
		)
		done <- syncErr
	}()

	<-store.entered
	if _, err := os.Lstat(path); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace exists before durable commit: %v", err)
	}
	close(store.release)
	if err := <-done; err == nil {
		t.Fatal("Sync succeeded after injected commit failure")
	}
	if _, err := os.Lstat(path); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed commit left workspace behind: %v", err)
	}
	if got := client.CurrentGenerationID(); got != before {
		t.Fatalf("active generation = %q, want prior embedded %q", got, before)
	}
	if _, err := store.Current(context.Background()); !stderrors.Is(err, starmaperrors.ErrNotFound) {
		t.Fatalf("store Current error = %v, want not found", err)
	}
}

func localEvidenceKeys(catalog *catalogs.Catalog) map[string]struct{} {
	keys := make(map[string]struct{})
	for resource, history := range catalog.Provenance().Map() {
		for _, entry := range history {
			if entry.Source == sources.LocalCatalogID {
				keys[resource] = struct{}{}
			}
		}
	}
	return keys
}

func assertEveryProviderModelJoinsAuthoredDefinition(t *testing.T, catalog *catalogs.Catalog) {
	t.Helper()

	authored := make(map[catalogs.ModelDefinitionID]struct{})
	for _, record := range catalog.AuthoredModels() {
		authored[record.ID()] = struct{}{}
	}
	if len(authored) == 0 {
		t.Fatal("catalog has no authored canonical model definitions")
	}

	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%q): %v", provider.ID, err)
		}
		if len(offerings) != len(provider.Models) {
			t.Fatalf(
				"provider %q offerings = %d, want one for each of %d provider models",
				provider.ID,
				len(offerings),
				len(provider.Models),
			)
		}
		for _, offering := range offerings {
			if _, found := authored[offering.DefinitionID]; !found {
				t.Fatalf(
					"provider offering %q/%q references missing authored definition %q",
					offering.ProviderID,
					offering.ProviderModelID,
					offering.DefinitionID,
				)
			}
		}
	}
}
