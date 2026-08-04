package providers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type testApplication struct {
	catalog *catalogs.Catalog
	logger  zerolog.Logger
	output  string
}

func (app testApplication) Catalog() (*catalogs.Catalog, error) {
	return app.catalog, nil
}

func (app testApplication) Logger() *zerolog.Logger {
	return &app.logger
}

func (app testApplication) OutputFormat() string {
	return app.output
}

func TestProviderCredentialJSONKeepsProgressOnStderr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"test-model","object":"model","owned_by":"test"}]}`))
	}))
	defer server.Close()

	const apiKeyEnvironment = "STARMAP_PROVIDER_OUTPUT_TEST_KEY"
	t.Setenv(apiKeyEnvironment, "test-key")

	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID:   "openai",
		Name: "OpenAI",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   apiKeyEnvironment,
			Header: "Authorization",
			Scheme: catalogs.ProviderAPIKeySchemeBearer,
		},
		Catalog: &catalogs.ProviderCatalog{
			Auth: catalogs.ProviderCatalogAuth{Method: catalogs.ProviderCatalogAuthAPIKey, Required: true},
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				URL:  server.URL,
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := NewCommand(testApplication{
		catalog: catalog,
		logger:  zerolog.Nop(),
		output:  "json",
	})
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.Flags().Bool("verbose", false, "test root verbose flag")
	command.SetArgs([]string{"openai", "--test"})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr:\n%s", err, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not valid JSON:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Testing") || strings.Contains(stdout.String(), "successful") {
		t.Fatalf("stdout contains progress text:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Testing openai credentials") ||
		!strings.Contains(stderr.String(), "Test successful") {
		t.Fatalf("stderr lacks credential-test progress:\n%s", stderr.String())
	}
}
