package embedded

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/utc"
	"github.com/goccy/go-yaml"
)

func TestSave(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "starmap-save-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to the temp directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create a new catalog with test data
	cat := NewCatalog()

	// Add test providers
	testProvider1 := catalogs.Provider{
		ID:   "test-provider-1",
		Name: "Test Provider 1",
	}
	testProvider2 := catalogs.Provider{
		ID:   "test-provider-2",
		Name: "Test Provider 2",
	}
	if err := cat.AddProvider(testProvider1); err != nil {
		t.Fatalf("Failed to add test provider 1: %v", err)
	}
	if err := cat.AddProvider(testProvider2); err != nil {
		t.Fatalf("Failed to add test provider 2: %v", err)
	}

	// Add test authors
	testAuthor1 := catalogs.Author{
		ID:   "test-author-1",
		Name: "Test Author 1",
		Catalog: &catalogs.AuthorCatalog{
			ProviderID: "test-provider-1",
		},
		CreatedAt: utc.Time{},
		UpdatedAt: utc.Time{},
	}
	testAuthor2 := catalogs.Author{
		ID:        "test-author-2",
		Name:      "Test Author 2",
		CreatedAt: utc.Time{},
		UpdatedAt: utc.Time{},
	}
	if err := cat.AddAuthor(testAuthor1); err != nil {
		t.Fatalf("Failed to add test author 1: %v", err)
	}
	if err := cat.AddAuthor(testAuthor2); err != nil {
		t.Fatalf("Failed to add test author 2: %v", err)
	}

	// Add test models
	testModel1 := catalogs.Model{
		ID:   "test-model-1",
		Name: "Test Model 1",
		Authors: []catalogs.Author{
			{ID: "test-author-1", Name: "Test Author 1"},
		},
		CreatedAt: utc.Time{},
		UpdatedAt: utc.Time{},
	}
	testModel2 := catalogs.Model{
		ID:   "test-model-2",
		Name: "Test Model 2",
		Authors: []catalogs.Author{
			{ID: "test-author-2", Name: "Test Author 2"},
		},
		CreatedAt: utc.Time{},
		UpdatedAt: utc.Time{},
	}
	if err := cat.AddModel(testModel1); err != nil {
		t.Fatalf("Failed to add test model 1: %v", err)
	}
	if err := cat.AddModel(testModel2); err != nil {
		t.Fatalf("Failed to add test model 2: %v", err)
	}

	// Manually add the model to the author with catalog relationship
	author1, _ := cat.Authors().Get("test-author-1")
	if author1.Models == nil {
		author1.Models = make(map[string]catalogs.Model)
	}
	author1.Models["test-model-1"] = testModel1

	// Add test endpoints
	testEndpoint := catalogs.Endpoint{
		ID: "test-endpoint",
	}
	if err := cat.AddEndpoint(testEndpoint); err != nil {
		t.Fatalf("Failed to add test endpoint: %v", err)
	}

	// Call Save
	if err := cat.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify that the directory structure was created
	expectedDirs := []string{
		"internal/embedded/catalog",
		"internal/embedded/catalog/authors",
		"internal/embedded/catalog/providers",
		"internal/embedded/catalog/authors/test-author-1",
		"internal/embedded/catalog/authors/test-author-2",
		"internal/embedded/catalog/providers/test-provider-1",
	}
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s was not created", dir)
		}
	}

	// Verify that the YAML files were created
	expectedFiles := []string{
		"internal/embedded/catalog/providers.yaml",
		"internal/embedded/catalog/authors.yaml",
		"internal/embedded/catalog/endpoints.yaml",
		"internal/embedded/catalog/authors/test-author-1/test-model-1.yaml",
		"internal/embedded/catalog/authors/test-author-2/test-model-2.yaml",
		"internal/embedded/catalog/providers/test-provider-1/test-model-1.yaml",
	}
	for _, file := range expectedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", file)
		}
	}
}

func TestSaveProviders(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "starmap-save-providers-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cat := NewCatalog()
	provider := catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
		Models: map[string]catalogs.Model{
			"should-not-be-saved": {ID: "should-not-be-saved", Name: "Should Not Be Saved"},
		},
	}
	if err := cat.AddProvider(provider); err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	c := cat.(*catalog)
	if err := c.saveProviders(tmpDir); err != nil {
		t.Fatalf("saveProviders failed: %v", err)
	}

	// Read the saved file
	data, err := os.ReadFile(filepath.Join(tmpDir, "providers.yaml"))
	if err != nil {
		t.Fatalf("Failed to read providers.yaml: %v", err)
	}

	// Unmarshal and verify
	var providers []catalogs.Provider
	if err := yaml.Unmarshal(data, &providers); err != nil {
		t.Fatalf("Failed to unmarshal providers.yaml: %v", err)
	}

	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	if providers[0].ID != "test-provider" {
		t.Errorf("Expected provider ID 'test-provider', got '%s'", providers[0].ID)
	}

	if len(providers[0].Models) > 0 {
		t.Errorf("Expected Models to be empty in saved provider, got %v", providers[0].Models)
	}
}

func TestSaveAuthors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "starmap-save-authors-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cat := NewCatalog()
	author := catalogs.Author{
		ID:   "test-author",
		Name: "Test Author",
		Models: map[string]catalogs.Model{
			"should-not-be-saved": {ID: "should-not-be-saved", Name: "Should Not Be Saved"},
		},
		CreatedAt: utc.Time{},
		UpdatedAt: utc.Time{},
	}
	if err := cat.AddAuthor(author); err != nil {
		t.Fatalf("Failed to add author: %v", err)
	}

	c := cat.(*catalog)
	if err := c.saveAuthors(tmpDir); err != nil {
		t.Fatalf("saveAuthors failed: %v", err)
	}

	// Read the saved file
	data, err := os.ReadFile(filepath.Join(tmpDir, "authors.yaml"))
	if err != nil {
		t.Fatalf("Failed to read authors.yaml: %v", err)
	}

	// Unmarshal and verify
	var authors []catalogs.Author
	if err := yaml.Unmarshal(data, &authors); err != nil {
		t.Fatalf("Failed to unmarshal authors.yaml: %v", err)
	}

	if len(authors) != 1 {
		t.Errorf("Expected 1 author, got %d", len(authors))
	}

	if authors[0].ID != "test-author" {
		t.Errorf("Expected author ID 'test-author', got '%s'", authors[0].ID)
	}

	if len(authors[0].Models) > 0 {
		t.Errorf("Expected Models to be empty in saved author, got %v", authors[0].Models)
	}
}

func TestSaveEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "starmap-save-endpoints-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cat := NewCatalog()
	endpoint := catalogs.Endpoint{
		ID: "test-endpoint",
	}
	if err := cat.AddEndpoint(endpoint); err != nil {
		t.Fatalf("Failed to add endpoint: %v", err)
	}

	c := cat.(*catalog)
	if err := c.saveEndpoints(tmpDir); err != nil {
		t.Fatalf("saveEndpoints failed: %v", err)
	}

	// Read the saved file
	data, err := os.ReadFile(filepath.Join(tmpDir, "endpoints.yaml"))
	if err != nil {
		t.Fatalf("Failed to read endpoints.yaml: %v", err)
	}

	// Unmarshal and verify
	var endpoints []catalogs.Endpoint
	if err := yaml.Unmarshal(data, &endpoints); err != nil {
		t.Fatalf("Failed to unmarshal endpoints.yaml: %v", err)
	}

	if len(endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(endpoints))
	}

	if endpoints[0].ID != "test-endpoint" {
		t.Errorf("Expected endpoint ID 'test-endpoint', got '%s'", endpoints[0].ID)
	}
}

func TestSaveModelsToAuthors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "starmap-save-models-authors-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cat := NewCatalog()
	model := catalogs.Model{
		ID:   "test-model",
		Name: "Test Model",
		Authors: []catalogs.Author{
			{ID: "author-1", Name: "Author 1"},
			{ID: "author-2", Name: "Author 2"},
		},
		CreatedAt: utc.Time{},
		UpdatedAt: utc.Time{},
	}
	if err := cat.AddModel(model); err != nil {
		t.Fatalf("Failed to add model: %v", err)
	}

	c := cat.(*catalog)
	if err := c.saveModelsToAuthors(tmpDir); err != nil {
		t.Fatalf("saveModelsToAuthors failed: %v", err)
	}

	// Verify model files were created for each author
	expectedFiles := []string{
		filepath.Join(tmpDir, "authors", "author-1", "test-model.yaml"),
		filepath.Join(tmpDir, "authors", "author-2", "test-model.yaml"),
	}

	for _, file := range expectedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected model file %s was not created", file)
		}

		// Verify the content
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("Failed to read model file %s: %v", file, err)
		}

		var savedModel catalogs.Model
		if err := yaml.Unmarshal(data, &savedModel); err != nil {
			t.Fatalf("Failed to unmarshal model from %s: %v", file, err)
		}

		if savedModel.ID != "test-model" {
			t.Errorf("Expected model ID 'test-model', got '%s'", savedModel.ID)
		}
	}
}

func TestSaveModelsToProviders(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "starmap-save-models-providers-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cat := NewCatalog()

	// Add an author with a catalog relationship
	author := catalogs.Author{
		ID:   "test-author",
		Name: "Test Author",
		Catalog: &catalogs.AuthorCatalog{
			ProviderID: "test-provider",
		},
		Models: map[string]catalogs.Model{
			"test-model": {
				ID:        "test-model",
				Name:      "Test Model",
				CreatedAt: utc.Time{},
				UpdatedAt: utc.Time{},
			},
		},
		CreatedAt: utc.Time{},
		UpdatedAt: utc.Time{},
	}
	if err := cat.AddAuthor(author); err != nil {
		t.Fatalf("Failed to add author: %v", err)
	}

	c := cat.(*catalog)
	if err := c.saveModelsToProviders(tmpDir); err != nil {
		t.Fatalf("saveModelsToProviders failed: %v", err)
	}

	// Verify model file was created for the provider
	expectedFile := filepath.Join(tmpDir, "providers", "test-provider", "test-model.yaml")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected model file %s was not created", expectedFile)
	}

	// Verify the content
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read model file %s: %v", expectedFile, err)
	}

	var savedModel catalogs.Model
	if err := yaml.Unmarshal(data, &savedModel); err != nil {
		t.Fatalf("Failed to unmarshal model from %s: %v", expectedFile, err)
	}

	if savedModel.ID != "test-model" {
		t.Errorf("Expected model ID 'test-model', got '%s'", savedModel.ID)
	}
}
