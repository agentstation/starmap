package catalogs

import (
	"testing"
	"time"
)

// TestProvider creates a test provider with sensible defaults.
// The t.Helper() call ensures stack traces point to the test, not this function.
func TestProvider(t testing.TB) *Provider {
	t.Helper()
	hq := "Test City, Test Country"
	apiKeyRequired := true
	apiURL := "https://api.test-provider.com/v1/models"
	return &Provider{
		ID:           "test-provider",
		Name:         "Test Provider",
		Headquarters: &hq,
		APIKey: &ProviderAPIKey{
			Name:    "TEST_API_KEY",
			Pattern: "test-.*",
			Header:  "Authorization",
			Scheme:  "Bearer",
		},
		Catalog: &ProviderCatalog{
			Endpoint: ProviderEndpoint{
				AuthRequired: apiKeyRequired,
				URL:          apiURL,
			},
			Authors: []AuthorID{
				"test-author",
			},
		},
	}
}

// TestModel creates a test model with sensible defaults.
func TestModel(t testing.TB) *Model {
	t.Helper()
	return &Model{
		ID:          "test-model",
		Name:        "Test Model",
		Description: "A test model for unit tests",
	}
}

// TestAuthor creates a test author with sensible defaults.
func TestAuthor(t testing.TB) *Author {
	t.Helper()
	return &Author{
		ID:   "test-author",
		Name: "Test Author",
	}
}

// TestEndpoint creates a test endpoint with sensible defaults.
func TestEndpoint(t testing.TB) *Endpoint {
	t.Helper()
	return &Endpoint{
		ID: "test-endpoint",
	}
}

// TestCatalog creates a test catalog with sample data.
func TestCatalog(t testing.TB) Catalog {
	t.Helper()

	catalog := Empty()

	// Add test provider with a model
	provider := TestProvider(t)
	model := TestModel(t)
	provider.Models = map[string]*Model{
		model.ID: model,
	}
	if err := catalog.SetProvider(*provider); err != nil {
		t.Fatalf("failed to add test provider: %v", err)
	}

	// Add test author
	author := TestAuthor(t)
	if err := catalog.SetAuthor(*author); err != nil {
		t.Fatalf("failed to add test author: %v", err)
	}

	return catalog
}

// TestModelOption is a functional option for configuring a test model.
type TestModelOption func(*Model)

// WithModelID sets a custom ID for the test model.
func WithModelID(id string) TestModelOption {
	return func(m *Model) {
		m.ID = id
	}
}

// WithModelName sets a custom name for the test model.
func WithModelName(name string) TestModelOption {
	return func(m *Model) {
		m.Name = name
	}
}

// TestModelWithOptions creates a test model with custom options.
func TestModelWithOptions(t testing.TB, opts ...TestModelOption) *Model {
	t.Helper()

	model := TestModel(t)
	for _, opt := range opts {
		opt(model)
	}

	return model
}

// TestProviderOption is a functional option for configuring a test provider.
type TestProviderOption func(*Provider)

// WithProviderID sets a custom ID for the test provider.
func WithProviderID(id ProviderID) TestProviderOption {
	return func(p *Provider) {
		p.ID = id
	}
}

// WithProviderAPIKey sets a custom API key configuration.
func WithProviderAPIKey(name, pattern string) TestProviderOption {
	return func(p *Provider) {
		p.APIKey = &ProviderAPIKey{
			Name:    name,
			Pattern: pattern,
			Header:  "Authorization",
			Scheme:  "Bearer",
		}
	}
}

// WithProviderEnvVars sets environment variables for the test provider.
func WithProviderEnvVars(envVars []ProviderEnvVar) TestProviderOption {
	return func(p *Provider) {
		p.EnvVars = envVars
	}
}

// TestProviderWithOptions creates a test provider with custom options.
func TestProviderWithOptions(t testing.TB, opts ...TestProviderOption) *Provider {
	t.Helper()

	provider := TestProvider(t)
	for _, opt := range opts {
		opt(provider)
	}

	return provider
}

// AssertModelsEqual asserts that two models are equal, providing detailed diff on failure.
func AssertModelsEqual(t testing.TB, expected, actual *Model) {
	t.Helper()

	if expected.ID != actual.ID {
		t.Errorf("Model ID mismatch: expected %q, got %q", expected.ID, actual.ID)
	}

	if expected.Name != actual.Name {
		t.Errorf("Model Name mismatch: expected %q, got %q", expected.Name, actual.Name)
	}

	if expected.Description != actual.Description {
		t.Errorf("Model Description mismatch: expected %q, got %q", expected.Description, actual.Description)
	}
}

// AssertProvidersEqual asserts that two providers are equal.
func AssertProvidersEqual(t testing.TB, expected, actual *Provider) {
	t.Helper()

	if expected.ID != actual.ID {
		t.Errorf("Provider ID mismatch: expected %q, got %q", expected.ID, actual.ID)
	}

	if expected.Name != actual.Name {
		t.Errorf("Provider Name mismatch: expected %q, got %q", expected.Name, actual.Name)
	}

	if expected.APIKey.Name != actual.APIKey.Name {
		t.Errorf("Provider APIKey Name mismatch: expected %q, got %q", expected.APIKey.Name, actual.APIKey.Name)
	}
}

// AssertCatalogHasModel asserts that a catalog contains a model with the given ID.
func AssertCatalogHasModel(t testing.TB, catalog Catalog, modelID string) {
	t.Helper()

	_, err := catalog.FindModel(modelID)
	if err != nil {
		t.Errorf("Expected catalog to have model %q, but got error: %v", modelID, err)
	}
}

// AssertCatalogHasProvider asserts that a catalog contains a provider with the given ID.
func AssertCatalogHasProvider(t testing.TB, catalog Catalog, providerID ProviderID) {
	t.Helper()

	_, err := catalog.Provider(providerID)
	if err != nil {
		t.Errorf("Expected catalog to have provider %q, but got error: %v", providerID, err)
	}
}

// TestTimeNow returns a consistent time for testing.
func TestTimeNow() time.Time {
	return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
}

// TestAPIResponse creates a test API response for provider testing.
func TestAPIResponse(models ...string) map[string]any {
	modelList := make([]map[string]any, len(models))
	for i, modelID := range models {
		modelList[i] = map[string]any{
			"id":       modelID,
			"object":   "model",
			"created":  TestTimeNow().Unix(),
			"owned_by": "test-owner",
		}
	}

	return map[string]any{
		"object": "list",
		"data":   modelList,
	}
}
