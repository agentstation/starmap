package providers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/testcatalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
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

func (app testApplication) CredentialResolver() (sources.ProviderCredentialResolver, error) {
	return auth.NewResolver(), nil
}

func TestYAMLOnlyProviderCredentialJSONKeepsProgressOnStderr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"test-model","object":"model","owned_by":"test"}]}`))
	}))
	defer server.Close()

	const apiKeyEnvironment = "STARMAP_PROVIDER_OUTPUT_TEST_KEY"
	t.Setenv(apiKeyEnvironment, "test-key")

	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID: "yaml-only", Name: "YAML-only provider",
		Credentials: testcatalog.APIKeyCredentials(
			apiKeyEnvironment, "Authorization", catalogs.ProviderCredentialSchemeBearer,
		),
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:            catalogs.EndpointTypeOpenAI,
				URL:             server.URL,
				ProtocolOptions: testcatalog.OpenAIProtocolOptions(),
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
	command.SetArgs([]string{"yaml-only", "--test"})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr:\n%s", err, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not valid JSON:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Testing") || strings.Contains(stdout.String(), "successful") {
		t.Fatalf("stdout contains progress text:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Testing yaml-only credentials") ||
		!strings.Contains(stderr.String(), "Test successful") {
		t.Fatalf("stderr lacks credential-test progress:\n%s", stderr.String())
	}
}
