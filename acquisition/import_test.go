package acquisition

import (
	"bytes"
	"context"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type importPublisherVerifier struct {
	err   error
	want  []byte
	calls int
}

func (v *importPublisherVerifier) VerifyPublisher(
	_ context.Context,
	name string,
	data []byte,
) error {
	v.calls++
	if name != catalogartifact.Filename || len(data) == 0 ||
		(v.want != nil && !bytes.Equal(data, v.want)) {
		return stderrors.New("unexpected publisher subject")
	}
	return v.err
}

func TestImportReleaseVerifiesReconcilesPublishesAndRollsBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workspacePath := t.TempDir()
	baselineBuilder := importCatalogBuilder(t, "Human Model Name", "", false, true)
	if err := baselineBuilder.SaveTo(workspacePath); err != nil {
		t.Fatalf("SaveTo baseline workspace: %v", err)
	}
	baseline := buildImportCatalog(t, baselineBuilder)

	store := catalogstore.NewMemory()
	client, err := starmap.New(
		starmap.WithCatalogStore(store),
		starmap.WithCatalogPath(workspacePath),
	)
	if err != nil {
		t.Fatalf("New consumer: %v", err)
	}
	prior, err := client.Update(ctx, func(
		context.Context,
		*catalogs.Catalog,
	) (*starmap.Candidate, error) {
		return starmap.NewCandidate(baseline)
	})
	if err != nil {
		t.Fatalf("publish baseline: %v", err)
	}
	priorGeneration, err := client.CurrentGeneration(ctx)
	if err != nil {
		t.Fatalf("CurrentGeneration baseline: %v", err)
	}

	releaseCatalog := buildImportCatalog(
		t,
		importCatalogBuilder(t, "Release Model Name", "Release Description", true, false),
	)
	release := importReleaseFixture(t, releaseCatalog)
	verifier := &importPublisherVerifier{want: append([]byte(nil), release.Archive...)}
	syncer, err := New(client)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}
	result, err := syncer.ImportRelease(ctx, release, verifier)
	if err != nil {
		t.Fatalf("ImportRelease: %v", err)
	}
	if verifier.calls != 1 {
		t.Fatalf("publisher verifier calls = %d, want 1", verifier.calls)
	}
	if result.SourceGenerationID == "" ||
		!result.Publication.Published ||
		result.Publication.GenerationID == "" ||
		result.Publication.GenerationID == result.SourceGenerationID {
		t.Fatalf("import result = %#v", result)
	}
	if result.Projection == nil ||
		result.Projection.Status != catalogmeta.ProjectionStatusApplied {
		t.Fatalf("workspace projection = %#v, want applied", result.Projection)
	}

	imported := client.Catalog()
	provider, err := imported.Provider("provider-a")
	if err != nil {
		t.Fatalf("Provider provider-a: %v", err)
	}
	model := provider.Models["model-a"]
	if model == nil ||
		model.Name != "Human Model Name" ||
		model.Description != "Release Description" {
		t.Fatalf("reconciled model-a = %#v", model)
	}
	if _, err := imported.Offering("provider-a", "model-b"); err != nil {
		t.Fatalf("release-only model-b offering: %v", err)
	}
	if _, err := imported.Definition("release-lab/model-b"); err != nil {
		t.Fatalf("release-only model-b definition: %v", err)
	}
	if _, err := imported.Offering("manual-provider", "manual-model"); err != nil {
		t.Fatalf("manual-only offering: %v", err)
	}

	current, err := client.CurrentGeneration(ctx)
	if err != nil {
		t.Fatalf("CurrentGeneration imported: %v", err)
	}
	var releaseLinkFound bool
	for _, observation := range current.Manifest.SourceObservations {
		if observation.Source == catalogmeta.ReleaseArtifactID {
			releaseLinkFound = true
			if observation.Revision.Value != result.SourceGenerationID {
				t.Fatalf("release revision = %#v", observation.Revision)
			}
		}
	}
	if !releaseLinkFound {
		t.Fatalf("imported generation observations = %#v", current.Manifest.SourceObservations)
	}

	activeBeforeFailure := client.CurrentGenerationID()
	payloadBeforeFailure, err := catalogs.EncodeCatalogPayload(client.Catalog())
	if err != nil {
		t.Fatalf("EncodeCatalogPayload before failure: %v", err)
	}
	tampered := release
	tampered.Checksum = []byte("0  " + catalogartifact.Filename + "\n")
	if _, err := syncer.ImportRelease(ctx, tampered, verifier); err == nil {
		t.Fatal("tampered release import succeeded")
	}
	if client.CurrentGenerationID() != activeBeforeFailure {
		t.Fatal("tampered release changed the active generation")
	}
	payloadAfterFailure, err := catalogs.EncodeCatalogPayload(client.Catalog())
	if err != nil {
		t.Fatalf("EncodeCatalogPayload after failure: %v", err)
	}
	if !bytes.Equal(payloadBeforeFailure, payloadAfterFailure) {
		t.Fatal("tampered release changed the active catalog")
	}

	rollback, err := client.Rollback(ctx, prior.GenerationID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rollback.FromGenerationID != result.Publication.GenerationID ||
		rollback.GenerationID != prior.GenerationID {
		t.Fatalf("rollback result = %#v", rollback)
	}
	rolledBack, err := catalogs.EncodeCatalogPayload(client.Catalog())
	if err != nil {
		t.Fatalf("EncodeCatalogPayload rollback: %v", err)
	}
	if !bytes.Equal(rolledBack, priorGeneration.Payload) {
		t.Fatal("rollback did not restore the exact retained prior generation")
	}
}

func TestImportReleasePublisherFailureCannotMutateClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := catalogstore.NewMemory()
	client, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	beforeID := client.CurrentGenerationID()
	before := client.Catalog()
	releaseCatalog := buildImportCatalog(
		t,
		importCatalogBuilder(t, "Release", "Description", true, false),
	)
	release := importReleaseFixture(t, releaseCatalog)
	rejected := stderrors.New("publisher identity rejected")
	verifier := &importPublisherVerifier{
		err: rejected, want: append([]byte(nil), release.Archive...),
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}

	if _, err := syncer.ImportRelease(ctx, release, verifier); !stderrors.Is(err, rejected) {
		t.Fatalf("ImportRelease error = %v, want publisher rejection", err)
	}
	if verifier.calls != 1 || client.CurrentGenerationID() != beforeID ||
		client.Catalog() != before {
		t.Fatal("publisher rejection changed immutable client state")
	}
	if _, err := store.Current(ctx); !stderrors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("store Current error = %v, want not found", err)
	}
}

func importCatalogBuilder(
	t testing.TB,
	modelName, description string,
	includeReleaseOnly, includeManualOnly bool,
) *catalogs.Builder {
	t.Helper()

	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{
		ID: "release-lab", Name: "Release Lab",
	}); err != nil {
		t.Fatalf("SetAuthor release-lab: %v", err)
	}
	if err := builder.SetAuthorModel("release-lab", catalogs.Model{
		ID: "model-a", Name: "Canonical Model A",
		Authors: []catalogs.Author{{ID: "release-lab", Name: "Release Lab"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel model-a: %v", err)
	}
	providerModels := map[string]*catalogs.Model{
		"model-a": {
			ID:          "model-a",
			ModelRef:    "release-lab/model-a",
			Name:        modelName,
			Description: description,
		},
	}
	if includeReleaseOnly {
		if err := builder.SetAuthorModel("release-lab", catalogs.Model{
			ID: "model-b", Name: "Canonical Model B",
			Authors: []catalogs.Author{{ID: "release-lab", Name: "Release Lab"}},
		}); err != nil {
			t.Fatalf("SetAuthorModel model-b: %v", err)
		}
		providerModels["model-b"] = &catalogs.Model{
			ID:       "model-b",
			ModelRef: "release-lab/model-b",
			Name:     "Release Model B",
		}
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID: "provider-a", Name: "Provider A", Models: providerModels,
	}); err != nil {
		t.Fatalf("SetProvider provider-a: %v", err)
	}
	if includeManualOnly {
		if err := builder.SetAuthor(catalogs.Author{
			ID: "manual-lab", Name: "Manual Lab",
		}); err != nil {
			t.Fatalf("SetAuthor manual-lab: %v", err)
		}
		if err := builder.SetAuthorModel("manual-lab", catalogs.Model{
			ID: "manual-model", Name: "Manual Definition",
			Authors: []catalogs.Author{{ID: "manual-lab", Name: "Manual Lab"}},
		}); err != nil {
			t.Fatalf("SetAuthorModel manual: %v", err)
		}
		if err := builder.SetProvider(catalogs.Provider{
			ID: "manual-provider", Name: "Manual Provider",
			Models: map[string]*catalogs.Model{
				"manual-model": {
					ID:       "manual-model",
					ModelRef: "manual-lab/manual-model",
					Name:     "Manual Offering",
				},
			},
		}); err != nil {
			t.Fatalf("SetProvider manual: %v", err)
		}
	}
	return builder
}

func buildImportCatalog(t testing.TB, builder *catalogs.Builder) *catalogs.Catalog {
	t.Helper()
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build catalog: %v", err)
	}
	return catalog
}

func importReleaseFixture(
	t testing.TB,
	catalog *catalogs.Catalog,
) catalogartifact.Release {
	t.Helper()
	producerStore := catalogstore.NewMemory()
	producer, err := starmap.New(starmap.WithCatalogStore(producerStore))
	if err != nil {
		t.Fatalf("New producer: %v", err)
	}
	if _, err := producer.Update(context.Background(), func(
		context.Context,
		*catalogs.Catalog,
	) (*starmap.Candidate, error) {
		return starmap.NewCandidate(catalog)
	}); err != nil {
		t.Fatalf("producer Update: %v", err)
	}
	generation, err := producer.CurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("producer CurrentGeneration: %v", err)
	}
	bundle, err := catalogartifact.Build(generation)
	if err != nil {
		t.Fatalf("Build artifact: %v", err)
	}
	return catalogartifact.Release{
		Archive:     bundle.Data,
		Checksum:    []byte(strings.TrimPrefix(bundle.Checksum, "sha256:") + "  " + catalogartifact.Filename + "\n"),
		Attestation: bundle.Attestation,
	}
}
