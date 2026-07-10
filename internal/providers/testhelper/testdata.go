// Package testhelper provides utilities for managing testdata files in provider tests.
package testhelper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/types"
)

const defaultFixtureMaxAge = 365 * 24 * time.Hour

// FixtureMetadata binds a provider fixture to its source revision and freshness policy.
type FixtureMetadata struct {
	Version        uint64                    `json:"version"`
	Provider       string                    `json:"provider"`
	FetchedAt      time.Time                 `json:"fetched_at"`
	SourceRevision types.ObservationRevision `json:"source_revision"`
	Payload        FixturePayload            `json:"payload"`
	MaxAge         string                    `json:"max_age"`
}

// FixturePayload identifies the exact fixture bytes governed by metadata.
type FixturePayload struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
}

// UpdateTestdata is the global flag for updating testdata files.
var UpdateTestdata = flag.Bool("update", false, "update testdata files")

// LoadTestdata loads a testdata file from the caller's testdata directory.
func LoadTestdata(t *testing.T, filename string) []byte {
	t.Helper()

	// Get the testdata path relative to the test file
	testdataPath := filepath.Join("testdata", filename)

	data, err := os.ReadFile(testdataPath) //nolint:gosec // Test file paths are controlled
	if err != nil {
		t.Fatalf("Failed to load testdata file %s: %v", testdataPath, err)
	}

	return data
}

// SaveTestdata saves data to a testdata file if the -update flag is set.
func SaveTestdata(t *testing.T, filename string, data []byte) {
	t.Helper()

	if !*UpdateTestdata {
		return
	}

	// Ensure testdata directory exists
	testdataDir := "testdata"
	if err := os.MkdirAll(testdataDir, constants.DirPermissions); err != nil {
		t.Fatalf("Failed to create testdata directory: %v", err)
	}

	testdataPath := filepath.Join(testdataDir, filename)

	if err := os.WriteFile(testdataPath, data, constants.FilePermissions); err != nil {
		t.Fatalf("Failed to save testdata file %s: %v", testdataPath, err)
	}
	provider, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("Failed to resolve provider directory: %v", err)
	}
	checksum := fixtureChecksum(data)
	metadata := FixtureMetadata{
		Version: 1, Provider: filepath.Base(provider), FetchedAt: time.Now().UTC(),
		SourceRevision: types.ObservationRevision{Kind: types.ObservationRevisionKindContentDigest, Value: checksum},
		Payload:        FixturePayload{Path: filename, Checksum: checksum}, MaxAge: defaultFixtureMaxAge.String(),
	}
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal fixture metadata: %v", err)
	}
	metadataData = append(metadataData, '\n')
	metadataPath := fixtureMetadataPath(testdataPath)
	if err := os.WriteFile(metadataPath, metadataData, constants.FilePermissions); err != nil {
		t.Fatalf("Failed to save fixture metadata %s: %v", metadataPath, err)
	}

	t.Logf("Updated testdata file: %s", testdataPath)
}

// VerifyFixture validates payload identity, source revision, and freshness.
func VerifyFixture(testdataPath string, now time.Time) error {
	payload, err := os.ReadFile(testdataPath) //nolint:gosec // Test fixture path is caller-controlled test input.
	if err != nil {
		return errors.WrapIO("read", testdataPath, err)
	}
	metadataPath := fixtureMetadataPath(testdataPath)
	metadataData, err := os.ReadFile(metadataPath) //nolint:gosec // Metadata is adjacent to the controlled fixture path.
	if err != nil {
		return errors.WrapIO("read", metadataPath, err)
	}
	var metadata FixtureMetadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		return &errors.ValidationError{Field: "fixture.metadata", Value: metadataPath, Message: err.Error()}
	}
	if metadata.Version != 1 || strings.TrimSpace(metadata.Provider) == "" {
		return &errors.ValidationError{Field: "fixture.metadata", Value: metadata.Version, Message: "version 1 and provider are required"}
	}
	expectedProvider := filepath.Base(filepath.Dir(filepath.Dir(testdataPath)))
	if metadata.Provider != expectedProvider {
		return &errors.ValidationError{Field: "fixture.provider", Value: metadata.Provider, Message: "does not match provider directory"}
	}
	if metadata.Payload.Path != filepath.Base(testdataPath) {
		return &errors.ValidationError{Field: "fixture.payload.path", Value: metadata.Payload.Path, Message: "must name the adjacent fixture"}
	}
	checksum := fixtureChecksum(payload)
	if metadata.Payload.Checksum != checksum {
		return &errors.ValidationError{Field: "fixture.payload.checksum", Value: metadata.Payload.Checksum, Message: "does not match fixture bytes"}
	}
	if metadata.SourceRevision.Kind != types.ObservationRevisionKindContentDigest || metadata.SourceRevision.Value != checksum {
		return &errors.ValidationError{Field: "fixture.source_revision", Value: metadata.SourceRevision, Message: "must identify the fixture content digest"}
	}
	if metadata.FetchedAt.IsZero() || metadata.FetchedAt.After(now.Add(5*time.Minute)) {
		return &errors.ValidationError{Field: "fixture.fetched_at", Value: metadata.FetchedAt, Message: "must be a non-future capture time"}
	}
	maxAge, err := time.ParseDuration(metadata.MaxAge)
	if err != nil || maxAge <= 0 {
		return &errors.ValidationError{Field: "fixture.max_age", Value: metadata.MaxAge, Message: "must be a positive Go duration"}
	}
	if now.Sub(metadata.FetchedAt) > maxAge {
		return &errors.ValidationError{Field: "fixture.fetched_at", Value: metadata.FetchedAt, Message: "provider fixture is stale"}
	}
	return nil
}

func fixtureMetadataPath(testdataPath string) string {
	return strings.TrimSuffix(testdataPath, filepath.Ext(testdataPath)) + ".metadata.json"
}

func fixtureChecksum(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// SaveJSON saves JSON data to a testdata file with proper formatting.
func SaveJSON(t *testing.T, filename string, v any) {
	t.Helper()

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON for testdata file %s: %v", filename, err)
	}

	SaveTestdata(t, filename, data)
}

// LoadJSON loads and unmarshals JSON from a testdata file.
func LoadJSON(t *testing.T, filename string, v any) {
	t.Helper()

	data := LoadTestdata(t, filename)

	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("Failed to unmarshal JSON from testdata file %s: %v", filename, err)
	}
}

// CompareWithTestdata compares actual data with expected testdata file
// If -update flag is set, it updates the testdata file with actual data.
func CompareWithTestdata(t *testing.T, filename string, actual []byte) {
	t.Helper()

	if *UpdateTestdata {
		SaveTestdata(t, filename, actual)
		return
	}

	expected := LoadTestdata(t, filename)

	if string(actual) != string(expected) {
		t.Errorf("Data does not match testdata file %s\nActual:\n%s\nExpected:\n%s",
			filename, string(actual), string(expected))
	}
}

// CompareJSONWithTestdata compares actual JSON data with expected testdata file.
func CompareJSONWithTestdata(t *testing.T, filename string, actual any) {
	t.Helper()

	actualData, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal actual data for comparison: %v", err)
	}

	if *UpdateTestdata {
		SaveTestdata(t, filename, actualData)
		return
	}

	expected := LoadTestdata(t, filename)

	if string(actualData) != string(expected) {
		t.Errorf("JSON data does not match testdata file %s\nActual:\n%s\nExpected:\n%s",
			filename, string(actualData), string(expected))
	}
}

// TestdataExists checks if a testdata file exists.
func TestdataExists(filename string) bool {
	testdataPath := filepath.Join("testdata", filename)
	_, err := os.Stat(testdataPath)
	return err == nil
}
