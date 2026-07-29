package openrouter

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestProjectModelUsesCanonicalDefinitionAndPreferredProviderPrice(t *testing.T) {
	t.Parallel()

	catalog := testCatalog(t)
	model, err := ProjectModel(catalog, "moonshot", "kimi-k2.5", "/api/v1")
	if err != nil {
		t.Fatalf("ProjectModel: %v", err)
	}
	if model.ID != "moonshot-ai/kimi-k2.5" ||
		model.CanonicalSlug != "moonshot-ai/kimi-k2.5" ||
		model.Name != "Kimi K2.5" {
		t.Fatalf("model identity = %#v", model)
	}
	if model.Links.Details != "/api/v1/models/moonshot-ai/kimi-k2.5/endpoints" {
		t.Fatalf("details link = %q", model.Links.Details)
	}
	if model.Created != time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("created = %d, want release timestamp", model.Created)
	}
	if model.KnowledgeCutoff == nil || *model.KnowledgeCutoff != "2024-11-01" {
		t.Fatalf("knowledge cutoff = %#v", model.KnowledgeCutoff)
	}
	if model.ContextLength != 262_144 {
		t.Fatalf("context length = %d, want maximum eligible provider limit", model.ContextLength)
	}
	if model.Pricing == nil || model.Pricing.Prompt == nil ||
		*model.Pricing.Prompt != "0.000001" ||
		model.Pricing.Completion == nil ||
		*model.Pricing.Completion != "0.000005" {
		t.Fatalf("preferred provider pricing = %#v", model.Pricing)
	}
	if model.TopProvider == nil ||
		model.TopProvider.ContextLength == nil ||
		*model.TopProvider.ContextLength != 131_072 ||
		model.TopProvider.MaxCompletionTokens == nil ||
		*model.TopProvider.MaxCompletionTokens != 8_192 ||
		model.TopProvider.IsModerated == nil ||
		!*model.TopProvider.IsModerated {
		t.Fatalf("top provider = %#v", model.TopProvider)
	}
	if model.Architecture.Modality != "text+image+file->text" ||
		model.Architecture.Tokenizer != "Qwen3" {
		t.Fatalf("architecture = %#v", model.Architecture)
	}
	wantParameters := []string{
		"tools", "tool_choice", "reasoning", "temperature", "top_p",
		"max_tokens", "stop", "structured_outputs",
	}
	if strings.Join(model.SupportedParameters, ",") != strings.Join(wantParameters, ",") {
		t.Fatalf(
			"supported parameters = %#v, want %#v",
			model.SupportedParameters,
			wantParameters,
		)
	}
	if model.DefaultParameters["temperature"] != 0.7 ||
		model.DefaultParameters["max_tokens"] != 2048 {
		t.Fatalf("default parameters = %#v", model.DefaultParameters)
	}
}

func TestProjectEndpointsPreservesProviderIdentityAndOmitsTelemetry(t *testing.T) {
	t.Parallel()

	catalog := testCatalog(t)
	endpoints, err := ProjectEndpoints(catalog, "moonshot-ai", "kimi-k2.5")
	if err != nil {
		t.Fatalf("ProjectEndpoints: %v", err)
	}
	if len(endpoints.Endpoints) != 2 {
		t.Fatalf("endpoints = %#v, want two eligible provider offerings", endpoints.Endpoints)
	}
	first, second := endpoints.Endpoints[0], endpoints.Endpoints[1]
	if first.Tag != "alibaba" ||
		first.ProviderName != "Alibaba Cloud" ||
		first.ModelID != "kimi-k2.5" {
		t.Fatalf("first endpoint = %#v", first)
	}
	if second.Tag != "deepinfra" ||
		second.ProviderName != "DeepInfra" ||
		second.ModelID != "moonshotai/Kimi-K2.5" {
		t.Fatalf("second endpoint = %#v", second)
	}
	if first.SupportsImplicitCaching || second.SupportsImplicitCaching {
		t.Fatalf(
			"implicit caching = (%t, %t), want (false, false) without an explicit source fact",
			first.SupportsImplicitCaching,
			second.SupportsImplicitCaching,
		)
	}
	data, err := json.Marshal(EndpointsEnvelope{Data: endpoints})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, forbidden := range []string{
		"latency_last_30m",
		"throughput_last_30m",
		"uptime_last_1d",
		"uptime_last_30m",
		"uptime_last_5m",
	} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("response invented telemetry field %q: %s", forbidden, data)
		}
	}
}

func TestProjectVariantUsesOnlyExplicitProviderModes(t *testing.T) {
	t.Parallel()

	catalog := testCatalog(t)
	model, err := ProjectModel(
		catalog,
		"moonshot-ai",
		"kimi-k2.5:free",
		"/api/v1",
	)
	if err != nil {
		t.Fatalf("ProjectModel variant: %v", err)
	}
	if model.ID != "moonshot-ai/kimi-k2.5:free" ||
		model.CanonicalSlug != "moonshot-ai/kimi-k2.5" ||
		model.Name != "Kimi K2.5 (free)" {
		t.Fatalf("variant model = %#v", model)
	}
	if model.Pricing == nil || model.Pricing.Prompt == nil ||
		*model.Pricing.Prompt != "0" {
		t.Fatalf("variant pricing = %#v, want explicit free mode", model.Pricing)
	}
	endpoints, err := ProjectEndpoints(
		catalog,
		"moonshot-ai",
		"kimi-k2.5:free",
	)
	if err != nil {
		t.Fatalf("ProjectEndpoints variant: %v", err)
	}
	if len(endpoints.Endpoints) != 1 ||
		endpoints.Endpoints[0].Tag != "alibaba" {
		t.Fatalf("variant endpoints = %#v, want explicit Alibaba mode only", endpoints.Endpoints)
	}

	_, err = ProjectModel(
		catalog,
		"moonshot-ai",
		"kimi-k2.5:unknown",
		"/api/v1",
	)
	var notFound *pkgerrors.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("unknown variant error = %T %v, want NotFoundError", err, err)
	}
}

func TestResolveKnownSlugAliasAndRejectsAmbiguousAlias(t *testing.T) {
	t.Parallel()

	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "lab", Aliases: []catalogs.AuthorID{"laboratory"}, Name: "Lab"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	for _, slug := range []string{"model-a", "model-b"} {
		if err := builder.SetAuthorModel(author.ID, catalogs.Model{
			ID: slug, Name: slug, Authors: []catalogs.Author{author},
		}); err != nil {
			t.Fatalf("SetAuthorModel(%s): %v", slug, err)
		}
	}
	for _, provider := range []catalogs.Provider{
		{
			ID: "provider-a", Name: "Provider A",
			Models: map[string]*catalogs.Model{
				"known-alias": {
					ID: "known-alias", ModelRef: "lab/model-a", Name: "A",
				},
				"ambiguous": {
					ID: "ambiguous", ModelRef: "lab/model-a", Name: "A",
				},
			},
		},
		{
			ID: "provider-b", Name: "Provider B",
			Models: map[string]*catalogs.Model{
				"ambiguous": {
					ID: "ambiguous", ModelRef: "lab/model-b", Name: "B",
				},
			},
		},
	} {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	model, err := ProjectModel(catalog, "laboratory", "known-alias", "/api/v1")
	if err != nil {
		t.Fatalf("ProjectModel known alias: %v", err)
	}
	if model.ID != "lab/model-a" {
		t.Fatalf("known alias resolved to %q, want lab/model-a", model.ID)
	}
	_, err = ProjectModel(catalog, "lab", "ambiguous", "/api/v1")
	var conflict *pkgerrors.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("ambiguous alias error = %T %v, want ConflictError", err, err)
	}
}

func TestEligibleOfferingsExcludeUnavailableAndRetiredRecords(t *testing.T) {
	t.Parallel()

	offerings := []catalogs.ProviderOffering{
		{ProviderID: "a"},
		{ProviderID: "b", Availability: catalogs.OfferingAvailabilityUnavailable},
		{ProviderID: "c", Lifecycle: catalogs.OfferingLifecycleRetired},
		{ProviderID: "d", Availability: catalogs.OfferingAvailabilityRestricted},
	}
	got := eligibleOfferings(offerings)
	if len(got) != 2 || got[0].ProviderID != "a" || got[1].ProviderID != "d" {
		t.Fatalf("eligible offerings = %#v, want unknown and restricted records", got)
	}
}

func TestHTTPContractUsesDataAndNumericErrorEnvelopes(t *testing.T) {
	t.Parallel()

	success := httptest.NewRecorder()
	WriteModel(success, Model{ID: "lab/model"})
	if success.Code != http.StatusOK ||
		!strings.Contains(success.Body.String(), `"data"`) ||
		strings.Contains(success.Body.String(), `"error"`) {
		t.Fatalf("success response = %d %s", success.Code, success.Body.String())
	}

	failure := httptest.NewRecorder()
	WriteError(failure, http.StatusNotFound, "Resource not found")
	if failure.Code != http.StatusNotFound ||
		failure.Body.String() != `{"error":{"code":404,"message":"Resource not found"}}`+"\n" {
		t.Fatalf("error response = %d %s", failure.Code, failure.Body.String())
	}

	internalFailure := httptest.NewRecorder()
	WriteProjectionError(internalFailure, &pkgerrors.ValidationError{
		Field: "openrouter.provider", Message: "referenced provider is unavailable",
	})
	if internalFailure.Code != http.StatusInternalServerError ||
		internalFailure.Body.String() !=
			`{"error":{"code":500,"message":"Internal Server Error"}}`+"\n" {
		t.Fatalf(
			"internal error response = %d %s",
			internalFailure.Code,
			internalFailure.Body.String(),
		)
	}
}

func TestEndpointTelemetryEncodesOnlyWhenSupplied(t *testing.T) {
	t.Parallel()

	uptime := 99.9
	endpoint := Endpoint{
		LatencyLast30m: &Percentiles{P50: 0.25, P99: 0.85},
		UptimeLast5m:   &uptime,
	}
	data, err := json.Marshal(endpoint)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"latency_last_30m"`) ||
		!strings.Contains(string(data), `"uptime_last_5m":99.9`) ||
		strings.Contains(string(data), `"throughput_last_30m"`) {
		t.Fatalf("optional telemetry JSON = %s", data)
	}
}

func TestCompatibilityPathMatchesOnlyExactRoutes(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v1/model/openai/gpt-4o",
		"/api/v1/models/openai/gpt-4o/endpoints",
	} {
		if !IsCompatibilityPath(path, "/api/v1") {
			t.Fatalf("IsCompatibilityPath(%q) = false", path)
		}
	}
	for _, path := range []string{
		"/api/v1/model/openai",
		"/api/v1/model/openai/gpt-4o/extra",
		"/api/v1/models/openai/gpt-4o",
		"/api/v1/models/openai/gpt-4o/endpoints/extra",
	} {
		if IsCompatibilityPath(path, "/api/v1") {
			t.Fatalf("IsCompatibilityPath(%q) = true", path)
		}
	}
}

func TestEmbeddedCatalogProjectsEveryCanonicalDefinition(t *testing.T) {
	t.Parallel()

	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	catalog := client.Catalog()
	for _, definition := range catalog.Definitions() {
		authorID, slug, err := catalogs.ParseModelDefinitionID(definition.ID)
		if err != nil {
			t.Fatalf("ParseModelDefinitionID(%s): %v", definition.ID, err)
		}
		model, err := ProjectModel(catalog, authorID, slug, "/api/v1")
		if err != nil {
			t.Fatalf("ProjectModel(%s): %v", definition.ID, err)
		}
		if model.ID != string(definition.ID) ||
			model.CanonicalSlug != string(definition.ID) {
			t.Fatalf("ProjectModel(%s) identity = %#v", definition.ID, model)
		}
		endpoints, err := ProjectEndpoints(catalog, authorID, slug)
		if err != nil {
			t.Fatalf("ProjectEndpoints(%s): %v", definition.ID, err)
		}
		if endpoints.ID != string(definition.ID) {
			t.Fatalf("ProjectEndpoints(%s) ID = %q", definition.ID, endpoints.ID)
		}
		if _, err := json.Marshal(ModelEnvelope{Data: model}); err != nil {
			t.Fatalf("Marshal model %s: %v", definition.ID, err)
		}
		if _, err := json.Marshal(EndpointsEnvelope{Data: endpoints}); err != nil {
			t.Fatalf("Marshal endpoints %s: %v", definition.ID, err)
		}
	}
}

func testCatalog(t testing.TB) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	author := catalogs.Author{
		ID: "moonshot-ai", Aliases: []catalogs.AuthorID{"moonshot"}, Name: "Moonshot AI",
	}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	knowledgeCutoff := utc.New(time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC))
	defaultEffort := catalogs.ModelControlLevelMedium
	maxTokens := 2048
	if err := builder.SetAuthorModel(author.ID, catalogs.Model{
		ID: "kimi-k2.5", Name: "Kimi K2.5", Description: "Canonical description.",
		Authors: []catalogs.Author{author},
		Metadata: &catalogs.ModelMetadata{
			ReleaseDate:     utc.New(time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)),
			KnowledgeCutoff: &knowledgeCutoff,
			Architecture: &catalogs.ModelArchitecture{
				Tokenizer:    catalogs.TokenizerQwen3,
				Quantization: catalogs.QuantizationFP16,
			},
		},
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input: []catalogs.ModelModality{
					catalogs.ModelModalityText,
					catalogs.ModelModalityImage,
					catalogs.ModelModalityPDF,
				},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
			Tools:             true,
			ToolChoice:        true,
			Reasoning:         true,
			Temperature:       true,
			TopP:              true,
			MaxTokens:         true,
			Stop:              true,
			StructuredOutputs: true,
		},
		Generation: &catalogs.ModelGeneration{
			Temperature: &catalogs.FloatRange{Min: 0, Max: 2, Default: 0.7},
			MaxTokens:   &maxTokens,
		},
		Reasoning: &catalogs.ModelControlLevels{
			Levels: []catalogs.ModelControlLevel{
				catalogs.ModelControlLevelLow,
				catalogs.ModelControlLevelMedium,
				catalogs.ModelControlLevelHigh,
			},
			Default: &defaultEffort,
		},
		CreatedAt: utc.New(time.Date(2025, 1, 22, 0, 0, 0, 0, time.UTC)),
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	moderated := true
	for _, provider := range []catalogs.Provider{
		{
			ID: "deepinfra", Name: "DeepInfra",
			Models: map[string]*catalogs.Model{
				"moonshotai/Kimi-K2.5": providerModel(
					"moonshotai/Kimi-K2.5",
					2,
					10,
					262_144,
					16_384,
				),
			},
		},
		{
			ID: "alibaba", Aliases: []catalogs.ProviderID{"qwen-cloud"}, Name: "Alibaba Cloud",
			GovernancePolicy: &catalogs.ProviderGovernancePolicy{Moderated: &moderated},
			Models: map[string]*catalogs.Model{
				"kimi-k2.5": providerModel(
					"kimi-k2.5",
					1,
					5,
					131_072,
					8_192,
				),
			},
		},
	} {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}

func providerModel(
	id string,
	inputPer1M float64,
	outputPer1M float64,
	context int64,
	output int64,
) *catalogs.Model {
	model := &catalogs.Model{
		ID: id, ModelRef: "moonshot-ai/kimi-k2.5", Name: "Kimi K2.5",
		Status: catalogs.ModelStatusActive,
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens: &catalogs.ModelTokenPricing{
				Input:  &catalogs.ModelTokenCost{Per1M: inputPer1M},
				Output: &catalogs.ModelTokenCost{Per1M: outputPer1M},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: context,
			OutputTokens:  output,
		},
	}
	if id == "kimi-k2.5" {
		model.Pricing.Tokens.CacheRead = &catalogs.ModelTokenCost{Per1M: 0.1}
		model.Modes = map[string]catalogs.ModelMode{
			"free": {
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyUSD,
					Tokens: &catalogs.ModelTokenPricing{
						Input:  &catalogs.ModelTokenCost{},
						Output: &catalogs.ModelTokenCost{},
					},
				},
			},
		}
	}
	return model
}
