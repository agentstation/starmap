package attribution

import (
	"testing"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestAttributions_NewWithEmptyCatalog(t *testing.T) {
	catalog := catalogs.NewEmpty()

	attributions, err := New(catalog)
	if err != nil {
		t.Fatalf("Expected no error for empty catalog, got: %v", err)
	}

	// Should return no attribution for any model
	authors, found := attributions.Attribute("any-model")
	if found {
		t.Errorf("Expected no attribution, got %v", authors)
	}
}

func TestAttributions_NilChecks(t *testing.T) {
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
	attributions, err := New(catalog)
	if err != nil {
		t.Fatalf("Expected no error with nil fields, got: %v", err)
	}

	authors, found := attributions.Attribute("any-model")
	if found {
		t.Errorf("Expected no attribution, got %v", authors)
	}
}

func TestAttributions_ProviderOnlyMode(t *testing.T) {
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

	attributions, err := New(catalog)
	if err != nil {
		t.Fatalf("Failed to create attributions: %v", err)
	}

	// Test that both models are attributed to Anthropic
	tests := []struct {
		modelID  string
		expected catalogs.AuthorID
	}{
		{"claude-3-opus", "anthropic"},
		{"claude-3-sonnet", "anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			authors, found := attributions.Attribute(tt.modelID)
			if !found {
				t.Errorf("Expected attribution for %s, got none", tt.modelID)
				return
			}

			if len(authors) != 1 {
				t.Errorf("Expected 1 author, got %d", len(authors))
				return
			}

			if authors[0] != tt.expected {
				t.Errorf("Expected author %s, got %s", tt.expected, authors[0])
			}
		})
	}
}

func TestAttributions_GlobalPatternsMode(t *testing.T) {
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

	attributions, err := New(catalog)
	if err != nil {
		t.Fatalf("Failed to create attributions: %v", err)
	}

	tests := []struct {
		modelID        string
		shouldMatch    bool
		expectedAuthor catalogs.AuthorID
	}{
		{"llama-3-8b", true, "meta"},   // Matches "llama*"
		{"meta-llama-3", true, "meta"}, // Matches "*-llama-*"
		{"mixtral-8x7b", false, ""},    // No match
		{"LLAMA-BIG", true, "meta"},    // Case insensitive match
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			authors, found := attributions.Attribute(tt.modelID)

			if tt.shouldMatch {
				if !found {
					t.Errorf("Expected attribution for %s, got none", tt.modelID)
					return
				}
				if len(authors) != 1 {
					t.Errorf("Expected 1 author, got %d", len(authors))
					return
				}
				if authors[0] != tt.expectedAuthor {
					t.Errorf("Expected author %s, got %s", tt.expectedAuthor, authors[0])
				}
			} else {
				if found {
					t.Errorf("Expected no attribution for %s, got %v", tt.modelID, authors)
				}
			}
		})
	}
}

func TestAttributions_InvalidPatterns(t *testing.T) {
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

	_, err = New(catalog)
	if err == nil {
		t.Fatal("Expected error for invalid patterns, got nil")
	}

	// The error should mention pattern compilation failure
	if !contains(err.Error(), "pattern") {
		t.Errorf("Expected error to mention pattern, got: %v", err)
	}
}

func TestAttributions_MultipleAuthors(t *testing.T) {
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

	attributions, err := New(catalog)
	if err != nil {
		t.Fatalf("Failed to create attributions: %v", err)
	}

	// Test a model that should be attributed to multiple authors
	authors, found := attributions.Attribute("shared-model")
	if !found {
		t.Fatal("Expected to find attribution for shared-model")
	}

	// Should have multiple authors
	if len(authors) != 2 {
		t.Errorf("Expected 2 authors, got %d: %v", len(authors), authors)
	}

	// Check that both expected authors are present (order may vary)
	expectedAuthors := map[catalogs.AuthorID]bool{"author1": false, "author2": false}
	for _, author := range authors {
		if _, exists := expectedAuthors[author]; exists {
			expectedAuthors[author] = true
		} else {
			t.Errorf("Unexpected author: %s", author)
		}
	}

	for author, found := range expectedAuthors {
		if !found {
			t.Errorf("Expected author %s not found in results", author)
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
