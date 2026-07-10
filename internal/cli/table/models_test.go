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
		secret  = "sk-production-secret-fingerprint-1234"
	)
	t.Setenv(envName, secret)

	data := ProvidersToTableData([]*catalogs.Provider{{
		ID: "test", Name: "Test", APIKey: &catalogs.ProviderAPIKey{Name: envName},
	}}, auth.NewChecker(), map[string]bool{"test": true})
	if len(data.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(data.Rows))
	}
	row := strings.Join(data.Rows[0], " ")
	if !strings.Contains(row, "(configured)") {
		t.Fatalf("configured marker missing from row %q", row)
	}
	for _, fragment := range []string{secret, secret[:8], secret[len(secret)-4:]} {
		if strings.Contains(row, fragment) {
			t.Fatalf("provider row exposed credential fragment %q: %q", fragment, row)
		}
	}
}
