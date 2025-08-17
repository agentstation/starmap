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
