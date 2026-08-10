package acquisition

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublishObservationsCommitsTenantOfferingGeneration(t *testing.T) {
	client, err := starmap.New(starmap.WithCatalogStore(catalogstore.NewMemory()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}

	builder, err := catalogs.NewBuilderFrom(client.Catalog())
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	if err := builder.SetAuthor(catalogs.Author{ID: "tenant", Name: "Tenant"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel("tenant", catalogs.Model{
		ID: "local-chat", Name: "Local Chat",
		Authors: []catalogs.Author{{ID: "tenant", Name: "Tenant"}},
		Features: &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProviderModel(catalogs.ProviderIDOllama, catalogs.Model{
		ID: "tenant-deployment", ModelRef: "tenant/local-chat", Name: "Tenant deployment",
		Status: catalogs.ModelStatusActive,
		Features: &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		}},
	}); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	if err := builder.SetProviderModel(catalogs.ProviderIDOllama, catalogs.Model{
		ID: "opaque/unreviewed@2026", Name: "Unreviewed provider model",
	}); err != nil {
		t.Fatalf("SetProviderModel unreviewed: %v", err)
	}
	observedCatalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatalf("NewObservationCatalog: %v", err)
	}
	observation, err := sources.NewObservation(
		sources.LocalCatalogID,
		observedCatalog,
		sources.ObservationMetadata{
			ObservedAt:   time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC),
			Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessComplete,
			Status:       sources.ObservationStatusSucceeded,
			Records:      sources.ObservationRecordCounts{Accepted: 3},
		},
	)
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}

	publication, err := syncer.PublishObservations(context.Background(), observation)
	if err != nil {
		t.Fatalf("PublishObservations: %v", err)
	}
	if !publication.Published || publication.GenerationID == "" {
		t.Fatalf("publication = %#v, want committed generation", publication)
	}
	offering, err := client.Catalog().Offering(catalogs.ProviderIDOllama, "tenant-deployment")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.DefinitionID != "tenant/local-chat" {
		t.Fatalf("DefinitionID = %q, want tenant/local-chat", offering.DefinitionID)
	}
	if !offering.Supports(catalogs.ProviderOperationChatCompletions) {
		t.Fatal("tenant offering does not support chat completions")
	}
	if _, err := client.Catalog().Offering(catalogs.ProviderIDOllama, "opaque/unreviewed@2026"); err == nil {
		t.Fatal("unreviewed provider model was published")
	}
	generation, err := client.CurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("CurrentGeneration: %v", err)
	}
	if len(generation.Manifest.ReviewCandidates) != 1 {
		t.Fatalf("review candidates = %#v", generation.Manifest.ReviewCandidates)
	}
	candidate := generation.Manifest.ReviewCandidates[0]
	if candidate.ProviderID != catalogs.ProviderIDOllama.String() ||
		candidate.ProviderModelID != "opaque/unreviewed@2026" ||
		candidate.SourceID != observation.SourceID ||
		candidate.SourceObservationID != observation.ID ||
		candidate.SourceRevision != observation.Revision ||
		candidate.EvidenceChecksum != observation.EvidenceChecksum ||
		candidate.Reason == "" ||
		candidate.PriorReviewedModelLink != "" {
		t.Fatalf("review candidate = %#v", candidate)
	}
}

func TestPublishObservationsPersistsReviewCandidateWithoutCatalogChange(t *testing.T) {
	client, err := starmap.New(starmap.WithCatalogStore(catalogstore.NewMemory()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}
	before, err := client.CurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("CurrentGeneration before publication: %v", err)
	}

	builder, err := catalogs.NewBuilderFrom(client.Catalog())
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	const providerModelID = "opaque/unreviewed-only@2026"
	if err := builder.SetProviderModel(catalogs.ProviderIDOllama, catalogs.Model{
		ID:   providerModelID,
		Name: "Unreviewed provider model",
	}); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	observedCatalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatalf("NewObservationCatalog: %v", err)
	}
	observation, err := sources.NewObservation(
		sources.LocalCatalogID,
		observedCatalog,
		sources.ObservationMetadata{
			ObservedAt:   time.Date(2026, 8, 3, 13, 0, 0, 0, time.UTC),
			Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessComplete,
			Status:       sources.ObservationStatusSucceeded,
			Records:      sources.ObservationRecordCounts{Accepted: 1},
		},
	)
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}

	publication, err := syncer.PublishObservations(context.Background(), observation)
	if err != nil {
		t.Fatalf("PublishObservations: %v", err)
	}
	if !publication.Published || publication.GenerationID == "" ||
		publication.GenerationID == before.Manifest.GenerationID {
		t.Fatalf("publication = %#v, want new durable generation", publication)
	}
	after, err := client.CurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("CurrentGeneration after publication: %v", err)
	}
	if _, err := client.Catalog().Offering(catalogs.ProviderIDOllama, providerModelID); err == nil {
		t.Fatal("unreviewed provider model was published")
	}
	if len(after.Manifest.ReviewCandidates) != 1 {
		t.Fatalf("review candidates = %#v", after.Manifest.ReviewCandidates)
	}
	candidate := after.Manifest.ReviewCandidates[0]
	if candidate.ProviderID != catalogs.ProviderIDOllama.String() ||
		candidate.ProviderModelID != providerModelID ||
		candidate.SourceID != observation.SourceID ||
		candidate.SourceObservationID != observation.ID ||
		candidate.SourceRevision != observation.Revision ||
		candidate.EvidenceChecksum != observation.EvidenceChecksum ||
		candidate.Reason == "" ||
		candidate.PriorReviewedModelLink != "" {
		t.Fatalf("review candidate = %#v", candidate)
	}
}
