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
	observedCatalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	observation, err := sources.NewObservation(
		sources.LocalCatalogID,
		observedCatalog,
		sources.ObservationMetadata{
			ObservedAt:   time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC),
			Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessComplete,
			Status:       sources.ObservationStatusSucceeded,
			Records:      sources.ObservationRecordCounts{Accepted: 2},
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
}
