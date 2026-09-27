package embedded_test

import (
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"testing"
)

func TestEmbeddedTokenRerankBilling(t *testing.T) {
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	for model, price := range map[catalogs.ProviderModelID]float64{"rerank-2.5": 0.05, "rerank-2.5-lite": 0.02} {
		offering, err := catalog.Offering("voyage", model)
		if err != nil {
			t.Fatal(err)
		}
		if offering.Billing == nil || offering.Billing.Validate() != nil || offering.Billing.Rerank == nil {
			t.Fatal("missing complete rerank billing")
		}
		b := offering.Billing.Rerank
		if b.Basis != catalogs.RerankBillingQueryDocumentTokens || b.RequestCharge == nil || *b.RequestCharge {
			t.Fatal("unexpected rerank billing")
		}
		if offering.Limits == nil || offering.Limits.ContextWindow != 32000 || offering.Limits.InputTokens != 600000 || offering.Limits.MaxDocuments != 1000 {
			t.Fatal("missing query-pair and request bounds")
		}
		if offering.Pricing == nil || offering.Pricing.Tokens == nil || offering.Pricing.Tokens.Input == nil || offering.Pricing.Tokens.Input.Per1M != price {
			t.Fatal("wrong token price")
		}
		if _, ok := offering.Endpoint(catalogs.ProviderOperationRerank); !ok {
			t.Fatal("missing rerank endpoint")
		}
	}
}
