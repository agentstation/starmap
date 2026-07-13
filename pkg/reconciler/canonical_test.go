package reconciler

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCanonicalReconciliationUsesAuthorityAndUnionsRegions(t *testing.T) {
	baseline := canonicalTestCatalog(t, catalogs.ModelDefinition{
		ID: "anthropic/claude", Name: "Claude", AuthorIDs: []catalogs.AuthorID{"anthropic"}, Description: "curated description",
	}, canonicalTestOffering("anthropic/claude", 1, "us-east-1"))
	bedrockDefinition := catalogs.ModelDefinition{
		ID: "anthropic/claude", Name: "Amazon Claude", AuthorIDs: []catalogs.AuthorID{"anthropic", "research-partner"},
		Capabilities: catalogs.ModelDefinitionCapabilities{Features: &catalogs.ModelFeatures{Tools: true}},
	}
	bedrockOffering := canonicalTestOffering("anthropic/claude", 2, "us-west-2")
	bedrock := canonicalTestCatalog(t, bedrockDefinition, bedrockOffering)

	reconcile, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconcile.Sources(context.Background(), "", []sources.Observation{{SourceID: sources.AmazonBedrockID, Catalog: bedrock}})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	published, err := result.Catalog.Build()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := published.Definition("anthropic/claude")
	if err != nil {
		t.Fatal(err)
	}
	if definition.Name != "Claude" || definition.Description != "curated description" || len(definition.AuthorIDs) != 2 || definition.Capabilities.Features == nil || !definition.Capabilities.Features.Tools {
		t.Fatalf("definition authority merge = %#v", definition)
	}
	offering, err := published.Offering("amazon-bedrock", "anthropic.claude")
	if err != nil {
		t.Fatal(err)
	}
	if offering.Pricing == nil || offering.Pricing.Tokens.Input.Per1M != 2 || len(offering.Regions) != 2 || offering.Regions[0].ID != "us-west-2" || offering.Regions[1].ID != "us-east-1" {
		t.Fatalf("offering authority merge = %#v", offering)
	}

	offering.Regions[0].ID = "mutated"
	again, err := published.Offering("amazon-bedrock", "anthropic.claude")
	if err != nil || again.Regions[0].ID != "us-west-2" {
		t.Fatalf("canonical reconciliation leaked mutable state: (%#v, %v)", again, err)
	}
}

func TestCanonicalReconciliationRetainsLastKnownGoodOnDegradedAbsence(t *testing.T) {
	baseline := canonicalTestCatalog(t, catalogs.ModelDefinition{ID: "anthropic/claude", Name: "Claude"}, canonicalTestOffering("anthropic/claude", 1, "us-east-1"))
	emptyBuilder := catalogs.NewEmpty()
	empty, err := emptyBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	reconcile, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconcile.Sources(context.Background(), "", []sources.Observation{{
		SourceID: sources.AmazonBedrockID, Catalog: empty,
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
	}})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	published, err := result.Catalog.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := published.Offering("amazon-bedrock", "anthropic.claude"); err != nil {
		t.Fatalf("degraded absence deleted last-known-good offering: %v", err)
	}
}

func TestCanonicalReconciliationRetainsCatalogPricingOnInventoryOnlySuccess(t *testing.T) {
	definition := catalogs.ModelDefinition{ID: "mistral/model", Name: "Mistral Model"}
	baselineOffering := canonicalTestOffering("mistral/model", 1.5, "global")
	baselineOffering.ProviderID = catalogs.ProviderIDMistralAI
	baselineOffering.ProviderModelID = "mistral-model"
	baseline := canonicalTestCatalogForProvider(t, catalogs.ProviderIDMistralAI, definition, baselineOffering)

	inventoryOffering := baselineOffering
	inventoryOffering.Pricing = nil
	inventory := canonicalTestCatalogForProvider(t, catalogs.ProviderIDMistralAI, definition, inventoryOffering)
	reconcile, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconcile.Sources(context.Background(), "", []sources.Observation{{
		SourceID: sources.ProvidersID, Catalog: inventory,
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	}})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	published, err := result.Catalog.Build()
	if err != nil {
		t.Fatal(err)
	}
	offering, err := published.Offering(catalogs.ProviderIDMistralAI, "mistral-model")
	if err != nil {
		t.Fatal(err)
	}
	if offering.Pricing == nil || offering.Pricing.Tokens.Input.Per1M != 1.5 {
		t.Fatalf("inventory-only success erased catalog pricing: %#v", offering.Pricing)
	}
}

func TestCanonicalPricingAuthorityRejectsDegradedLiveEvidenceBeforeFallback(t *testing.T) {
	definition := catalogs.ModelDefinition{ID: "mistral/model", Name: "Mistral Model"}
	baselineOffering := canonicalTestOffering("mistral/model", 1.5, "global")
	baselineOffering.ProviderID, baselineOffering.ProviderModelID = catalogs.ProviderIDMistralAI, "mistral-model"
	providerOffering := baselineOffering
	providerOffering.Pricing = canonicalTestPricing(2)
	modelsDevOffering := baselineOffering
	modelsDevOffering.Pricing = canonicalTestPricing(0.5)

	tests := []struct {
		name          string
		baselinePrice *catalogs.ModelPricing
		providerPrice *catalogs.ModelPricing
		status        sources.ObservationStatus
		completeness  sources.ObservationCompleteness
		issues        []sources.ObservationIssue
		want          float64
	}{
		{name: "valid official live price wins", baselinePrice: canonicalTestPricing(1.5), providerPrice: canonicalTestPricing(2), status: sources.ObservationStatusSucceeded, completeness: sources.ObservationCompletenessComplete, want: 2},
		{name: "inventory only retains last known good", baselinePrice: canonicalTestPricing(1.5), status: sources.ObservationStatusSucceeded, completeness: sources.ObservationCompletenessComplete, want: 1.5},
		{name: "partial malformed price retains last known good", baselinePrice: canonicalTestPricing(1.5), providerPrice: canonicalTestPricing(2), status: sources.ObservationStatusDegraded, completeness: sources.ObservationCompletenessPartial, issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Message: "malformed provider price quarantined"}}, want: 1.5},
		{name: "stale provider fallback retains last known good", baselinePrice: canonicalTestPricing(1.5), providerPrice: canonicalTestPricing(2), status: sources.ObservationStatusDegraded, completeness: sources.ObservationCompletenessComplete, issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "stale provider pricing cache"}}, want: 1.5},
		{name: "models dev is lower fallback only", providerPrice: nil, status: sources.ObservationStatusSucceeded, completeness: sources.ObservationCompletenessComplete, want: 0.5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baselineOffering.Pricing = test.baselinePrice
			providerOffering.Pricing = test.providerPrice
			baseline := canonicalTestCatalogForProvider(t, catalogs.ProviderIDMistralAI, definition, baselineOffering)
			provider := canonicalTestCatalogForProvider(t, catalogs.ProviderIDMistralAI, definition, providerOffering)
			modelsDev := canonicalTestCatalogForProvider(t, catalogs.ProviderIDMistralAI, definition, modelsDevOffering)
			reconcile, err := New(WithBaseline(baseline))
			if err != nil {
				t.Fatal(err)
			}
			result, err := reconcile.Sources(context.Background(), "", []sources.Observation{
				{SourceID: sources.ProvidersID, Catalog: provider, Status: test.status, Completeness: test.completeness, Issues: test.issues},
				{SourceID: sources.ModelsDevHTTPID, Catalog: modelsDev, Status: sources.ObservationStatusSucceeded, Completeness: sources.ObservationCompletenessComplete},
			})
			if err != nil {
				t.Fatalf("Sources: %v", err)
			}
			published, err := result.Catalog.Build()
			if err != nil {
				t.Fatal(err)
			}
			offering, err := published.Offering(catalogs.ProviderIDMistralAI, "mistral-model")
			if err != nil || offering.Pricing == nil || offering.Pricing.Tokens.Input.Per1M != test.want {
				t.Fatalf("pricing = %#v/%v, want %v", offering.Pricing, err, test.want)
			}
		})
	}
}

func TestCanonicalOfferingDeletionRequiresCompleteAuthoritativeObservation(t *testing.T) {
	definition := catalogs.ModelDefinition{ID: "mistral/model", Name: "Mistral Model"}
	offering := canonicalTestOffering("mistral/model", 1.5, "global")
	offering.ProviderID, offering.ProviderModelID = catalogs.ProviderIDMistralAI, "retired-model"
	baseline := canonicalTestCatalogForProvider(t, catalogs.ProviderIDMistralAI, definition, offering)
	empty := canonicalProviderOnlyCatalog(t, catalogs.ProviderIDMistralAI)

	for _, test := range []struct {
		name         string
		source       sources.ID
		status       sources.ObservationStatus
		completeness sources.ObservationCompleteness
		wantRetained bool
	}{
		{name: "partial provider absence retains", source: sources.ProvidersID, status: sources.ObservationStatusDegraded, completeness: sources.ObservationCompletenessPartial, wantRetained: true},
		{name: "complete models dev absence retains", source: sources.ModelsDevHTTPID, status: sources.ObservationStatusSucceeded, completeness: sources.ObservationCompletenessComplete, wantRetained: true},
		{name: "complete provider absence deletes", source: sources.ProvidersID, status: sources.ObservationStatusSucceeded, completeness: sources.ObservationCompletenessComplete, wantRetained: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			reconcile, err := New(WithBaseline(baseline))
			if err != nil {
				t.Fatal(err)
			}
			observation := sources.Observation{SourceID: test.source, Catalog: empty, Status: test.status, Completeness: test.completeness}
			if test.status == sources.ObservationStatusDegraded {
				observation.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodeFetchFailed, Message: "partial inventory"}}
			}
			result, err := reconcile.Sources(context.Background(), "", []sources.Observation{observation})
			if err != nil {
				t.Fatalf("Sources: %v", err)
			}
			published, err := result.Catalog.Build()
			if err != nil {
				t.Fatal(err)
			}
			_, err = published.Offering(catalogs.ProviderIDMistralAI, "retired-model")
			if (err == nil) != test.wantRetained {
				t.Fatalf("offering retained = %v, want %v (err %v)", err == nil, test.wantRetained, err)
			}
		})
	}
}

func TestCanonicalReconciliationRejectsOfferingDefinitionConflict(t *testing.T) {
	baseline := canonicalTestCatalog(t, catalogs.ModelDefinition{ID: "author/model-a", Name: "A"}, canonicalTestOffering("author/model-a", 1, "us-east-1"))
	conflicting := canonicalTestCatalog(t, catalogs.ModelDefinition{ID: "author/model-b", Name: "B"}, canonicalTestOffering("author/model-b", 2, "us-west-2"))
	reconcile, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatal(err)
	}
	_, err = reconcile.Sources(context.Background(), "", []sources.Observation{{SourceID: sources.AmazonBedrockID, Catalog: conflicting}})
	var conflict *pkgerrors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("Sources error = %T %v, want ConflictError", err, err)
	}
}

func canonicalTestCatalog(t testing.TB, definition catalogs.ModelDefinition, offering catalogs.ProviderOffering) *catalogs.Catalog {
	return canonicalTestCatalogForProvider(t, "amazon-bedrock", definition, offering)
}

func canonicalTestCatalogForProvider(t testing.TB, providerID catalogs.ProviderID, definition catalogs.ModelDefinition, offering catalogs.ProviderOffering) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: providerID, Name: string(providerID)}); err != nil {
		t.Fatal(err)
	}
	for _, authorID := range definition.AuthorIDs {
		if err := builder.SetAuthor(catalogs.Author{ID: authorID, Name: string(authorID)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := builder.SetDefinition(definition); err != nil {
		t.Fatal(err)
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

func TestFillMissingReturnsTypedErrors(t *testing.T) {
	var validation *pkgerrors.ValidationError
	if err := fillMissing(nil, struct{}{}); !stderrors.As(err, &validation) {
		t.Fatalf("nil destination error = %T %v", err, err)
	}
	target := struct{ Value string }{}
	validation = nil
	if err := fillMissing(&target, 42); !stderrors.As(err, &validation) {
		t.Fatalf("mismatched source error = %T %v", err, err)
	}
	if err := fillMissing(&target, struct{ Value string }{Value: "filled"}); err != nil {
		t.Fatal(err)
	}
	if target.Value != "filled" {
		t.Fatalf("target = %#v", target)
	}
}

func canonicalTestOffering(definitionID catalogs.ModelDefinitionID, price float64, region string) catalogs.ProviderOffering {
	return catalogs.ProviderOffering{
		ProviderID: "amazon-bedrock", ProviderModelID: "anthropic.claude", DefinitionID: definitionID,
		Pricing:      &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: price}}},
		Availability: catalogs.OfferingAvailabilityAvailable,
		Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIBedrockConverse}},
		Regions:      []catalogs.CloudRegion{{ID: region, Realm: "aws"}}, Deployment: catalogs.ProviderDeployment{Type: "regional", Tier: "on_demand"},
		Endpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeBedrock}, Lifecycle: catalogs.OfferingLifecycleActive,
	}
}

func canonicalTestPricing(price float64) *catalogs.ModelPricing {
	return &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: price}}}
}

func canonicalProviderOnlyCatalog(t testing.TB, providerID catalogs.ProviderID) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: providerID, Name: string(providerID)}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
