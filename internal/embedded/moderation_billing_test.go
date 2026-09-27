package embedded_test

import (
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"testing"
)

func TestEmbeddedModerationBillingContract(t *testing.T) {
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []catalogs.ProviderModelID{"omni-moderation-latest", "omni-moderation-2024-09-26"} {
		offering, err := catalog.Offering(catalogs.ProviderIDOpenAI, model)
		if err != nil {
			t.Fatal(err)
		}
		if offering.Billing == nil || offering.Billing.Validate() != nil || offering.Billing.Moderations == nil || offering.Billing.Moderations.Basis != catalogs.ModerationBillingRequests {
			t.Fatalf("missing moderation contract: %s", model)
		}
		if offering.Pricing == nil || offering.Pricing.Currency != catalogs.ModelPricingCurrencyUSD || offering.Pricing.Operations == nil || offering.Pricing.Operations.Request == nil || *offering.Pricing.Operations.Request != 0 {
			t.Fatalf("missing explicit free price: %s", model)
		}
		if _, found := offering.Endpoint(catalogs.ProviderOperationModerations); !found {
			t.Fatalf("missing moderation endpoint: %s", model)
		}
		if _, found := offering.Endpoint(catalogs.ProviderOperationChatCompletions); found {
			t.Fatalf("moderation advertises chat: %s", model)
		}
	}
}
