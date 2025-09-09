package inspect

import (
	"io/fs"
	"strings"
	"testing"

	inspectutil "github.com/agentstation/starmap/internal/cmd/inspect"
)

func TestEmbeddedFilesystemAccess(t *testing.T) {
	fsys := inspectutil.GetEmbeddedFS()
	
	// Test that we can access the embedded filesystem
	if fsys == nil {
		t.Fatal("GetEmbeddedFS() returned nil")
	}
	
	// Test root directory listing
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatalf("Failed to read root directory: %v", err)
	}
	
	// Should have at least catalog and sources
	foundCatalog := false
	foundSources := false
	
	for _, entry := range entries {
		switch entry.Name() {
		case "catalog":
			foundCatalog = true
			if !entry.IsDir() {
				t.Error("catalog should be a directory")
			}
		case "sources":
			foundSources = true
			if !entry.IsDir() {
				t.Error("sources should be a directory")
			}
		}
	}
	
	if !foundCatalog {
		t.Error("catalog directory not found in embedded filesystem")
	}
	if !foundSources {
		t.Error("sources directory not found in embedded filesystem")
	}
}

func TestEmbeddedCatalogAccess(t *testing.T) {
	fsys := inspectutil.GetEmbeddedFS()
	
	// Test catalog directory
	entries, err := fs.ReadDir(fsys, "catalog")
	if err != nil {
		t.Fatalf("Failed to read catalog directory: %v", err)
	}
	
	foundProviders := false
	foundAuthors := false
	
	for _, entry := range entries {
		switch entry.Name() {
		case "providers":
			foundProviders = true
		case "authors":
			foundAuthors = true
		}
	}
	
	if !foundProviders {
		t.Error("providers directory not found in catalog")
	}
	if !foundAuthors {
		t.Error("authors directory not found in catalog")
	}
}

func TestEmbeddedModelsDevAccess(t *testing.T) {
	fsys := inspectutil.GetEmbeddedFS()
	
	// Test that models.dev api.json exists and is readable
	data, err := fs.ReadFile(fsys, "sources/models.dev/api.json")
	if err != nil {
		t.Fatalf("Failed to read sources/models.dev/api.json: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("sources/models.dev/api.json is empty")
	}
	
	// Should be JSON content
	content := string(data)
	if !strings.HasPrefix(content, "{") {
		t.Error("sources/models.dev/api.json doesn't appear to be JSON")
	}
	
	// Should be reasonably sized (> 100KB as per our validation)
	if len(data) < 100000 {
		t.Errorf("sources/models.dev/api.json is too small (%d bytes), expected > 100KB", len(data))
	}
}

func TestPathNormalizationIntegration(t *testing.T) {
	tests := []struct {
		path        string
		shouldExist bool
	}{
		{"catalog", true},
		{"/catalog", true},
		{"catalog/", true},
		{"/catalog/", true},
		{"sources/models.dev", true},
		{"/sources/models.dev/", true},
		{"nonexistent", false},
	}
	
	fsys := inspectutil.GetEmbeddedFS()
	
	for _, test := range tests {
		normalizedPath := inspectutil.NormalizePath(test.path)
		_, err := fs.Stat(fsys, normalizedPath)
		
		if test.shouldExist && err != nil {
			t.Errorf("Path %q (normalized to %q) should exist but got error: %v", test.path, normalizedPath, err)
		}
		if !test.shouldExist && err == nil {
			t.Errorf("Path %q (normalized to %q) should not exist but no error", test.path, normalizedPath)
		}
	}
}