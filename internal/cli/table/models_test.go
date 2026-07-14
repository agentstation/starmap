package table

import (
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestProviderTableDoesNotExposeCredentialFingerprint(t *testing.T) {
	const (
		envName = "STARMAP_TABLE_TEST_API_KEY"
		secret  = "credential-production-secret-fingerprint-1234"
	)
	t.Setenv(envName, secret)

	data := ProvidersToTableData([]*catalogs.Provider{{
		ID: "test", Name: "Test", Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{envName}}},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth:     catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models"},
		}}},
	}}, auth.NewChecker(), map[string]bool{"test": true})
	if len(data.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(data.Rows))
	}
	row := strings.Join(data.Rows[0], " ")
	if !strings.Contains(row, envName) || !strings.Contains(row, "Ready") {
		t.Fatalf("configured marker missing from row %q", row)
	}
	for _, fragment := range []string{secret, secret[:8], secret[len(secret)-4:]} {
		if strings.Contains(row, fragment) {
			t.Fatalf("provider row exposed credential fragment %q: %q", fragment, row)
		}
	}
}
