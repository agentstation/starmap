package cohere

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestListModelsPaginatesMapsNativeFactsAndExcludesCustomerFineTunes(t *testing.T) {
	t.Setenv("COHERE_API_KEY", "cohere-fixture-key")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Header.Get("Authorization") != "Bearer cohere-fixture-key" || request.URL.Query().Get("page_size") != "1000" {
			t.Errorf("auth/page_size = %q/%q", request.Header.Get("Authorization"), request.URL.Query().Get("page_size"))
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Query().Get("page_token") {
		case "":
			_, _ = writer.Write([]byte(`{"models":[{"name":"command-r-plus-08-2024","is_deprecated":false,"endpoints":["chat"],"finetuned":false,"context_length":128000,"tokenizer_url":"https://models.cohere.com/tokenizers/command.json","default_endpoints":["chat"],"features":["instruction-following","tool-use"],"sampling_defaults":{"temperature":0.3,"k":40,"p":0.9,"frequency_penalty":0,"presence_penalty":0}},{"name":"customer-private-tune","endpoints":["chat"],"finetuned":true,"context_length":4096}],"next_page_token":"page-two"}`))
		case "page-two":
			_, _ = writer.Write([]byte(`{"models":[{"name":"embed-v4.0","is_deprecated":false,"endpoints":["embed"],"finetuned":false,"context_length":128000,"tokenizer_url":"https://models.cohere.com/tokenizers/embed.json","features":[]}],"next_page_token":""}`))
		default:
			t.Errorf("unexpected cursor %q", request.URL.Query().Get("page_token"))
		}
	}))
	defer server.Close()
	provider := testProvider(server.URL)
	models, err := NewClient(testsource.Authenticated(t, provider)).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if requests.Load() != 2 || len(models) != 2 {
		t.Fatalf("requests/models = %d/%#v", requests.Load(), models)
	}
	command := models[0]
	if command.ID != "command-r-plus-08-2024" || command.Status != catalogs.ModelStatusActive ||
		command.Limits == nil || command.Limits.ContextWindow != 128000 ||
		!slices.Equal(command.InvocationAPIs, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}) {
		t.Fatalf("command mapping = %#v", command)
	}
	if command.Metadata == nil || command.Metadata.Architecture == nil || command.Metadata.Architecture.Tokenizer != catalogs.TokenizerCohere ||
		command.Features == nil || !command.Features.Tools || !command.Features.Temperature || !command.Features.TopK || !command.Features.TopP {
		t.Fatalf("command capabilities = %#v/%#v", command.Metadata, command.Features)
	}
	if command.Pricing != nil {
		t.Fatalf("live inventory invented curated pricing = %#v", command.Pricing)
	}
	embed := models[1]
	if !slices.Equal(embed.InvocationAPIs, []catalogs.InvocationAPI{catalogs.InvocationAPIEmbeddings}) ||
		!slices.Equal(embed.Features.Modalities.Output, []catalogs.ModelModality{catalogs.ModelModalityEmbedding}) || embed.Pricing != nil {
		t.Fatalf("embed mapping = %#v", embed)
	}
}

func TestCohereCuratedPricingLivesInEmbeddedCatalog(t *testing.T) {
	builder, err := catalogs.NewFromPath(filepath.Join("..", "..", "..", "internal", "embedded", "catalog"))
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDCohere, "command-r-plus-08-2024")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.Pricing == nil || offering.Pricing.Currency != catalogs.ModelPricingCurrencyUSD ||
		offering.Pricing.Tokens == nil || offering.Pricing.Tokens.Input == nil || offering.Pricing.Tokens.Input.Per1M != 2.5 ||
		offering.Pricing.Tokens.Output == nil || offering.Pricing.Tokens.Output.Per1M != 10 {
		t.Fatalf("catalog pricing = %#v", offering.Pricing)
	}
}

func TestCohereCanonicalOfferingsPreserveInvocationAPIAndChannelSeparation(t *testing.T) {
	chat, err := convertModel(model{Name: "command-r-plus-08-2024", Endpoints: []string{"chat"}, ContextLength: 128000})
	if err != nil {
		t.Fatalf("convert chat: %v", err)
	}
	embed, err := convertModel(model{Name: "embed-v4.0", Endpoints: []string{"embed"}, ContextLength: 128000})
	if err != nil {
		t.Fatalf("convert embed: %v", err)
	}
	provider := testProvider("https://api.cohere.com/v1/models")
	provider.Models = map[string]*catalogs.Model{chat.ID: &chat, embed.ID: &embed}
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: catalogs.AuthorIDCohere, Name: "Cohere"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	chatOffering, err := catalog.Offering(catalogs.ProviderIDCohere, catalogs.ProviderModelID(chat.ID))
	if err != nil {
		t.Fatalf("chat offering: %v", err)
	}
	embedOffering, err := catalog.Offering(catalogs.ProviderIDCohere, catalogs.ProviderModelID(embed.ID))
	if err != nil {
		t.Fatalf("embed offering: %v", err)
	}
	if !slices.Equal(chatOffering.Access.APIs, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}) ||
		!slices.Equal(embedOffering.Access.APIs, []catalogs.InvocationAPI{catalogs.InvocationAPIEmbeddings}) ||
		chatOffering.Endpoint.Type != catalogs.EndpointTypeCohere || embedOffering.Endpoint.Type != catalogs.EndpointTypeCohere {
		t.Fatalf("offering channels = %#v/%#v", chatOffering, embedOffering)
	}
	definition, err := catalog.Definition(chatOffering.DefinitionID)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if len(definition.AuthorIDs) != 1 || definition.AuthorIDs[0] != catalogs.AuthorIDCohere || chatOffering.ProviderID != catalogs.ProviderIDCohere {
		t.Fatalf("author/provider separation = %#v/%#v", definition, chatOffering)
	}
	chat.InvocationAPIs[0] = catalogs.InvocationAPIRerank
	readback, _ := catalog.Offering(catalogs.ProviderIDCohere, catalogs.ProviderModelID(chat.ID))
	if !slices.Equal(readback.Access.APIs, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}) {
		t.Fatalf("catalog aliased source invocation APIs: %#v", readback.Access.APIs)
	}
}

func TestListModelsFailsClosedOnMalformedEnvelopeAndRepeatedCursor(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantConflict bool
	}{
		{name: "missing models", body: `{"next_page_token":""}`},
		{name: "repeated cursor", body: `{"models":[],"next_page_token":"same"}`, wantConflict: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				if test.wantConflict && request.URL.Query().Get("page_token") == "same" {
					_, _ = writer.Write([]byte(`{"models":[],"next_page_token":"same"}`))
					return
				}
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			provider := testProvider(server.URL)
			provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}
			_, err := NewClient(testsource.Authenticated(t, provider)).ListModels(context.Background())
			if err == nil {
				t.Fatal("expected failure")
			}
			if test.wantConflict {
				var conflict *pkgerrors.ConflictError
				if !stderrors.As(err, &conflict) {
					t.Fatalf("error = %T %v, want ConflictError", err, err)
				}
			}
		})
	}
}

func TestDecodeModelsRejectsDuplicateIdentity(t *testing.T) {
	client := NewClient(testsource.Unauthenticated(t, testProvider("https://example.test/models")))
	if _, err := client.DecodeModels([]byte(`{"models":[{"name":"duplicate"},{"name":"duplicate"}]}`)); err == nil {
		t.Fatal("DecodeModels accepted duplicate model identity")
	}
}

func testProvider(endpoint string) *catalogs.Provider {
	return &catalogs.Provider{
		ID: catalogs.ProviderIDCohere, Name: "Cohere",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"COHERE_API_KEY"}}},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth:     catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeCohere, URL: endpoint}, Authors: []catalogs.AuthorID{catalogs.AuthorIDCohere},
		}}},
	}
}
