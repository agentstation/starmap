// Package testhelper provides read-only utilities for provider behavior tests.
package testhelper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/providerfixture"
)

// FixtureMetadata is the production fixture metadata contract.
type FixtureMetadata = providerfixture.Metadata

// FixturePayload is the production fixture payload identity contract.
type FixturePayload = providerfixture.Payload

// LoadTestdata loads a testdata file from the caller's testdata directory.
func LoadTestdata(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path) //nolint:gosec // Test file paths are controlled.
	if err != nil {
		t.Fatalf("Failed to load testdata file %s: %v", path, err)
	}
	return data
}

// LoadJSON loads and unmarshals JSON from a testdata file.
func LoadJSON(t *testing.T, filename string, value any) {
	t.Helper()
	if err := json.Unmarshal(LoadTestdata(t, filename), value); err != nil {
		t.Fatalf("Failed to unmarshal JSON from testdata file %s: %v", filename, err)
	}
}

// VerifyFixture validates payload identity, source revision, and freshness.
func VerifyFixture(path string, now time.Time) error {
	return providerfixture.Verify(path, now)
}
