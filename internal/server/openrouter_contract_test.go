package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/openrouter"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestOpenRouterRoutesProjectCanonicalModelAndProviderEndpoints(t *testing.T) {
	t.Parallel()

	handler := openRouterTestHandler(t, false)
	modelResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		modelResponse,
		httptest.NewRequest(http.MethodGet, "/api/v1/model/laboratory/model", nil),
	)
	if modelResponse.Code != http.StatusOK {
		t.Fatalf("model status = %d: %s", modelResponse.Code, modelResponse.Body.String())
	}
	var modelEnvelope struct {
		Data struct {
			ID            string `json:"id"`
			CanonicalSlug string `json:"canonical_slug"`
		} `json:"data"`
	}
	if err := json.Unmarshal(modelResponse.Body.Bytes(), &modelEnvelope); err != nil {
		t.Fatalf("decode model response: %v", err)
	}
	if modelEnvelope.Data.ID != "lab/model" ||
		modelEnvelope.Data.CanonicalSlug != "lab/model" {
		t.Fatalf("model response = %#v", modelEnvelope.Data)
	}
	if modelResponse.Header().Get("X-Starmap-Generation-ID") != "test-generation" {
		t.Fatalf(
			"generation header = %q",
			modelResponse.Header().Get("X-Starmap-Generation-ID"),
		)
	}
	assertJSONFixture(t, modelResponse.Body.Bytes(), "testdata/openrouter/model.json")

	endpointResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		endpointResponse,
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/models/laboratory/model/endpoints",
			nil,
		),
	)
	if endpointResponse.Code != http.StatusOK {
		t.Fatalf(
			"endpoint status = %d: %s",
			endpointResponse.Code,
			endpointResponse.Body.String(),
		)
	}
	var endpointsEnvelope struct {
		Data struct {
			Endpoints []struct {
				ModelID      string `json:"model_id"`
				ProviderName string `json:"provider_name"`
				Tag          string `json:"tag"`
			} `json:"endpoints"`
		} `json:"data"`
	}
	if err := json.Unmarshal(endpointResponse.Body.Bytes(), &endpointsEnvelope); err != nil {
		t.Fatalf("decode endpoint response: %v", err)
	}
	if len(endpointsEnvelope.Data.Endpoints) != 1 ||
		endpointsEnvelope.Data.Endpoints[0].ModelID != "provider/model-v1" ||
		endpointsEnvelope.Data.Endpoints[0].ProviderName != "Provider" ||
		endpointsEnvelope.Data.Endpoints[0].Tag != "provider" {
		t.Fatalf("endpoint response = %#v", endpointsEnvelope.Data.Endpoints)
	}
	assertJSONFixture(
		t,
		endpointResponse.Body.Bytes(),
		"testdata/openrouter/endpoints.json",
	)

	genericResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		genericResponse,
		httptest.NewRequest(http.MethodGet, "/api/v1/models/lab/model", nil),
	)
	if genericResponse.Code != http.StatusOK {
		t.Fatalf(
			"existing generic model route status = %d: %s",
			genericResponse.Code,
			genericResponse.Body.String(),
		)
	}
}

func assertJSONFixture(t testing.TB, actual []byte, path string) {
	t.Helper()
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var actualValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		t.Fatalf("decode actual JSON: %v", err)
	}
	var expectedValue any
	if err := json.Unmarshal(expected, &expectedValue); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		t.Fatalf(
			"response does not match %s\nactual:\n%s\nexpected:\n%s",
			path,
			actual,
			expected,
		)
	}
}

func TestOpenRouterRoutesUseNumericNotFoundEnvelope(t *testing.T) {
	t.Parallel()

	handler := openRouterTestHandler(t, false)
	for _, path := range []string{
		"/api/v1/model/lab/missing",
		"/api/v1/models/lab/missing/endpoints",
		"/api/v1/model/lab",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder,
			httptest.NewRequest(http.MethodGet, path, nil),
		)
		if recorder.Code != http.StatusNotFound ||
			recorder.Body.String() !=
				`{"error":{"code":404,"message":"Resource not found"}}`+"\n" {
			t.Fatalf(
				"%s response = %d %s",
				path,
				recorder.Code,
				recorder.Body.String(),
			)
		}
	}
}

func TestOpenRouterRouteMapsAmbiguousAliasToDeterministicNotFound(t *testing.T) {
	t.Parallel()

	handler := openRouterHandlerForCatalog(t, ambiguousOpenRouterCatalog(t), false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/model/lab/ambiguous",
			nil,
		),
	)
	if recorder.Code != http.StatusNotFound ||
		recorder.Body.String() !=
			`{"error":{"code":404,"message":"Resource not found"}}`+"\n" {
		t.Fatalf(
			"ambiguous response = %d %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestOpenRouterAuthenticationUsesCompatibilityEnvelopeOnlyOnAdapterRoutes(t *testing.T) {
	t.Setenv("API_KEY", "test-key")
	handler := openRouterTestHandler(t, true)

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(
		unauthorized,
		httptest.NewRequest(http.MethodGet, "/api/v1/model/lab/model", nil),
	)
	if unauthorized.Code != http.StatusUnauthorized ||
		unauthorized.Body.String() !=
			`{"error":{"code":401,"message":"No auth credentials found"}}`+"\n" {
		t.Fatalf(
			"OpenRouter auth response = %d %s",
			unauthorized.Code,
			unauthorized.Body.String(),
		)
	}

	encoded := httptest.NewRecorder()
	handler.ServeHTTP(
		encoded,
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/model/lab%2Fother/model",
			nil,
		),
	)
	if encoded.Code != http.StatusUnauthorized ||
		encoded.Body.String() !=
			`{"error":{"code":401,"message":"No auth credentials found"}}`+"\n" {
		t.Fatalf(
			"encoded OpenRouter auth response = %d %s",
			encoded.Code,
			encoded.Body.String(),
		)
	}

	generic := httptest.NewRecorder()
	handler.ServeHTTP(
		generic,
		httptest.NewRequest(http.MethodGet, "/api/v1/models", nil),
	)
	if generic.Code != http.StatusUnauthorized ||
		!strings.Contains(generic.Body.String(), `"code":"UNAUTHORIZED"`) {
		t.Fatalf("generic auth response = %d %s", generic.Code, generic.Body.String())
	}

	authorizedRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/model/lab/model",
		nil,
	)
	authorizedRequest.Header.Set("Authorization", "Bearer test-key")
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized response = %d %s", authorized.Code, authorized.Body.String())
	}
}

func TestOpenRouterModelDetailsUsesConfiguredPathPrefix(t *testing.T) {
	t.Parallel()

	const prefix = "/catalog/v2"
	handler := openRouterHandlerForCatalogAtPrefix(
		t,
		openRouterTestCatalog(t),
		false,
		prefix,
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			prefix+"/model/lab/model",
			nil,
		),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("model response = %d %s", recorder.Code, recorder.Body.String())
	}
	var response openrouter.ModelEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	const want = prefix + "/models/lab/model/endpoints"
	if response.Data.Links.Details != want {
		t.Fatalf("details link = %q, want %q", response.Data.Links.Details, want)
	}
}

func TestNativeModelPathRetainsNativeEnvelopeOutsideCompatibilitySuffix(t *testing.T) {
	t.Parallel()

	handler := openRouterTestHandler(t, false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/models/lab%2Fother/model/detail",
			nil,
		),
	)
	if recorder.Code != http.StatusNotFound ||
		!strings.Contains(recorder.Body.String(), `"data":null`) ||
		!strings.Contains(recorder.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf(
			"native model response = %d %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func openRouterTestHandler(t testing.TB, authEnabled bool) http.Handler {
	t.Helper()
	return openRouterHandlerForCatalog(t, openRouterTestCatalog(t), authEnabled)
}

func openRouterHandlerForCatalog(
	t testing.TB,
	catalog *catalogs.Catalog,
	authEnabled bool,
) http.Handler {
	t.Helper()
	return openRouterHandlerForCatalogAtPrefix(
		t,
		catalog,
		authEnabled,
		DefaultConfig().PathPrefix,
	)
}

func openRouterHandlerForCatalogAtPrefix(
	t testing.TB,
	catalog *catalogs.Catalog,
	authEnabled bool,
	pathPrefix string,
) http.Handler {
	t.Helper()
	app := newMockApplication()
	app.catalog = catalog
	app.catalogState = &starmap.CatalogState{
		Catalog:      catalog,
		GenerationID: "test-generation",
		Sequence:     7,
	}
	config := DefaultConfig()
	config.PathPrefix = pathPrefix
	config.AuthEnabled = authEnabled
	config.RateLimit = 0
	config.CacheTTL = time.Minute
	server, err := New(app, config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return server.setupRouter()
}

func ambiguousOpenRouterCatalog(t testing.TB) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "lab", Name: "Lab"}
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
	return catalog
}

func openRouterTestCatalog(t testing.TB) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	author := catalogs.Author{
		ID: "lab", Aliases: []catalogs.AuthorID{"laboratory"}, Name: "Lab",
	}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel(author.ID, catalogs.Model{
		ID: "model", Name: "Model", Authors: []catalogs.Author{author},
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
			Temperature: true,
		},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*catalogs.Model{
			"provider/model-v1": {
				ID: "provider/model-v1", ModelRef: "lab/model", Name: "Model",
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyUSD,
					Tokens: &catalogs.ModelTokenPricing{
						Input: &catalogs.ModelTokenCost{Per1M: 1},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}
