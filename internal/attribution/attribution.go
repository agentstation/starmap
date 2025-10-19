// Package attribution provides model-to-author mapping functionality across multiple providers.
// It supports three attribution modes to handle different provider scenarios:
//
//  1. Provider-only: All models from a specific provider belong to an author
//  2. Provider + patterns: Only matching models from a provider, then cross-provider attribution
//  3. Global patterns: Direct pattern matching across all providers
package attribution

import (
	"fmt"

	"github.com/agentstation/starmap/internal/matcher"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/logging"
)

// Apply populates author.Models in the catalog using attribution patterns from authors.yaml.
// This is the primary entry point for applying attributions.
//
// The function:
//  1. Builds attribution mappings from catalog's authors
//  2. Matches models to authors based on patterns
//  3. Populates author.Models for each matched author
//
// Example:
//
//	if err := attribution.Apply(catalog); err != nil {
//	    return fmt.Errorf("failed to apply attributions: %w", err)
//	}
func Apply(catalog catalogs.Catalog) error {
	// Build attribution mappings from catalog's authors
	attributions, err := buildAttributions(catalog)
	if err != nil {
		return err
	}

	// Apply attributions to populate author.Models
	return applyAttributions(catalog, attributions)
}

// buildAttributions creates model-to-author mappings from catalog's authors.
// All attribution mappings are pre-computed for O(1) lookups during application.
func buildAttributions(catalog catalogs.Reader) (map[string][]catalogs.AuthorID, error) {
	attributions := make(map[string][]catalogs.AuthorID)

	// Process authors in priority order: provider-only, provider+patterns, patterns-only
	if err := processProviderOnlyAttributions(catalog, attributions); err != nil {
		return nil, fmt.Errorf("failed to process provider-only attributions: %w", err)
	}

	if err := processProviderPatternAttributions(catalog, attributions); err != nil {
		return nil, fmt.Errorf("failed to process provider+pattern attributions: %w", err)
	}

	if err := processGlobalPatternAttributions(catalog, attributions); err != nil {
		return nil, fmt.Errorf("failed to process global pattern attributions: %w", err)
	}

	return attributions, nil
}

// processProviderOnlyAttributions handles Mode 1: All models from a provider belong to an author.
func processProviderOnlyAttributions(catalog catalogs.Reader, attributions map[string][]catalogs.AuthorID) error {
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
				attributions[model.ID] = append(attributions[model.ID], author.ID)
			}
		}
	}

	return nil
}

// processProviderPatternAttributions handles Mode 2: Only matching models from a provider.
func processProviderPatternAttributions(catalog catalogs.Reader, attributions map[string][]catalogs.AuthorID) error {
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
					attributions[model.ID] = append(attributions[model.ID], author.ID)
				}
			}
		}
	}

	return nil
}

// processGlobalPatternAttributions handles Mode 3: Pattern matching across all providers.
func processGlobalPatternAttributions(catalog catalogs.Reader, attributions map[string][]catalogs.AuthorID) error {
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
					attributions[model.ID] = append(attributions[model.ID], author.ID)
				}
			}
		}
	}

	return nil
}

// applyAttributions populates author.Models in the catalog using the attribution map.
// This uses the attribution patterns from authors.yaml to determine which models
// belong to which authors. Models may have .Authors field set by provider clients
// (for display/metadata), but author.Models is populated based on attribution patterns.
func applyAttributions(cat catalogs.Catalog, attributions map[string][]catalogs.AuthorID) error {
	// Get all models from all providers
	allModels := cat.Models().List()

	// For each model, determine which authors it belongs to via attribution
	for _, model := range allModels {
		authorIDs, found := attributions[model.ID]
		if !found || len(authorIDs) == 0 {
			continue // No authors matched this model via attribution
		}

		// Add model to each matching author's Models map
		for _, authorID := range authorIDs {
			// Get the author
			author, err := cat.Author(authorID)
			if err != nil {
				logging.Debug().
					Err(err).
					Str("author", string(authorID)).
					Str("model", model.ID).
					Msg("Author not found, skipping")
				continue
			}

			// Initialize Models map if needed
			if author.Models == nil {
				author.Models = make(map[string]*catalogs.Model)
			}

			// Add model to author - need to copy model value and take address
			modelCopy := model
			author.Models[model.ID] = &modelCopy

			// Update the author in catalog
			if err := cat.SetAuthor(author); err != nil {
				logging.Debug().
					Err(err).
					Str("author", string(authorID)).
					Str("model", model.ID).
					Msg("Failed to add model to author")
				continue
			}
		}
	}

	return nil
}
