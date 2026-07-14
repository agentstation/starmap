package mistral_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/providers/fixtures"
	"github.com/agentstation/starmap/internal/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedMistralCompositionUsesSharedOpenAIConnector(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("testdata", "models_list.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	t.Setenv("MISTRAL_API_KEY", "mistral-fixture-key")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodGet || request.Header.Get("Authorization") != "Bearer mistral-fixture-key" {
			t.Errorf("request = %s auth %q", request.Method, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	provider := fixtures.EmbeddedProvider(t, catalogs.ProviderIDMistralAI)
	provider.Catalog.Sources[0].Endpoint.URL = server.URL
	provider.Catalog.Sources[0].Endpoint.Path = ""
	resolved, err := acquisition.NewResolver().Resolve(context.Background(), &provider, "models")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	client, err := registry.New(resolved)
	if err != nil {
		t.Fatalf("registry.New: %v", err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if requests.Load() != 1 || len(models) != 1 {
		t.Fatalf("requests/models = %d/%d", requests.Load(), len(models))
	}
	model := models[0]
	if model.ID != "mistral-small-latest" || len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDMistralAI {
		t.Fatalf("identity/authorship = %#v", model)
	}
	if model.Status != catalogs.ModelStatusActive || model.Limits == nil || model.Limits.ContextWindow != 262144 {
		t.Fatalf("lifecycle/limits = %#v/%#v", model.Status, model.Limits)
	}
	if model.Features == nil || !model.Features.Tools || !model.Features.Streaming ||
		!slices.Contains(model.Features.Modalities.Input, catalogs.ModelModalityImage) {
		t.Fatalf("configured capabilities = %#v", model.Features)
	}
	if model.Pricing != nil {
		t.Fatalf("live inventory invented curated pricing = %#v", model.Pricing)
	}
	extension := model.Extensions[string(catalogs.ProviderIDMistralAI)].Fields
	aliases, ok := extension["aliases"].([]any)
	if !ok || len(aliases) != 1 || aliases[0] != "mistral-small-2603" || extension["completion_fim"] != true {
		t.Fatalf("provider extension = %#v", extension)
	}

	provider.Models = map[string]*catalogs.Model{model.ID: &model}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDMistralAI, catalogs.ProviderModelID(model.ID))
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	definition, err := catalog.Definition(offering.DefinitionID)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if offering.ProviderID != catalogs.ProviderIDMistralAI || len(definition.AuthorIDs) != 1 || definition.AuthorIDs[0] != catalogs.AuthorIDMistralAI {
		t.Fatalf("provider/author separation = %#v/%#v", offering, definition)
	}
}

func TestMistralCuratedPricingLivesInCanonicalEmbeddedOffering(t *testing.T) {
	builder, err := catalogs.NewFromPath(filepath.Join("..", "..", "..", "internal", "embedded", "catalog"))
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDMistralAI, "mistral-small-latest")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.Pricing == nil || offering.Pricing.Currency != catalogs.ModelPricingCurrencyUSD ||
		offering.Pricing.Tokens == nil || offering.Pricing.Tokens.Input == nil || offering.Pricing.Tokens.Input.Per1M != 0.15 ||
		offering.Pricing.Tokens.Output == nil || offering.Pricing.Tokens.Output.Per1M != 0.6 {
		t.Fatalf("catalog pricing = %#v", offering.Pricing)
	}
}
