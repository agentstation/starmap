package attribution

import (
	"testing"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestApply_EmptyCatalog(t *testing.T) {
	catalog := catalogs.NewEmpty()

	err := Apply(catalog)
	if err != nil {
		t.Fatalf("Expected no error for empty catalog, got: %v", err)
	}

	// Should have no authors with models
	for _, author := range catalog.Authors().List() {
		if len(author.Models) > 0 {
			t.Errorf("Expected no models for author %s, got %d", author.ID, len(author.Models))
		}
	}
}

func TestApply_NilChecks(t *testing.T) {
	// Test with authors that have nil catalog/attribution
	catalog := catalogs.NewEmpty()

	// Add authors with nil catalog/attribution fields
	author1 := catalogs.Author{
		ID:        "nil-catalog",
		Name:      "Nil Catalog",
		Catalog:   nil, // Nil catalog
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}
	author2 := catalogs.Author{
		ID:   "nil-attribution",
		Name: "Nil Attribution",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: nil, // Nil attribution
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}

	if err := catalog.SetAuthor(author1); err != nil {
		t.Fatalf("Failed to set author1: %v", err)
	}
	if err := catalog.SetAuthor(author2); err != nil {
		t.Fatalf("Failed to set author2: %v", err)
	}

	// Should not panic or error with nil catalog/attribution fields
	err := Apply(catalog)
	if err != nil {
		t.Fatalf("Expected no error with nil fields, got: %v", err)
	}

	// Verify no models attributed
	for _, author := range catalog.Authors().List() {
		if len(author.Models) > 0 {
			t.Errorf("Expected no models for author %s with nil fields, got %d", author.ID, len(author.Models))
		}
	}
}

func TestApply_ProviderOnlyMode(t *testing.T) {
	// Test Mode 1: Provider-only attribution
	catalog := catalogs.NewEmpty()

	// Create author with provider-only attribution
	author := catalogs.Author{
		ID:   "anthropic",
		Name: "Anthropic",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: &catalogs.AuthorAttribution{
				ProviderID: "anthropic",
				// No patterns - Mode 1
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}

	// Create provider with models
	provider := catalogs.Provider{
		ID:   "anthropic",
		Name: "Anthropic",
		Models: map[string]*catalogs.Model{
			"claude-3-opus":   &catalogs.Model{ID: "claude-3-opus", Name: "Claude 3 Opus"},
			"claude-3-sonnet": &catalogs.Model{ID: "claude-3-sonnet", Name: "Claude 3 Sonnet"},
		},
	}

	// Add to catalog
	if err := catalog.SetAuthor(author); err != nil {
		t.Fatalf("Failed to set author: %v", err)
	}
	if err := catalog.SetProvider(provider); err != nil {
		t.Fatalf("Failed to set provider: %v", err)
	}

	// Apply attributions
	err := Apply(catalog)
	if err != nil {
		t.Fatalf("Failed to apply attributions: %v", err)
	}

	// Verify author has both models
	updatedAuthor, err := catalog.Author("anthropic")
	if err != nil {
		t.Fatalf("Failed to get author: %v", err)
	}

	if len(updatedAuthor.Models) != 2 {
		t.Errorf("Expected 2 models for author, got %d", len(updatedAuthor.Models))
	}

	// Verify specific models
	expectedModels := []string{"claude-3-opus", "claude-3-sonnet"}
	for _, modelID := range expectedModels {
		if _, exists := updatedAuthor.Models[modelID]; !exists {
			t.Errorf("Expected model %s in author.Models, not found", modelID)
		}
	}
}

func TestApply_GlobalPatternsMode(t *testing.T) {
	// Test Mode 3: Global pattern matching
	catalog := catalogs.NewEmpty()

	// Create author with global patterns
	author := catalogs.Author{
		ID:   "meta",
		Name: "Meta",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: &catalogs.AuthorAttribution{
				Patterns: []string{"llama*", "*-llama-*"},
				// Mode 3 - no provider
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}

	// Create a provider with test models
	provider := catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
		Models: map[string]*catalogs.Model{
			"llama-3-8b":   &catalogs.Model{ID: "llama-3-8b", Name: "Llama 3 8B"},
			"meta-llama-3": &catalogs.Model{ID: "meta-llama-3", Name: "Meta Llama 3"},
			"mixtral-8x7b": &catalogs.Model{ID: "mixtral-8x7b", Name: "Mixtral 8x7B"},
			"LLAMA-BIG":    &catalogs.Model{ID: "LLAMA-BIG", Name: "Llama Big"},
		},
	}

	// Add to catalog
	if err := catalog.SetAuthor(author); err != nil {
		t.Fatalf("Failed to set author: %v", err)
	}
	if err := catalog.SetProvider(provider); err != nil {
		t.Fatalf("Failed to set provider: %v", err)
	}

	// Apply attributions
	err := Apply(catalog)
	if err != nil {
		t.Fatalf("Failed to apply attributions: %v", err)
	}

	// Verify author has matching models
	updatedAuthor, err := catalog.Author("meta")
	if err != nil {
		t.Fatalf("Failed to get author: %v", err)
	}

	tests := []struct {
		modelID     string
		shouldMatch bool
	}{
		{"llama-3-8b", true},    // Matches "llama*"
		{"meta-llama-3", true},  // Matches "*-llama-*"
		{"mixtral-8x7b", false}, // No match
		{"LLAMA-BIG", true},     // Case insensitive match
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			_, exists := updatedAuthor.Models[tt.modelID]
			if tt.shouldMatch && !exists {
				t.Errorf("Expected model %s to be attributed to author, not found", tt.modelID)
			}
			if !tt.shouldMatch && exists {
				t.Errorf("Expected model %s NOT to be attributed to author, but it was", tt.modelID)
			}
		})
	}
}

func TestApply_ProviderPatternMode(t *testing.T) {
	// Test Mode 2: Provider + patterns
	catalog := catalogs.NewEmpty()

	// Create author with provider + patterns
	author := catalogs.Author{
		ID:   "moonshot-ai",
		Name: "Moonshot AI",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: &catalogs.AuthorAttribution{
				ProviderID: "moonshot-ai",
				Patterns:   []string{"*kimi*", "moonshot-ai/*"},
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}

	// Create provider with models
	provider := catalogs.Provider{
		ID:   "moonshot-ai",
		Name: "Moonshot AI",
		Models: map[string]*catalogs.Model{
			"kimi-k2-0711-preview":    &catalogs.Model{ID: "kimi-k2-0711-preview", Name: "Kimi K2 Preview"},
			"moonshot-v1-128k":        &catalogs.Model{ID: "moonshot-v1-128k", Name: "Moonshot v1 128K"},
			"moonshot-ai/other-model": &catalogs.Model{ID: "moonshot-ai/other-model", Name: "Other Model"},
			"unrelated-model":         &catalogs.Model{ID: "unrelated-model", Name: "Unrelated"},
		},
	}

	// Add to catalog
	if err := catalog.SetAuthor(author); err != nil {
		t.Fatalf("Failed to set author: %v", err)
	}
	if err := catalog.SetProvider(provider); err != nil {
		t.Fatalf("Failed to set provider: %v", err)
	}

	// Apply attributions
	err := Apply(catalog)
	if err != nil {
		t.Fatalf("Failed to apply attributions: %v", err)
	}

	// Verify author has matching models only
	updatedAuthor, err := catalog.Author("moonshot-ai")
	if err != nil {
		t.Fatalf("Failed to get author: %v", err)
	}

	tests := []struct {
		modelID     string
		shouldMatch bool
	}{
		{"kimi-k2-0711-preview", true},    // Matches "*kimi*"
		{"moonshot-v1-128k", false},       // Doesn't match patterns
		{"moonshot-ai/other-model", true}, // Matches "moonshot-ai/*"
		{"unrelated-model", false},        // No match
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			_, exists := updatedAuthor.Models[tt.modelID]
			if tt.shouldMatch && !exists {
				t.Errorf("Expected model %s to be attributed to author, not found", tt.modelID)
			}
			if !tt.shouldMatch && exists {
				t.Errorf("Expected model %s NOT to be attributed to author, but it was", tt.modelID)
			}
		})
	}

	// Verify total count
	expectedCount := 2 // kimi-k2-0711-preview and moonshot-ai/other-model
	if len(updatedAuthor.Models) != expectedCount {
		t.Errorf("Expected %d models for author, got %d", expectedCount, len(updatedAuthor.Models))
	}
}

func TestApply_InvalidPatterns(t *testing.T) {
	// Test that invalid glob patterns cause an error
	catalog := catalogs.NewEmpty()

	// Create author with invalid pattern
	author := catalogs.Author{
		ID:   "bad",
		Name: "Bad Pattern Author",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: &catalogs.AuthorAttribution{
				Patterns: []string{"[invalid"}, // Invalid glob pattern
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}

	err := catalog.SetAuthor(author)
	if err != nil {
		t.Fatalf("Failed to set author: %v", err)
	}

	err = Apply(catalog)
	if err == nil {
		t.Fatal("Expected error for invalid patterns, got nil")
	}

	// The error should mention pattern compilation failure
	if !contains(err.Error(), "pattern") {
		t.Errorf("Expected error to mention pattern, got: %v", err)
	}
}

func TestApply_MultipleAuthors(t *testing.T) {
	// Test a model that matches patterns from multiple authors
	catalog := catalogs.NewEmpty()

	// Create two authors with overlapping patterns
	author1 := catalogs.Author{
		ID:   "author1",
		Name: "Author 1",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: &catalogs.AuthorAttribution{
				Patterns: []string{"shared-*"},
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}
	author2 := catalogs.Author{
		ID:   "author2",
		Name: "Author 2",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: &catalogs.AuthorAttribution{
				Patterns: []string{"*-model"},
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}

	// Create a provider with test model
	provider := catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
		Models: map[string]*catalogs.Model{
			"shared-model": &catalogs.Model{ID: "shared-model", Name: "Shared Model"},
		},
	}

	// Add to catalog
	if err := catalog.SetAuthor(author1); err != nil {
		t.Fatalf("Failed to set author1: %v", err)
	}
	if err := catalog.SetAuthor(author2); err != nil {
		t.Fatalf("Failed to set author2: %v", err)
	}
	if err := catalog.SetProvider(provider); err != nil {
		t.Fatalf("Failed to set provider: %v", err)
	}

	// Apply attributions
	err := Apply(catalog)
	if err != nil {
		t.Fatalf("Failed to apply attributions: %v", err)
	}

	// Verify both authors have the shared model
	for _, authorID := range []catalogs.AuthorID{"author1", "author2"} {
		author, err := catalog.Author(authorID)
		if err != nil {
			t.Fatalf("Failed to get author %s: %v", authorID, err)
		}

		if _, exists := author.Models["shared-model"]; !exists {
			t.Errorf("Expected shared-model in author %s, not found", authorID)
		}
	}
}

func TestApply_NewProviderWithModels(t *testing.T) {
	// Test the moonshot-ai scenario: new provider with 12 models
	catalog := catalogs.NewEmpty()

	// Create moonshot-ai author with attribution patterns
	author := catalogs.Author{
		ID:   "moonshot-ai",
		Name: "Moonshot AI",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: &catalogs.AuthorAttribution{
				ProviderID: "moonshot-ai",
				Patterns:   []string{"*kimi*", "moonshot-ai/*"},
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}

	// Create provider with 12 models (like API fetch)
	models := make(map[string]*catalogs.Model)
	modelIDs := []string{
		"kimi-k2-0711-preview",
		"kimi-k2-0905-preview",
		"moonshot-v1-128k",
		"moonshot-v1-32k",
		"moonshot-v1-8k",
	}

	for _, id := range modelIDs {
		models[id] = &catalogs.Model{
			ID:   id,
			Name: id,
		}
	}

	provider := catalogs.Provider{
		ID:     "moonshot-ai",
		Name:   "Moonshot AI",
		Models: models,
	}

	// Add to catalog
	if err := catalog.SetAuthor(author); err != nil {
		t.Fatalf("Failed to set author: %v", err)
	}
	if err := catalog.SetProvider(provider); err != nil {
		t.Fatalf("Failed to set provider: %v", err)
	}

	// Apply attributions
	err := Apply(catalog)
	if err != nil {
		t.Fatalf("Failed to apply attributions: %v", err)
	}

	// Verify author has kimi models attributed
	updatedAuthor, err := catalog.Author("moonshot-ai")
	if err != nil {
		t.Fatalf("Failed to get author: %v", err)
	}

	// Should have 2 models (the kimi ones)
	expectedModels := []string{
		"kimi-k2-0711-preview",
		"kimi-k2-0905-preview",
	}

	for _, modelID := range expectedModels {
		if _, exists := updatedAuthor.Models[modelID]; !exists {
			t.Errorf("Expected model %s in author.Models, not found", modelID)
		}
	}

	// Verify moonshot-v1-* models are NOT attributed (don't match patterns)
	notExpected := []string{
		"moonshot-v1-128k",
		"moonshot-v1-32k",
		"moonshot-v1-8k",
	}

	for _, modelID := range notExpected {
		if _, exists := updatedAuthor.Models[modelID]; exists {
			t.Errorf("Model %s should NOT be in author.Models (doesn't match patterns)", modelID)
		}
	}
}

// Helper functions

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
