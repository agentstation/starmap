package embedded

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/goccy/go-yaml"
)

func TestLoad(t *testing.T) {
	// Create a temporary directory with test data
	tmpDir, err := os.MkdirTemp("", "starmap-load-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create the directory structure directly in tmpDir (no "catalog" subdirectory for LoadFromPath)
	authorsDir := filepath.Join(tmpDir, "authors", "test-author")
	if err := os.MkdirAll(authorsDir, 0755); err != nil {
		t.Fatalf("Failed to create authors directory: %v", err)
	}

	// Create test providers.yaml
	providers := []catalogs.Provider{
		{ID: "test-provider-1", Name: "Test Provider 1"},
		{ID: "test-provider-2", Name: "Test Provider 2"},
	}
	providerData, _ := yaml.Marshal(providers)
	if err := os.WriteFile(filepath.Join(tmpDir, "providers.yaml"), providerData, 0644); err != nil {
		t.Fatalf("Failed to write providers.yaml: %v", err)
	}

	// Create test authors.yaml
	authors := []catalogs.Author{
		{ID: "test-author-1", Name: "Test Author 1"},
		{ID: "test-author-2", Name: "Test Author 2"},
	}
	authorData, _ := yaml.Marshal(authors)
	if err := os.WriteFile(filepath.Join(tmpDir, "authors.yaml"), authorData, 0644); err != nil {
		t.Fatalf("Failed to write authors.yaml: %v", err)
	}

	// Create test model file
	testModel := `id: test-model
name: Test Model
authors:
  - test-author-1
`
	if err := os.WriteFile(filepath.Join(authorsDir, "test-model.yaml"), []byte(testModel), 0644); err != nil {
		t.Fatalf("Failed to write test model: %v", err)
	}

	// Create catalog and load from the temp directory using LoadFromPath
	catalogImpl := NewCatalog().(*catalog)
	if err := catalogImpl.LoadFromPath(tmpDir); err != nil {
		t.Fatalf("LoadFromPath failed: %v", err)
	}
	catalog := catalogs.Catalog(catalogImpl)

	// Verify providers were loaded
	if catalog.Providers().Len() != 2 {
		t.Errorf("Expected 2 providers, got %d", catalog.Providers().Len())
	}

	// Verify authors were loaded
	if catalog.Authors().Len() != 2 {
		t.Errorf("Expected 2 authors, got %d", catalog.Authors().Len())
	}

	// Verify models were loaded
	if catalog.Models().Len() != 1 {
		t.Errorf("Expected 1 model, got %d", catalog.Models().Len())
	}

	// Check specific model
	model, ok := catalog.Models().Get("test-model")
	if !ok {
		t.Error("Model 'test-model' not found")
	} else if model.Name != "Test Model" {
		t.Errorf("Expected model name 'Test Model', got '%s'", model.Name)
	}
}

func TestLoadFromPath(t *testing.T) {
	// Create a temporary directory with test data
	tmpDir, err := os.MkdirTemp("", "starmap-loadfrompath-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test providers.yaml
	providers := []catalogs.Provider{
		{ID: "provider-1", Name: "Provider 1"},
		{ID: "provider-2", Name: "Provider 2"},
	}
	data, _ := yaml.Marshal(providers)
	if err := os.WriteFile(filepath.Join(tmpDir, "providers.yaml"), data, 0644); err != nil {
		t.Fatalf("Failed to write providers.yaml: %v", err)
	}

	c := NewCatalog().(*catalog)
	if err := c.LoadFromPath(tmpDir); err != nil {
		t.Fatalf("LoadFromPath failed: %v", err)
	}
	cat := catalogs.Catalog(c)

	if cat.Providers().Len() != 2 {
		t.Errorf("Expected 2 providers, got %d", cat.Providers().Len())
	}

	// Check specific providers
	provider1, ok := cat.Providers().Get("provider-1")
	if !ok {
		t.Error("Provider 'provider-1' not found")
	} else if provider1.Name != "Provider 1" {
		t.Errorf("Expected provider name 'Provider 1', got '%s'", provider1.Name)
	}

	provider2, ok := cat.Providers().Get("provider-2")
	if !ok {
		t.Error("Provider 'provider-2' not found")
	} else if provider2.Name != "Provider 2" {
		t.Errorf("Expected provider name 'Provider 2', got '%s'", provider2.Name)
	}
}
