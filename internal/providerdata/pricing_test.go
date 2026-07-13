package providerdata

import (
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestParsePricingCatalog(t *testing.T) {
	payload := []byte(`
provider_id: example
evidence:
  url: https://example.com/pricing
  revision: effective-2026-07-12
  observed_at: 2026-07-12T00:00:00Z
  scope: public-serverless
models:
  example-model:
    pricing:
      currency: USD
      tokens:
        input: {per_1m: 1}
        output: {per_1m: 2}
`)
	catalog, err := ParsePricingCatalog("example", payload)
	if err != nil {
		t.Fatalf("ParsePricingCatalog: %v", err)
	}
	if catalog.Models["example-model"].Pricing.Tokens.Output.Per1M != 2 {
		t.Fatalf("catalog = %#v", catalog)
	}

	for name, invalid := range map[string][]byte{
		"provider mismatch": []byte(strings.Replace(string(payload), "provider_id: example", "provider_id: other", 1)),
		"secret URL":        []byte(strings.Replace(string(payload), "https://example.com/pricing", "file:///secret", 1)),
		"negative price":    []byte(strings.Replace(string(payload), "per_1m: 2", "per_1m: -2", 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePricingCatalog(catalogs.ProviderID("example"), invalid); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}
