// Package fixtures provides read-only loaders for provider behavior-test fixtures.
package fixtures

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Load reads a file from the calling provider package's testdata directory.
func Load(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path) //nolint:gosec // Test file paths are controlled.
	if err != nil {
		t.Fatalf("Failed to load testdata file %s: %v", path, err)
	}
	return data
}

// LoadJSON reads and unmarshals JSON from the calling provider package's testdata directory.
func LoadJSON(t *testing.T, filename string, value any) {
	t.Helper()
	if err := json.Unmarshal(Load(t, filename), value); err != nil {
		t.Fatalf("Failed to unmarshal JSON from testdata file %s: %v", filename, err)
	}
}

// EmbeddedProvider loads one caller-owned copy of the real embedded provider configuration.
func EmbeddedProvider(t *testing.T, providerID catalogs.ProviderID) catalogs.Provider {
	t.Helper()
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	provider, err := builder.Provider(providerID)
	if err != nil {
		t.Fatalf("Provider(%s): %v", providerID, err)
	}
	return provider
}
