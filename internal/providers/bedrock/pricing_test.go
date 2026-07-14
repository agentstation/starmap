package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestParsePricingAndApplyPerRegionModes(t *testing.T) {
	payload := pricingFixture(t,
		pricingFixtureSKU("input", "InputTokenCount-Units", "3.0", "Million Input Tokens"),
		pricingFixtureSKU("output", "OutputTokenCount-Units", "15.0", "Million Output Tokens"),
		pricingFixtureSKU("global-input", "InputTokenCount_Global-Units", "2.7", "Million Input Tokens Global"),
		pricingFixtureSKU("image", "Created_image-Units", "0.04", "Image"),
	)
	prices, err := parsePricing([]byte(payload))
	if err != nil {
		t.Fatalf("parsePricing: %v", err)
	}
	if prices.Version != "20260703085857" || prices.AcceptedSKUs != 3 || prices.IgnoredSKUs != 1 || !prices.PublishedAt.Equal(time.Date(2026, 7, 3, 8, 58, 57, 0, time.UTC)) {
		t.Fatalf("pricing evidence = %#v", prices)
	}
	regional := prices.Prices[pricingKey{ServiceName: "claude sonnet", Region: "us-east-1", Mode: "regional"}]
	if regional == nil || regional.Tokens.Input.Per1M != 3 || regional.Tokens.Output.Per1M != 15 {
		t.Fatalf("regional pricing = %#v", regional)
	}

	result := Result{
		Definitions: []catalogs.ModelDefinition{{ID: "anthropic/claude", Name: "Claude Sonnet"}},
		Offerings: []catalogs.ProviderOffering{
			canonicalTestPriceOffering("anthropic.claude", "anthropic/claude", nil),
			canonicalTestPriceOffering("global.anthropic.claude", "anthropic/claude", &catalogs.CrossRegionInferenceProfile{ID: "global.anthropic.claude", Scope: "GLOBAL", DestinationRegions: []string{"us-east-1"}}),
		},
	}
	if matched := applyPricing(&result, prices); matched != 2 {
		t.Fatalf("matched prices = %d, want 2; keys=%#v", matched, prices.Prices)
	}
	if got := result.Offerings[0].Modes["regional/us-east-1"].Pricing.Tokens.Output.Per1M; got != 15 {
		t.Fatalf("regional output price = %v", got)
	}
	if got := result.Offerings[1].Modes["global/us-east-1"].Pricing.Tokens.Input.Per1M; got != 2.7 {
		t.Fatalf("global input price = %v", got)
	}
	if _, found := result.Offerings[0].Modes["global/us-east-1"]; found {
		t.Fatal("global profile pricing leaked onto regional foundation model")
	}
}

func TestParsePricingRejectsConflictingSKUPrices(t *testing.T) {
	payload := pricingFixture(t,
		pricingFixtureSKU("one", "InputTokenCount-Units", "3.0", "Million Input Tokens"),
		pricingFixtureSKU("two", "InputTokenCount-Units", "4.0", "Million Input Tokens"),
	)
	if _, err := parsePricing([]byte(payload)); err == nil {
		t.Fatal("parsePricing accepted conflicting prices for one model/region/mode/component")
	}
}

func TestLiveBedrockPublicPriceList(t *testing.T) {
	if os.Getenv("STARMAP_LIVE_BEDROCK_PRICING") != "1" {
		t.Skip("set STARMAP_LIVE_BEDROCK_PRICING=1 for first-party bulk price-list proof")
	}
	prices, err := newHTTPPricingFetcher().Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if prices.Version == "" || prices.ETag == "" || prices.AcceptedSKUs == 0 || prices.PublishedAt.IsZero() {
		t.Fatalf("live price-list evidence incomplete: version=%q etag=%q accepted=%d published=%s", prices.Version, prices.ETag, prices.AcceptedSKUs, prices.PublishedAt)
	}
	t.Logf("Bedrock price list version=%s published=%s accepted=%d ignored=%d", prices.Version, prices.PublishedAt.Format(time.RFC3339), prices.AcceptedSKUs, prices.IgnoredSKUs)
}

func pricingFixture(t testing.TB, products ...string) string {
	t.Helper()
	productJSON := ""
	termJSON := ""
	for index, product := range products {
		if index > 0 {
			productJSON += ","
			termJSON += ","
		}
		productJSON += product
		var wrapper map[string]struct {
			SKU          string `json:"sku"`
			FixturePrice string `json:"fixturePrice"`
		}
		if err := json.Unmarshal([]byte("{"+product+"}"), &wrapper); err != nil {
			t.Fatal(err)
		}
		var value struct {
			SKU          string
			FixturePrice string
		}
		for _, item := range wrapper {
			value.SKU, value.FixturePrice = item.SKU, item.FixturePrice
		}
		termJSON += fmt.Sprintf(`%q:{"term":{"priceDimensions":{"dimension":{"beginRange":"0","endRange":"Inf","unit":"Units","description":"Million tokens","pricePerUnit":{"USD":%q}}}}}`, value.SKU, value.FixturePrice)
	}
	return fmt.Sprintf(`{"formatVersion":"v1.0","offerCode":"AmazonBedrockFoundationModels","version":"20260703085857","publicationDate":"2026-07-03T08:58:57Z","products":{%s},"terms":{"OnDemand":{%s}}}`, productJSON, termJSON)
}

func pricingFixtureSKU(sku, usage, price, description string) string {
	return fmt.Sprintf(`%q:{"sku":%q,"attributes":{"servicename":"Claude Sonnet (Amazon Bedrock Edition)","regionCode":"us-east-1","usagetype":"USE1-MP:USE1_%s"},"fixturePrice":%q,"fixtureDescription":%q}`, sku, sku, usage, price, description)
}

func canonicalTestPriceOffering(modelID catalogs.ProviderModelID, definitionID catalogs.ModelDefinitionID, profile *catalogs.CrossRegionInferenceProfile) catalogs.ProviderOffering {
	return catalogs.ProviderOffering{ProviderID: ProviderID, ProviderModelID: modelID, DefinitionID: definitionID, Regions: []catalogs.CloudRegion{{ID: "us-east-1", Realm: "aws"}}, InferenceProfile: profile}
}
