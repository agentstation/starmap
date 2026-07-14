package starmap

import (
	"context"
	stderrors "errors"
	"os"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestCredentialScopedCatalogIsQueryableButNeverPersistedOrReusedAsBaseline(t *testing.T) {
	baselineBuilder := catalogs.NewEmpty()
	if err := baselineBuilder.SetProvider(catalogs.Provider{ID: "provider-a", Name: "Provider A"}); err != nil {
		t.Fatal(err)
	}
	baseline, err := baselineBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	store := catalogstore.NewMemory()
	client := &Client{
		options: &options{catalogStore: store}, catalog: baseline, publicationBaseline: baseline,
		generationSequence: 1, hooks: newHooks(),
	}

	contextualBuilder, err := catalogs.NewBuilderFrom(baseline)
	if err != nil {
		t.Fatal(err)
	}
	offering := catalogs.ProviderOffering{
		ProviderID: "provider-a", ProviderModelID: "workspace-model", DefinitionID: "workspace-model",
		DeploymentID: "deployment-account-a", Aliases: []string{"account-a-alias"},
		Availability: catalogs.OfferingAvailabilityRestricted,
		Access: catalogs.OfferingAccess{
			Channel:     catalogs.OfferingAccessChannelServerToServer,
			Routability: catalogs.OfferingRoutabilityRoutable,
			APIs:        []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions},
		},
		Deployment: catalogs.ProviderDeployment{Type: "workspace"},
		Endpoint: catalogs.ProviderOfferingEndpoint{
			Type: catalogs.EndpointTypeOpenAI, BaseURL: "https://workspace.example.test", Path: "/v1/chat/completions",
		},
		Lifecycle: catalogs.OfferingLifecycleActive,
	}
	if err := contextualBuilder.SetDefinition(catalogs.ModelDefinition{ID: "workspace-model", Name: "Workspace Model"}); err != nil {
		t.Fatal(err)
	}
	if err := contextualBuilder.SetOffering(offering); err != nil {
		t.Fatal(err)
	}
	contextual, err := contextualBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, contextual, sources.ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 14, 6, 30, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Scope: catalogmeta.ObservationScopeCredentialScoped, Kind: catalogmeta.SourceKindDirectInventory,
	})
	if err != nil {
		t.Fatal(err)
	}
	export := t.TempDir()
	publication, err := client.save(context.Background(), contextualBuilder, &pkgsync.Options{OutputPath: export}, &differ.Changeset{}, []sources.Observation{observation})
	if err != nil {
		t.Fatalf("save contextual: %v", err)
	}
	if publication != (pipeline.Publication{Contextual: true}) {
		t.Fatalf("publication = %#v", publication)
	}
	if _, err := client.Catalog().OfferingByKey(offering.Key()); err != nil {
		t.Fatalf("contextual offering is not queryable: %v", err)
	}
	if _, err := client.publicationCatalog().OfferingByKey(offering.Key()); err == nil {
		t.Fatal("credential-scoped offering entered the public reconciliation baseline")
	}
	if client.CurrentGenerationID() != "" {
		t.Fatalf("contextual view has public generation ID %q", client.CurrentGenerationID())
	}
	if _, err := store.Current(context.Background()); !stderrors.Is(err, errors.ErrNotFound) {
		t.Fatalf("credential-scoped catalog reached generation store: %v", err)
	}
	entries, err := os.ReadDir(export)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("credential-scoped catalog wrote export files: %#v", entries)
	}
	if client.publicationCatalog() != baseline {
		t.Fatal("public baseline identity changed during contextual application")
	}
}
