package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestRuntimeStartupExplicitlyRepairsWorkspaceFromDurableCurrent(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "workspace")
	store := storage.NewMemory()
	seed, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "repair-provider", Name: "Repair Provider"}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.LocalCatalogID, catalog, sources.ObservationMetadata{
		ObservedAt: time.Now(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
		return starmap.NewCandidate(catalog, starmap.CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{observation.Link()}})
	}); err != nil {
		t.Fatal(err)
	}
	generation, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	options := []starmap.Option{starmap.WithCatalogStore(store), starmap.WithCatalogPath(path)}
	if _, err := starmap.NewContext(t.Context(), options...); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("passive constructor created the workspace: %v", err)
	}
	runtime, err := Open(t.Context(), WithClientOptions(options...), WithCatalogSource("embedded"), WithSourcePollInterval(0), WithAcquisitionEnabled(false))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = runtime.Close() }()
	if _, err := catalogs.NewFromPath(path); err != nil {
		t.Fatalf("runtime did not restore the workspace: %v", err)
	}
	current, err := store.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != generation.Manifest.GenerationID {
		t.Fatalf("repair replaced the durable current: %v", err)
	}
}
