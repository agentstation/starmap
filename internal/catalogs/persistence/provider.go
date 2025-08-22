package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/catalogs/operations"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/goccy/go-yaml"
)

// GetProviderModels returns all models for a specific provider from the catalog
func GetProviderModels(catalog catalogs.Catalog, providerID catalogs.ProviderID) (map[string]catalogs.Model, error) {
	result := make(map[string]catalogs.Model)

	// First, try to get models directly from the provider if they exist
	provider, err := catalog.Provider(providerID)
	if err == nil && provider.Models != nil {
		// Copy models from provider
		for modelID, model := range provider.Models {
			result[modelID] = model
		}
	}

	// Note: We no longer check author models here to maintain provider isolation
	// Models should only come from the provider's own directory to avoid
	// cross-provider contamination during sync operations

	// Finally, try loading from the providers directory on disk if we don't have any models yet
	if len(result) == 0 {
		diskModels, err := LoadProviderModels(providerID)
		if err != nil {
			// Log the error but don't fail completely
			// return nil, fmt.Errorf("loading provider models from disk: %w", err)
		} else {
			result = diskModels
		}
	}

	return result, nil
}

// ApplyChangeset applies a provider changeset to the catalog and saves to disk
func ApplyChangeset(catalog catalogs.Catalog, changeset operations.ProviderChangeset) error {
	return ApplyChangesetToOutput(catalog, changeset, "")
}

// ApplyChangesetToOutput applies a provider changeset to the catalog and saves to a custom output directory
func ApplyChangesetToOutput(catalog catalogs.Catalog, changeset operations.ProviderChangeset, outputDir string) error {
	// Apply additions
	for _, model := range changeset.Added {
		// Check if model exists first
		if existingModel, err := catalog.Model(model.ID); err == nil {
			// Model exists - perform smart merge
			mergedModel := operations.SmartMergeModels(existingModel, model)
			if err := catalog.UpdateModel(mergedModel); err != nil {
				return fmt.Errorf("updating existing model %s: %w", model.ID, err)
			}
		} else {
			// Model doesn't exist - add it
			if err := catalog.AddModel(model); err != nil {
				return fmt.Errorf("adding new model %s: %w", model.ID, err)
			}
		}
	}

	// Apply updates
	for _, update := range changeset.Updated {
		// For updates, also do smart merge with existing model
		if existingModel, err := catalog.Model(update.ModelID); err == nil {
			mergedModel := operations.SmartMergeModels(existingModel, update.NewModel)
			if err := catalog.UpdateModel(mergedModel); err != nil {
				return fmt.Errorf("updating model %s: %w", update.ModelID, err)
			}
		} else {
			// Model doesn't exist - treat as addition
			if err := catalog.AddModel(update.NewModel); err != nil {
				return fmt.Errorf("adding model %s during update: %w", update.ModelID, err)
			}
		}
	}

	// Note: We don't automatically remove models as they might be manually maintained

	// Save the updated catalog to disk
	if err := SaveProviderModelsToOutput(changeset.ProviderID, getProviderModelsFromChangeset(changeset), outputDir); err != nil {
		return fmt.Errorf("saving provider models: %w", err)
	}

	// Also save models to authors directory based on catalog mapping
	if err := SaveAuthorModelsToOutput(catalog, changeset.ProviderID, getProviderModelsFromChangeset(changeset), outputDir); err != nil {
		return fmt.Errorf("saving author models: %w", err)
	}

	return nil
}

// getProviderModelsFromChangeset extracts all models (added + updated) from a changeset
func getProviderModelsFromChangeset(changeset operations.ProviderChangeset) []catalogs.Model {
	var models []catalogs.Model

	// Add new models
	models = append(models, changeset.Added...)

	// Add updated models (using the new version)
	for _, update := range changeset.Updated {
		models = append(models, update.NewModel)
	}

	return models
}

// SaveProviderModels saves all models for a provider to the providers directory
func SaveProviderModels(providerID catalogs.ProviderID, models []catalogs.Model) error {
	return SaveProviderModelsToOutput(providerID, models, "")
}

// SaveProviderModelsToOutput saves all models for a provider to a custom output directory
func SaveProviderModelsToOutput(providerID catalogs.ProviderID, models []catalogs.Model, outputDir string) error {
	// Create provider directory if it doesn't exist
	var providerDir string
	if outputDir != "" {
		providerDir = filepath.Join(outputDir, string(providerID))
	} else {
		providerDir = filepath.Join("internal", "embedded", "catalog", "providers", string(providerID))
	}

	if err := os.MkdirAll(providerDir, 0755); err != nil {
		return fmt.Errorf("creating provider directory: %w", err)
	}

	// Save each model as a separate YAML file
	for _, model := range models {
		if err := SaveProviderModelToOutput(providerID, model, outputDir); err != nil {
			return fmt.Errorf("saving model %s: %w", model.ID, err)
		}
	}

	return nil
}

// SaveProviderModel saves a single model to the providers directory
func SaveProviderModel(providerID catalogs.ProviderID, model catalogs.Model) error {
	return SaveProviderModelToOutput(providerID, model, "")
}

// SaveProviderModelToOutput saves a single model to a custom output directory
func SaveProviderModelToOutput(providerID catalogs.ProviderID, model catalogs.Model, outputDir string) error {
	// Create provider directory if it doesn't exist
	var providerDir string
	if outputDir != "" {
		providerDir = filepath.Join(outputDir, string(providerID))
	} else {
		providerDir = filepath.Join("internal", "embedded", "catalog", "providers", string(providerID))
	}

	if err := os.MkdirAll(providerDir, 0755); err != nil {
		return fmt.Errorf("creating provider directory: %w", err)
	}

	// Create nested directories for models with '/' in their ID
	var modelFile string
	if strings.Contains(model.ID, "/") {
		// Split the model ID to create nested directory structure
		parts := strings.Split(model.ID, "/")
		// All parts except the last are directories, last part is the filename
		nestedPath := filepath.Join(parts[:len(parts)-1]...)
		nestedDir := filepath.Join(providerDir, nestedPath)

		// Create nested directories
		if err := os.MkdirAll(nestedDir, 0755); err != nil {
			return fmt.Errorf("creating nested directory: %w", err)
		}

		modelFile = filepath.Join(nestedDir, parts[len(parts)-1]+".yaml")
	} else {
		// Simple model ID without '/', save directly in provider directory
		modelFile = filepath.Join(providerDir, model.ID+".yaml")
	}

	// Generate structured YAML with comments
	yamlContent := GenerateStructuredModelYAML(model)

	if err := os.WriteFile(modelFile, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("writing model file: %w", err)
	}

	return nil
}

// LoadProviderModels loads all models for a provider from the providers directory
func LoadProviderModels(providerID catalogs.ProviderID) (map[string]catalogs.Model, error) {
	models := make(map[string]catalogs.Model)

	providerDir := filepath.Join("internal", "embedded", "catalog", "providers", string(providerID))

	// Check if directory exists
	if _, err := os.Stat(providerDir); os.IsNotExist(err) {
		return models, nil // No models for this provider yet
	}

	// Recursively walk through the provider directory to find all YAML files
	err := filepath.Walk(providerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-YAML files
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading model file %s: %w", path, err)
		}

		var model catalogs.Model
		if err := yaml.Unmarshal(data, &model); err != nil {
			return fmt.Errorf("parsing model file %s: %w", path, err)
		}

		models[model.ID] = model
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking provider directory: %w", err)
	}

	return models, nil
}

// CleanProviderDirectory removes all existing YAML files for a provider from the output directory
func CleanProviderDirectory(providerID catalogs.ProviderID, outputDir string) error {
	// Determine provider directory path
	var providerDir string
	if outputDir != "" {
		providerDir = filepath.Join(outputDir, string(providerID))
	} else {
		providerDir = filepath.Join("internal", "embedded", "catalog", "providers", string(providerID))
	}

	// Check if directory exists
	if _, err := os.Stat(providerDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean
	}

	// Remove all YAML files recursively in the provider directory
	err := filepath.Walk(providerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Remove YAML files only, preserve directory structure
		if !info.IsDir() && strings.HasSuffix(path, ".yaml") {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("removing file %s: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cleaning provider directory %s: %w", providerDir, err)
	}

	return nil
}

// SaveUpdatedProviders saves providers configuration to providers.yaml, preserving formatting and comments
func SaveUpdatedProviders(updatedProviders map[catalogs.ProviderID]catalogs.Provider, outputDir string) error {
	return SaveUpdatedProvidersWithOptions(updatedProviders, outputDir, false)
}

// SaveUpdatedProvidersWithOptions saves providers configuration with optional force update
func SaveUpdatedProvidersWithOptions(updatedProviders map[catalogs.ProviderID]catalogs.Provider, outputDir string, forceUpdate bool) error {
	// Determine providers.yaml path
	var providersPath string
	if outputDir != "" {
		providersPath = filepath.Join(outputDir, "providers.yaml")
	} else {
		providersPath = filepath.Join("internal", "embedded", "catalog", "providers.yaml")
	}

	// Read the original file to preserve formatting and comments
	originalContent, err := os.ReadFile(providersPath)
	if err != nil {
		return fmt.Errorf("reading original providers.yaml: %w", err)
	}

	// Parse existing providers to understand structure
	var existingProviders []catalogs.Provider
	if err := yaml.Unmarshal(originalContent, &existingProviders); err != nil {
		return fmt.Errorf("parsing existing providers.yaml: %w", err)
	}

	// Create map for easy lookup
	existingMap := make(map[catalogs.ProviderID]catalogs.Provider)
	for _, provider := range existingProviders {
		existingMap[provider.ID] = provider
	}

	// Check if we need to update any providers
	needsUpdate := forceUpdate
	if !needsUpdate {
		for providerID, updatedProvider := range updatedProviders {
			if existing, found := existingMap[providerID]; found {
				// Compare authors field
				if !equalAuthorIDSlices(existing.Authors, updatedProvider.Authors) {
					needsUpdate = true
					break
				}
			}
		}
	}

	if !needsUpdate {
		return nil // No changes needed
	}

	// Merge updated providers with existing ones
	mergedProviders := make([]catalogs.Provider, 0, len(existingProviders))
	for _, provider := range existingProviders {
		if updated, found := updatedProviders[provider.ID]; found {
			// Merge the updated provider but keep all other fields from existing
			mergedProvider := provider
			// Merge authors additively - preserve existing authors and add new ones
			mergedProvider.Authors = mergeAuthorsAdditively(provider.Authors, updated.Authors)
			// Clear runtime fields
			mergedProvider.Models = nil
			mergedProvider.APIKeyValue = ""
			mergedProvider.EnvVarValues = nil
			mergedProviders = append(mergedProviders, mergedProvider)
		} else {
			// Keep existing provider as-is but clear runtime fields
			provider.Models = nil
			provider.APIKeyValue = ""
			provider.EnvVarValues = nil
			mergedProviders = append(mergedProviders, provider)
		}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(providersPath), 0755); err != nil {
		return fmt.Errorf("creating providers directory: %w", err)
	}

	// Marshal to YAML with custom formatting that preserves comments and spacing
	data, err := marshalProvidersWithFormatting(mergedProviders)
	if err != nil {
		return fmt.Errorf("marshaling providers: %w", err)
	}

	// Write the updated content
	if err := os.WriteFile(providersPath, data, 0644); err != nil {
		return fmt.Errorf("writing providers.yaml: %w", err)
	}

	return nil
}

// equalStringSlices compares two string slices for equality
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// equalAuthorIDSlices compares two AuthorID slices for equality
func equalAuthorIDSlices(a, b []catalogs.AuthorID) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// marshalProvidersWithFormatting marshals providers with proper formatting, comments, and spacing
func marshalProvidersWithFormatting(providers []catalogs.Provider) ([]byte, error) {
	// Create comment map for provider section headers and duration comments
	commentMap := yaml.CommentMap{}

	for i, provider := range providers {
		// Add provider section header comment using HeadComment with space formatting
		providerPath := fmt.Sprintf("$[%d]", i)
		commentMap[providerPath] = []*yaml.Comment{
			yaml.HeadComment(fmt.Sprintf(" %s", provider.Name)), // Space prefix for proper formatting
		}

		// Add duration comments
		if provider.RetentionPolicy != nil && provider.RetentionPolicy.Duration != nil {
			retentionPath := fmt.Sprintf("$[%d].retention_policy.duration", i)
			duration := provider.RetentionPolicy.Duration.String()
			var comment string
			switch duration {
			case "720h0m0s", "720h":
				comment = "30 days"
			case "48h0m0s", "48h":
				comment = "2 days"
			default:
				continue
			}
			commentMap[retentionPath] = []*yaml.Comment{
				yaml.LineComment(comment),
			}
		}
	}

	// Let the library handle the formatting properly
	yamlData, err := yaml.MarshalWithOptions(providers,
		yaml.Indent(2),               // 2-space indentation
		yaml.IndentSequence(false),   // Keep root array flush left (no indentation)
		yaml.WithComment(commentMap), // Add comments
	)
	if err != nil {
		return nil, fmt.Errorf("marshaling providers with options: %w", err)
	}

	// Minimal post-processing - just add spacing between providers
	formatted := addBlankLinesBetweenProviders(string(yamlData))

	return []byte(formatted), nil
}

// addBlankLinesBetweenProviders adds spacing between provider sections
func addBlankLinesBetweenProviders(yamlContent string) string {
	lines := strings.Split(yamlContent, "\n")
	var result []string

	for i, line := range lines {
		// Add blank line before each provider comment (except the first one)
		if strings.HasPrefix(line, "#") && i > 0 {
			result = append(result, "")
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// mergeAuthorsAdditively merges existing and new authors, preserving all existing authors and adding new ones
func mergeAuthorsAdditively(existing []catalogs.AuthorID, new []catalogs.AuthorID) []catalogs.AuthorID {
	authorSet := make(map[string]bool)
	
	// ALWAYS preserve existing authors (manual configuration)
	for _, author := range existing {
		if string(author) != "" {
			authorSet[string(author)] = true
		}
	}
	
	// Add new authors (from API discovery or other sources)
	for _, author := range new {
		if string(author) != "" {
			authorSet[string(author)] = true
		}
	}
	
	// Convert back to slice and sort
	var merged []catalogs.AuthorID
	for author := range authorSet {
		merged = append(merged, catalogs.AuthorID(author))
	}
	
	// Sort for consistent output
	for i := 0; i < len(merged); i++ {
		for j := i + 1; j < len(merged); j++ {
			if merged[i] > merged[j] {
				merged[i], merged[j] = merged[j], merged[i]
			}
		}
	}
	
	return merged
}

// SaveAuthorModelsToOutput saves models to the authors directory based on catalog mapping
func SaveAuthorModelsToOutput(catalog catalogs.Catalog, providerID catalogs.ProviderID, models []catalogs.Model, outputDir string) error {
	// Load all authors
	authorsMap := catalog.Authors().Map()
	
	// Find authors whose authoritative provider matches the current provider
	for _, author := range authorsMap {
		if author.Catalog == nil || author.Catalog.ProviderID != providerID {
			continue
		}
		
		// Filter models based on patterns (if specified)
		var authorModels []catalogs.Model
		for _, model := range models {
			if matchesAuthorPatterns(model.ID, author.Catalog.Patterns) {
				authorModels = append(authorModels, model)
			}
		}
		
		// Save matching models to authors/<author_id>/ directory
		for _, model := range authorModels {
			if err := saveModelToAuthorDirectory(author.ID, model, outputDir); err != nil {
				return fmt.Errorf("saving model %s to author %s directory: %w", model.ID, author.ID, err)
			}
		}
	}
	
	return nil
}

// matchesAuthorPatterns checks if a model ID matches any of the author's catalog patterns
func matchesAuthorPatterns(modelID string, patterns []string) bool {
	// If no patterns are specified, all models match
	if len(patterns) == 0 {
		return true
	}
	
	// Check if model ID matches any pattern
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, modelID); matched {
			return true
		}
	}
	
	return false
}

// saveModelToAuthorDirectory saves a single model to an author's directory
func saveModelToAuthorDirectory(authorID catalogs.AuthorID, model catalogs.Model, outputDir string) error {
	// Create author directory path
	var authorDir string
	if outputDir != "" {
		authorDir = filepath.Join(outputDir, "..", "authors", string(authorID))
	} else {
		authorDir = filepath.Join("internal", "embedded", "catalog", "authors", string(authorID))
	}
	
	// Create author directory if it doesn't exist
	if err := os.MkdirAll(authorDir, 0755); err != nil {
		return fmt.Errorf("creating author directory: %w", err)
	}
	
	// Create nested directories for models with '/' in their ID (similar to provider logic)
	var modelFile string
	if strings.Contains(model.ID, "/") {
		// Split the model ID to create nested directory structure
		parts := strings.Split(model.ID, "/")
		// All parts except the last are directories, last part is the filename
		nestedPath := filepath.Join(parts[:len(parts)-1]...)
		nestedDir := filepath.Join(authorDir, nestedPath)
		
		// Create nested directories
		if err := os.MkdirAll(nestedDir, 0755); err != nil {
			return fmt.Errorf("creating nested directory: %w", err)
		}
		
		modelFile = filepath.Join(nestedDir, parts[len(parts)-1]+".yaml")
	} else {
		// Simple model ID without '/', save directly in author directory
		modelFile = filepath.Join(authorDir, model.ID+".yaml")
	}
	
	// Generate structured YAML with comments (reuse the existing function)
	yamlContent := GenerateStructuredModelYAML(model)
	
	if err := os.WriteFile(modelFile, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("writing model file: %w", err)
	}
	
	return nil
}
