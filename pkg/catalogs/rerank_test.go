package catalogs

import (
	"slices"
	"strings"
	"testing"
)

// TestRerankIsAPublishableOperation holds the contract that lets a catalog
// state a provider serves reranking. Before this operation existed the word
// only disqualified a model from the chat view, so a provider that ranked
// documents had nothing to publish and the offering failed validation.
func TestRerankIsAPublishableOperation(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID:   ProviderIDMistralAI,
		Name: "Mistral AI",
		Inference: &ProviderInference{
			BaseURL: "https://api.example",
			Endpoints: []ProviderInferenceEndpoint{
				{Operation: ProviderOperationChatCompletions, Type: EndpointTypeOpenAI, Path: "/v1/chat/completions"},
				{
					Operation:     ProviderOperationRerank,
					Type:          EndpointTypeOpenAI,
					Path:          "/v1/rerank",
					PathsByAuthor: map[AuthorID]string{"cohere": "/v2/rerank"},
				},
			},
		},
	}
	if err := provider.ValidateContract(); err != nil {
		t.Fatalf("ValidateContract: %v", err)
	}

	endpoint, found := provider.Inference.Endpoint(ProviderOperationRerank)
	if !found {
		t.Fatal("rerank endpoint not found")
	}
	if got := provider.Inference.EndpointURL(endpoint, ""); got != "https://api.example/v1/rerank" {
		t.Fatalf("rerank URL = %q", got)
	}
	if got := endpoint.PathsByAuthor["cohere"]; got != "/v2/rerank" {
		t.Fatalf("author path = %q", got)
	}
}

// TestRerankEndpointStillNeedsAPath proves the operation gained no exemption
// from the path rule. An endpoint that names the operation and no path would
// route a rerank request to the provider base URL.
func TestRerankEndpointStillNeedsAPath(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID:   ProviderIDMistralAI,
		Name: "Mistral AI",
		Inference: &ProviderInference{
			BaseURL:   "https://api.example",
			Endpoints: []ProviderInferenceEndpoint{{Operation: ProviderOperationRerank, Type: EndpointTypeOpenAI}},
		},
	}
	err := provider.ValidateContract()
	if err == nil {
		t.Fatal("ValidateContract accepted a rerank endpoint with no path")
	}
	if !strings.Contains(err.Error(), "path") {
		t.Fatalf("error = %v, want it to name the path", err)
	}
}

// TestUnknownOperationStaysRefused pins the other half of the same guard. The
// operation list grew, so the refusal message has to grow with it, and an
// operation no member names still fails.
func TestUnknownOperationStaysRefused(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID:   ProviderIDMistralAI,
		Name: "Mistral AI",
		Inference: &ProviderInference{
			BaseURL: "https://api.example",
			Endpoints: []ProviderInferenceEndpoint{
				{Operation: ProviderOperation("ranking"), Type: EndpointTypeOpenAI, Path: "/v1/ranking"},
			},
		},
	}
	err := provider.ValidateContract()
	if err == nil {
		t.Fatal("ValidateContract accepted an operation no member names")
	}
	if !strings.Contains(err.Error(), string(ProviderOperationRerank)) {
		t.Fatalf("error = %v, want it to list %s", err, ProviderOperationRerank)
	}
}

// TestRerankModelServesRerankAndNotChat holds invariant R3 of the reranking
// plan. A reranker reads text and writes text, which is the shape of a chat
// model, so the tag is the only fact that separates them. A caller that saw a
// reranker in the chat list would send it a request it cannot answer.
func TestRerankModelServesRerankAndNotChat(t *testing.T) {
	t.Parallel()

	reranker := textModel(nil, ModelTagRerank)
	got := deriveOfferingCapabilities(reranker).Operations
	if !slices.Contains(got, ProviderOperationRerank) {
		t.Fatalf("operations = %v, want the rerank operation", got)
	}
	if slices.Contains(got, ProviderOperationChatCompletions) {
		t.Fatalf("operations = %v, want no chat completions", got)
	}

	// The same modalities without the tag stay a chat model, which is why the
	// media operation table cannot name this operation.
	chat := deriveOfferingCapabilities(textModel(nil)).Operations
	if !slices.Contains(chat, ProviderOperationChatCompletions) {
		t.Fatalf("untagged operations = %v, want chat completions", chat)
	}
	if slices.Contains(chat, ProviderOperationRerank) {
		t.Fatalf("untagged operations = %v, want no rerank", chat)
	}
}

// TestRerankBasisHoldsToItsPrice holds decision RNK-D4 as amended. Four
// providers bill reranking in two different units, so the catalog records which
// unit it means beside the price. A basis without its price leaves a consumer
// nothing to charge, and a price without the basis leaves the same consumer
// guessing which unit the provider counts.
func TestRerankBasisHoldsToItsPrice(t *testing.T) {
	t.Parallel()

	tokens := &ModelTokenPricing{Input: &ModelTokenCost{Per1M: 0.02}}
	for _, testCase := range []struct {
		name    string
		pricing ModelPricing
		wantErr string
	}{
		{
			name: "search unit basis with its price",
			pricing: ModelPricing{
				Currency:   ModelPricingCurrencyUSD,
				Operations: &ModelOperationPricing{SearchUnit: price(0.001), RerankBasis: ModelRerankBasisSearchUnit},
			},
		},
		{
			name: "token basis with an input token price",
			pricing: ModelPricing{
				Currency:   ModelPricingCurrencyUSD,
				Tokens:     tokens,
				Operations: &ModelOperationPricing{RerankBasis: ModelRerankBasisToken},
			},
		},
		{
			name: "search unit basis without a search unit price",
			pricing: ModelPricing{
				Currency:   ModelPricingCurrencyUSD,
				Tokens:     tokens,
				Operations: &ModelOperationPricing{RerankBasis: ModelRerankBasisSearchUnit},
			},
			wantErr: "requires a search_unit price",
		},
		{
			name: "token basis without a token price",
			pricing: ModelPricing{
				Currency:   ModelPricingCurrencyUSD,
				Operations: &ModelOperationPricing{SearchUnit: price(0.001), RerankBasis: ModelRerankBasisToken},
			},
			wantErr: "requires an input token price",
		},
		{
			name: "a search unit price with no basis stated",
			pricing: ModelPricing{
				Currency:   ModelPricingCurrencyUSD,
				Operations: &ModelOperationPricing{SearchUnit: price(0.001)},
			},
			wantErr: "when search_unit is priced",
		},
		{
			name: "a basis no constant names",
			pricing: ModelPricing{
				Currency:   ModelPricingCurrencyUSD,
				Tokens:     tokens,
				Operations: &ModelOperationPricing{RerankBasis: ModelRerankBasis("per-document")},
			},
			wantErr: "must be one of",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := testCase.pricing.Validate()
			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate accepted %s", testCase.name)
			}
			if !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, testCase.wantErr)
			}
		})
	}
}

// TestRerankPriceDeepCopies keeps the search unit price out of the aliasing
// class. Every other operation price is copied, and a shared pointer here would
// let one offering's edit change another offering's cost.
func TestRerankPriceDeepCopies(t *testing.T) {
	t.Parallel()

	pricing := &ModelPricing{
		Currency:   ModelPricingCurrencyUSD,
		Operations: &ModelOperationPricing{SearchUnit: price(0.001), RerankBasis: ModelRerankBasisSearchUnit},
	}
	copied := deepCopyModelPricing(pricing)
	if copied.Operations.SearchUnit == nil {
		t.Fatal("copied search unit price is missing")
	}
	if *copied.Operations.SearchUnit != 0.001 {
		t.Fatalf("copied search unit price = %v, want 0.001", *copied.Operations.SearchUnit)
	}
	if copied.Operations.SearchUnit == pricing.Operations.SearchUnit {
		t.Fatal("copied search unit price aliases the source")
	}
	if copied.Operations.RerankBasis != ModelRerankBasisSearchUnit {
		t.Fatalf("copied basis = %q", copied.Operations.RerankBasis)
	}
}
