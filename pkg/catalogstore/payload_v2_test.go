package catalogstore

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCanonicalPayloadV2RoundTripPreservesEnterpriseOffering(t *testing.T) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "aws-bedrock", Name: "Amazon Bedrock"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	definition := catalogs.ModelDefinition{ID: "anthropic/claude", Name: "Claude"}
	if err := builder.SetDefinition(definition); err != nil {
		t.Fatalf("SetDefinition: %v", err)
	}
	offering := catalogs.ProviderOffering{
		ProviderID: "aws-bedrock", ProviderModelID: "us.anthropic.claude-v1:0", DefinitionID: definition.ID,
		Availability:     catalogs.OfferingAvailabilityAvailable,
		Access:           catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIMessages}},
		Regions:          []catalogs.CloudRegion{{ID: "us-east-1", Realm: "aws", Residency: &catalogs.GeographicBoundary{ID: "us", Kind: catalogs.GeographicBoundaryGeography, Countries: []string{"US"}}}},
		Deployment:       catalogs.ProviderDeployment{Type: "cross_region", Tier: "on_demand"},
		InferenceProfile: &catalogs.CrossRegionInferenceProfile{ID: "us.anthropic.claude-v1:0", Scope: "US", SourceRegions: []string{"us-east-1"}, DestinationRegions: []string{"us-east-1", "us-west-2"}},
		Endpoint:         catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeAnthropic, BaseURL: "https://bedrock-runtime.us-east-1.amazonaws.com"},
		Lifecycle:        catalogs.OfferingLifecycleActive,
	}
	if err := builder.SetOffering(offering); err != nil {
		t.Fatalf("SetOffering: %v", err)
	}

	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	if bytes.Contains(payload, []byte("customer_inventory")) || bytes.Contains(payload, []byte("account_id")) {
		t.Fatalf("public payload contains customer inventory fields: %s", payload)
	}
	decoded, err := DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatalf("DecodeCatalogPayload: %v", err)
	}
	got, err := decoded.Offering("aws-bedrock", offering.ProviderModelID)
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if got.InferenceProfile == nil || len(got.InferenceProfile.DestinationRegions) != 2 || got.Regions[0].Realm != "aws" {
		t.Fatalf("enterprise offering was not preserved: %#v", got)
	}

	got.InferenceProfile.DestinationRegions[0] = "mutated"
	again, err := decoded.Offering("aws-bedrock", offering.ProviderModelID)
	if err != nil || again.InferenceProfile.DestinationRegions[0] != "us-east-1" {
		t.Fatalf("decoded catalog leaked mutable state: (%#v, %v)", again, err)
	}
}

func TestDecodedV2CatalogRemainsLastKnownGoodAcrossRestart(t *testing.T) {
	definition := catalogs.ModelDefinition{ID: "mistral/model", Name: "Mistral Model"}
	offering := catalogs.ProviderOffering{
		ProviderID: catalogs.ProviderIDMistralAI, ProviderModelID: "mistral-model", DefinitionID: definition.ID,
		Pricing:      &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: 1.5}}},
		Availability: catalogs.OfferingAvailabilityAvailable,
		Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}},
		Deployment:   catalogs.ProviderDeployment{Type: "serverless"}, Endpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOpenAI}, Lifecycle: catalogs.OfferingLifecycleActive,
	}
	baselineBuilder := catalogs.NewEmpty()
	if err := baselineBuilder.SetProvider(catalogs.Provider{ID: catalogs.ProviderIDMistralAI, Name: "Mistral AI"}); err != nil {
		t.Fatal(err)
	}
	if err := baselineBuilder.SetDefinition(definition); err != nil {
		t.Fatal(err)
	}
	if err := baselineBuilder.SetOffering(offering); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeCatalogPayload(baselineBuilder)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatal(err)
	}

	inventoryOffering := offering
	inventoryOffering.Pricing = nil
	inventoryBuilder := catalogs.NewEmpty()
	if err := inventoryBuilder.SetProvider(catalogs.Provider{ID: catalogs.ProviderIDMistralAI, Name: "Mistral AI"}); err != nil {
		t.Fatal(err)
	}
	if err := inventoryBuilder.SetDefinition(definition); err != nil {
		t.Fatal(err)
	}
	if err := inventoryBuilder.SetOffering(inventoryOffering); err != nil {
		t.Fatal(err)
	}
	inventory, err := inventoryBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	merge, err := reconciler.New(reconciler.WithBaseline(restarted))
	if err != nil {
		t.Fatal(err)
	}
	result, err := merge.Sources(context.Background(), "", []sources.Observation{{
		SourceID: sources.ProvidersID, Catalog: inventory,
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	}})
	if err != nil {
		t.Fatal(err)
	}
	published, err := result.Catalog.Build()
	if err != nil {
		t.Fatal(err)
	}
	got, err := published.Offering(catalogs.ProviderIDMistralAI, "mistral-model")
	if err != nil || got.Pricing == nil || got.Pricing.Tokens.Input.Per1M != 1.5 {
		t.Fatalf("restart last-known-good pricing = %#v/%v", got.Pricing, err)
	}
}

func TestCanonicalPayloadV2RejectsDuplicateIdentities(t *testing.T) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetDefinition(catalogs.ModelDefinition{ID: "model", Name: "Model"}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		t.Fatal(err)
	}
	definitions := object["definitions"].([]any)
	object["definitions"] = append(definitions, definitions[0])
	duplicate, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCatalogPayload(duplicate); err == nil {
		t.Fatal("DecodeCatalogPayload accepted duplicate definition identity")
	}
}

func TestCatalogPayloadV1FailsIncompatibly(t *testing.T) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "legacy", Name: "Legacy"}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		t.Fatal(err)
	}
	object["schema_version"] = float64(1)
	delete(object, "definitions")
	delete(object, "offerings")
	legacy, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCatalogPayload(legacy); err == nil {
		t.Fatal("DecodeCatalogPayload accepted prelaunch schema v1")
	}
}

func TestCatalogPayloadV2RequiresDefinitionsAndOfferings(t *testing.T) {
	builder := catalogs.NewEmpty()
	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"definitions", "offerings"} {
		var object map[string]any
		if err := json.Unmarshal(payload, &object); err != nil {
			t.Fatal(err)
		}
		delete(object, field)
		candidate, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeCatalogPayload(candidate); err == nil {
			t.Fatalf("DecodeCatalogPayload accepted missing %s", field)
		}
	}
}

func TestCanonicalPayloadV2RejectsOfferingWithMissingDefinition(t *testing.T) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetOffering(catalogs.ProviderOffering{
		ProviderID: "provider", ProviderModelID: "model", DefinitionID: "missing",
		Availability: catalogs.OfferingAvailabilityAvailable,
		Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}},
		Deployment:   catalogs.ProviderDeployment{Type: "serverless"}, Endpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOpenAI}, Lifecycle: catalogs.OfferingLifecycleActive,
	}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCatalogPayload(payload); err == nil {
		t.Fatal("DecodeCatalogPayload accepted offering with missing definition")
	}
}
