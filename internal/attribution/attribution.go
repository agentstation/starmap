// Package attribution provides model-to-author mapping functionality across multiple providers.
// It supports three attribution modes to handle different provider scenarios:
//
//  1. Provider-only: All models from a specific provider belong to an author
//  2. Provider + patterns: Only matching models from a provider, then cross-provider attribution
//  3. Global patterns: Direct pattern matching across all providers
package attribution

import (
	"fmt"
	"sync"

	"github.com/agentstation/starmap/internal/matcher"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Attributions defines the interface for model attribution functionality.
type Attributions interface {
	// Attribute attempts to match a model id to the author of a model. A model can have multiple authors.
	// Returns the list of authors and true if any attributions were found.
	Attribute(modelID string) ([]catalogs.AuthorID, bool)
}

// attributions implements Attribution with thread-safe operations.
type attributions struct {
	mu           sync.RWMutex
	attributions map[string][]catalogs.AuthorID
}

// New creates a new attribution service using the provided catalog.
// All attribution mappings are pre-computed during initialization for O(1) lookups.
func New(catalog catalogs.Reader) (Attributions, error) {
	a := &attributions{
		attributions: make(map[string][]catalogs.AuthorID),
	}

	// Process authors in priority order: provider-only, provider+patterns, patterns-only
	if err := a.processProviderOnlyAttributions(catalog); err != nil {
		return nil, fmt.Errorf("failed to process provider-only attributions: %w", err)
	}

	if err := a.processProviderPatternAttributions(catalog); err != nil {
		return nil, fmt.Errorf("failed to process provider+pattern attributions: %w", err)
	}

	if err := a.processGlobalPatternAttributions(catalog); err != nil {
		return nil, fmt.Errorf("failed to process global pattern attributions: %w", err)
	}

	return a, nil
}

// processProviderOnlyAttributions handles Mode 1: All models from a provider belong to an author.
func (a *attributions) processProviderOnlyAttributions(catalog catalogs.Reader) error {
	for _, author := range catalog.Authors().List() {
		// Check for nil pointers
		if author.Catalog == nil || author.Catalog.Attribution == nil {
			continue
		}

		attr := author.Catalog.Attribution
		hasProviderID := attr.ProviderID != ""
		hasPatterns := len(attr.Patterns) > 0

		// Mode 1: Provider-only (has provider, no patterns)
		if hasProviderID && !hasPatterns {
			provider, ok := catalog.Providers().Get(attr.ProviderID)
			if !ok {
				continue // Provider not found, skip
			}

			// Attribute all models from this provider to this author
			for _, model := range provider.Models {
				a.attributions[model.ID] = append(a.attributions[model.ID], author.ID)
			}
		}
	}

	return nil
}

// processProviderPatternAttributions handles Mode 2: Only matching models from a provider.
func (a *attributions) processProviderPatternAttributions(catalog catalogs.Reader) error {
	for _, author := range catalog.Authors().List() {
		// Check for nil pointers
		if author.Catalog == nil || author.Catalog.Attribution == nil {
			continue
		}

		attr := author.Catalog.Attribution
		hasProviderID := attr.ProviderID != ""
		hasPatterns := len(attr.Patterns) > 0

		// Mode 2: Provider + patterns
		if hasProviderID && hasPatterns {
			provider, ok := catalog.Providers().Get(attr.ProviderID)
			if !ok {
				continue // Provider not found, skip
			}

			// Create multi-matcher for all patterns
			multiMatcher, err := matcher.NewMultiMatcher(attr.Patterns, matcher.Glob, &matcher.Options{
				CaseInsensitive: true,
			})
			if err != nil {
				return fmt.Errorf("failed to create pattern matcher for author %s: %w", author.ID, err)
			}

			// Check each model from this provider against the patterns
			for _, model := range provider.Models {
				if multiMatcher.Match(model.ID) {
					a.attributions[model.ID] = append(a.attributions[model.ID], author.ID)
				}
			}
		}
	}

	return nil
}

// processGlobalPatternAttributions handles Mode 3: Pattern matching across all providers.
func (a *attributions) processGlobalPatternAttributions(catalog catalogs.Reader) error {
	for _, author := range catalog.Authors().List() {
		// Check for nil pointers
		if author.Catalog == nil || author.Catalog.Attribution == nil {
			continue
		}

		attr := author.Catalog.Attribution
		hasProviderID := attr.ProviderID != ""
		hasPatterns := len(attr.Patterns) > 0

		// Mode 3: Global patterns only (no provider specified)
		if !hasProviderID && hasPatterns {
			// Create multi-matcher for all patterns
			multiMatcher, err := matcher.NewMultiMatcher(attr.Patterns, matcher.Glob, &matcher.Options{
				CaseInsensitive: true,
			})
			if err != nil {
				return fmt.Errorf("failed to create global pattern matcher for author %s: %w", author.ID, err)
			}

			// Check all models across all providers
			for _, model := range catalog.Models().List() {
				if multiMatcher.Match(model.ID) {
					a.attributions[model.ID] = append(a.attributions[model.ID], author.ID)
				}
			}
		}
	}

	return nil
}

// Attribute finds the author(s) of a model by its ID.
// Returns the list of authors and true if any attributions were found.
func (a *attributions) Attribute(modelID string) ([]catalogs.AuthorID, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	authors, found := a.attributions[modelID]
	if !found || len(authors) == 0 {
		return []catalogs.AuthorID{}, false
	}

	// Return a copy to prevent external modification
	result := make([]catalogs.AuthorID, len(authors))
	copy(result, authors)
	return result, true
}
