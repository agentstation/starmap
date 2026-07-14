package differ

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestCatalogsDetectsCanonicalOnlyChangesDeterministically(t *testing.T) {
	baseline := canonicalDifferCatalog(t, nil)
	updatedOffering := canonicalDifferOffering()
	updatedOffering.Regions = []catalogs.CloudRegion{{ID: "us-east-1", Realm: "aws"}, {ID: "us-west-2", Realm: "aws"}}
	updated := canonicalDifferCatalog(t, &updatedOffering)

	changes := New().Catalogs(baseline, updated)
	if !changes.HasChanges() || changes.Summary.OfferingsUpdated != 1 || changes.Summary.TotalChanges != 1 {
		t.Fatalf("canonical-only changeset = %#v", changes)
	}
	if len(changes.Offerings.Updated) != 1 || changes.Offerings.Updated[0].Key.ProviderModelID != "model" {
		t.Fatalf("offering updates = %#v", changes.Offerings.Updated)
	}

	copyBuilder, err := catalogs.NewBuilderFrom(updated)
	if err != nil {
		t.Fatal(err)
	}
	copyCatalog, err := copyBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if noOp := New().Catalogs(updated, copyCatalog); noOp.HasChanges() {
		t.Fatalf("deep-copy no-op reported changes: %#v", noOp)
	}
}

func TestCatalogsDetectsCanonicalAdditionsAndRemovals(t *testing.T) {
	emptyBuilder := catalogs.NewEmpty()
	if err := emptyBuilder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatal(err)
	}
	empty, err := emptyBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	canonical := canonicalDifferCatalog(t, nil)
	added := New().Catalogs(empty, canonical)
	if added.Summary.DefinitionsAdded != 1 || added.Summary.OfferingsAdded != 1 || !added.HasChanges() {
		t.Fatalf("canonical additions = %#v", added.Summary)
	}
	removed := New().Catalogs(canonical, empty)
	if removed.Summary.DefinitionsRemoved != 1 || removed.Summary.OfferingsRemoved != 1 || !removed.HasChanges() {
		t.Fatalf("canonical removals = %#v", removed.Summary)
	}
	if additive := removed.Filter(ApplyAdditive); additive.HasChanges() {
		t.Fatalf("additive filter retained canonical removals: %#v", additive)
	}
}

func canonicalDifferCatalog(t testing.TB, offeringOverride *catalogs.ProviderOffering) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetDefinition(catalogs.ModelDefinition{ID: "author/model", Name: "Model", AuthorIDs: []catalogs.AuthorID{"author"}}); err != nil {
		t.Fatal(err)
	}
	offering := canonicalDifferOffering()
	if offeringOverride != nil {
		offering = *offeringOverride
	}
	if err := builder.SetOffering(offering); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func canonicalDifferOffering() catalogs.ProviderOffering {
	return catalogs.ProviderOffering{
		ProviderID: "provider", ProviderModelID: "model", DefinitionID: "author/model",
		Availability: catalogs.OfferingAvailabilityAvailable,
		Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}},
		Regions:      []catalogs.CloudRegion{{ID: "us-east-1", Realm: "aws"}}, Deployment: catalogs.ProviderDeployment{Type: "serverless"},
		Endpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOpenAI}, Lifecycle: catalogs.OfferingLifecycleActive,
	}
}
