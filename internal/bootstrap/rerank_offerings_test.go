package bootstrap

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestShippedRerankOfferingsCarryTheirCost reads the catalog this module
// embeds and requires two facts for every rerank offering. First, the price
// prevents spend limits from treating the request as free. Second, the document
// bound prevents callers from sending a list that the reranker will reject.
//
// The test also fails when the catalog ships no rerank offering at all. An
// operation the type system names and the data never carries is an operation
// no consumer can exercise.
func TestShippedRerankOfferingsCarryTheirCost(t *testing.T) {
	builder, err := NewEmbeddedBuilder()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	bases := map[catalogs.ModelRerankBasis]int{}
	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%s): %v", provider.ID, err)
		}
		for _, offering := range offerings {
			if !slices.Contains(offering.Service.Operations, catalogs.ProviderOperationRerank) {
				continue
			}
			name := string(provider.ID) + "/" + string(offering.ProviderModelID)

			if offering.Limits == nil || offering.Limits.MaxDocuments <= 0 {
				t.Errorf("%s serves rerank and states no document count", name)
			}

			pricing := offering.Pricing
			if pricing == nil {
				t.Errorf("%s serves rerank and carries no price", name)
				continue
			}
			operations := pricing.Operations
			if operations == nil {
				t.Errorf("%s serves rerank and names no operation price", name)
				continue
			}
			bases[operations.RerankBasis]++
			switch operations.RerankBasis {
			case catalogs.ModelRerankBasisSearchUnit:
				if operations.SearchUnit == nil || *operations.SearchUnit <= 0 {
					t.Errorf("%s bills a search unit and prices none", name)
				}
			case catalogs.ModelRerankBasisToken:
				if pricing.Tokens == nil || pricing.Tokens.Input == nil || pricing.Tokens.Input.Per1M <= 0 {
					t.Errorf("%s bills tokens and prices no input token", name)
				}
			default:
				t.Errorf("%s serves rerank and names the basis %q", name, operations.RerankBasis)
			}
		}
	}

	if len(bases) == 0 {
		t.Fatal("the shipped catalog holds no rerank offering")
	}
	for _, basis := range []catalogs.ModelRerankBasis{
		catalogs.ModelRerankBasisSearchUnit,
		catalogs.ModelRerankBasisToken,
	} {
		if bases[basis] == 0 {
			t.Errorf("no shipped rerank offering bills by the %s", basis)
		}
	}
}

func TestRerankProviderEndpointsResolve(t *testing.T) {
	builder, err := NewEmbeddedBuilder()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found := 0
	for _, provider := range catalog.Providers().List() {
		if provider.Inference == nil {
			continue
		}
		endpoint, ok := provider.Inference.Endpoint(catalogs.ProviderOperationRerank)
		if !ok {
			continue
		}
		found++
		if url := provider.Inference.EndpointURL(endpoint, ""); url == "" {
			t.Errorf("provider %s serves rerank and resolves no URL", provider.ID)
		}
		if endpoint.Type == "" {
			t.Errorf("provider %s serves rerank and names no endpoint type", provider.ID)
		}
	}
	if found == 0 {
		t.Fatal("no shipped provider publishes a rerank endpoint")
	}
}
