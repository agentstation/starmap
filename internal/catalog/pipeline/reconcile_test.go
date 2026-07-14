package pipeline

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestReconcileUsesSelectedSourceAsPrimaryWhenProvidersSourceAbsent(t *testing.T) {
	cat := catalogs.NewEmpty()
	model := catalogs.Model{
		ID:   "models-dev-only-model",
		Name: "models.dev Only Model",
	}
	provider := catalogs.Provider{
		ID:   "models-dev-provider",
		Name: "models.dev Provider",
		Models: map[string]*catalogs.Model{
			model.ID: &model,
		},
	}
	if err := cat.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	result, err := reconcile(context.Background(), asSnapshot(catalogs.NewEmpty()), []sources.Observation{
		{SourceID: sources.ModelsDevHTTPID, Catalog: asSnapshot(cat)},
	})
	if err != nil {
		t.Fatalf("reconcile models.dev-only source: %v", err)
	}
	if result == nil || result.Catalog == nil {
		t.Fatal("expected reconciled catalog")
	}
	if _, err := result.Catalog.FindModel(model.ID); err != nil {
		t.Fatalf("expected model from selected source: %v", err)
	}
}

func TestReconcileModelsDevOnlyEnrichesBaselineProviderData(t *testing.T) {
	baseline := catalogs.NewEmpty()
	inputCost := 1.25
	sharedRoot := "gpt-4o-root"
	sharedModel := catalogs.Model{
		ID:   "gpt-4o",
		Name: "GPT-4o",
		Lineage: &catalogs.ModelLineage{
			Root: &sharedRoot,
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 128000,
			InputTokens:   96000,
			OutputTokens:  4096,
		},
	}
	providerOnlyModel := catalogs.Model{
		ID:   "provider-only",
		Name: "Provider Only",
	}
	apiKey := catalogs.ProviderCredential{Env: catalogs.ProviderEnvironmentNames{"OPENAI_API_KEY"}}
	baselineProvider := catalogs.Provider{
		ID:          "openai",
		Name:        "OpenAI",
		Aliases:     []catalogs.ProviderID{"open-ai"},
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": apiKey},
		Models: map[string]*catalogs.Model{
			sharedModel.ID:       &sharedModel,
			providerOnlyModel.ID: &providerOnlyModel,
		},
	}
	if err := baseline.SetProvider(baselineProvider); err != nil {
		t.Fatalf("SetProvider baseline: %v", err)
	}

	modelsDev := catalogs.NewEmpty()
	modelsDevModel := catalogs.Model{
		ID:   sharedModel.ID,
		Name: "GPT-4o from models.dev",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{Per1M: inputCost},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 200000,
			InputTokens:   100000,
			OutputTokens:  8192,
		},
	}
	modelsDevProvider := catalogs.Provider{
		ID:   "openai",
		Name: "OpenAI from models.dev",
		Models: map[string]*catalogs.Model{
			modelsDevModel.ID: &modelsDevModel,
		},
	}
	if err := modelsDev.SetProvider(modelsDevProvider); err != nil {
		t.Fatalf("SetProvider models.dev: %v", err)
	}

	result, err := reconcile(context.Background(), asSnapshot(baseline), []sources.Observation{
		{SourceID: sources.ModelsDevHTTPID, Catalog: asSnapshot(modelsDev)},
	})
	if err != nil {
		t.Fatalf("reconcile models.dev-only source: %v", err)
	}

	provider, err := result.Catalog.Provider("openai")
	if err != nil {
		t.Fatalf("provider not found: %v", err)
	}
	if credential, found := provider.Credentials["api_key"]; !found || len(credential.Env) != 1 || credential.Env[0] != apiKey.Env[0] {
		t.Fatalf("provider API key was not preserved: %#v", provider.Credentials)
	}
	if len(provider.Aliases) != 1 || provider.Aliases[0] != "open-ai" {
		t.Fatalf("provider aliases were not preserved: %#v", provider.Aliases)
	}
	if provider.Models[providerOnlyModel.ID] == nil {
		t.Fatalf("baseline-only model %q was removed by models.dev enrichment", providerOnlyModel.ID)
	}
	got := provider.Models[sharedModel.ID]
	if got == nil {
		t.Fatalf("shared model %q missing", sharedModel.ID)
	}
	if got.Lineage == nil || got.Lineage.Root == nil || *got.Lineage.Root != sharedRoot {
		t.Fatalf("baseline lineage root was not preserved for matching model: %#v", got.Lineage)
	}
	if got.Pricing == nil ||
		got.Pricing.Tokens == nil ||
		got.Pricing.Tokens.Input == nil ||
		got.Pricing.Tokens.Input.Per1M != inputCost {
		t.Fatalf("models.dev pricing was not applied: %#v", got.Pricing)
	}
	if got.Limits == nil || got.Limits.InputTokens != modelsDevModel.Limits.InputTokens {
		t.Fatalf("models.dev limits were not applied: %#v", got.Limits)
	}
}

func TestReconcileModelsDevPrimaryIsRestrictedToBaselineProviders(t *testing.T) {
	baseline := catalogs.NewEmpty()
	baselineModel := catalogs.Model{
		ID:   "configured-model",
		Name: "Configured Model",
	}
	if err := baseline.SetProvider(catalogs.Provider{
		ID:   "configured-provider",
		Name: "Configured Provider",
		Models: map[string]*catalogs.Model{
			baselineModel.ID: &baselineModel,
		},
	}); err != nil {
		t.Fatalf("SetProvider baseline: %v", err)
	}

	modelsDev := catalogs.NewEmpty()
	configuredModelsDevModel := catalogs.Model{
		ID:   baselineModel.ID,
		Name: "Configured Model from models.dev",
	}
	if err := modelsDev.SetProvider(catalogs.Provider{
		ID:   "configured-provider",
		Name: "Configured Provider from models.dev",
		Models: map[string]*catalogs.Model{
			configuredModelsDevModel.ID: &configuredModelsDevModel,
		},
	}); err != nil {
		t.Fatalf("SetProvider configured models.dev: %v", err)
	}
	unconfiguredModel := catalogs.Model{
		ID:   "unconfigured-model",
		Name: "Unconfigured Model",
	}
	if err := modelsDev.SetProvider(catalogs.Provider{
		ID:   "unconfigured-provider",
		Name: "Unconfigured Provider from models.dev",
		Models: map[string]*catalogs.Model{
			unconfiguredModel.ID: &unconfiguredModel,
		},
	}); err != nil {
		t.Fatalf("SetProvider unconfigured models.dev: %v", err)
	}

	result, err := reconcile(context.Background(), asSnapshot(baseline), []sources.Observation{
		{SourceID: sources.ModelsDevHTTPID, Catalog: asSnapshot(modelsDev)},
	})
	if err != nil {
		t.Fatalf("reconcile models.dev-only source: %v", err)
	}

	if _, err := result.Catalog.Provider("configured-provider"); err != nil {
		t.Fatalf("configured provider missing: %v", err)
	}
	if _, err := result.Catalog.Provider("unconfigured-provider"); err == nil {
		t.Fatal("unconfigured models.dev provider was imported into baseline-scoped catalog")
	}
	if _, err := result.Catalog.FindModel(unconfiguredModel.ID); err == nil {
		t.Fatal("unconfigured models.dev model was imported into baseline-scoped catalog")
	}
}

func TestFilterCatalogToBaselineProvidersReturnsSetProviderError(t *testing.T) {
	baseline := catalogs.NewEmpty()
	if err := baseline.SetProvider(catalogs.Provider{
		ID:      "openai",
		Name:    "OpenAI",
		Aliases: []catalogs.ProviderID{"open-ai"},
	}); err != nil {
		t.Fatalf("SetProvider baseline: %v", err)
	}

	sourceCatalog := catalogs.NewEmpty()
	if err := sourceCatalog.SetProvider(catalogs.Provider{
		ID:   "open-ai",
		Name: "OpenAI from models.dev",
	}); err != nil {
		t.Fatalf("SetProvider source: %v", err)
	}

	setErr := stderrors.New("set provider failed")
	err := setBaselineProviders(
		&setProviderFailingCatalog{
			Builder: catalogs.NewEmpty(),
			err:     setErr,
		},
		sourceCatalog,
		baseline,
	)
	if !stderrors.Is(err, setErr) {
		t.Fatalf("expected wrapped SetProvider error, got %v", err)
	}
	if !strings.Contains(err.Error(), "openai") {
		t.Fatalf("expected error to include baseline provider ID, got %v", err)
	}
}

type setProviderFailingCatalog struct {
	*catalogs.Builder
	err error
}

func (c *setProviderFailingCatalog) SetProvider(catalogs.Provider) error {
	return c.err
}
