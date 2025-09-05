// Package testhelper provides utilities for managing testdata files in provider tests.
package testhelper

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
)

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

	t.Logf("Updated testdata file: %s", testdataPath)
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
