package embedded

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/goccy/go-yaml"
)

const (
	// file paths
	baseEmbeddedCatalogPath = "internal/embedded/catalog"
	authorsPath             = "authors"
	providersPath           = "providers"
	modelsPath              = "models"
	endpointsPath           = "endpoints"
	providersFile           = "providers.yaml"
	authorsFile             = "authors.yaml"
	endpointsFile           = "endpoints.yaml"
	modelsFile              = "models.yaml"

	// file extensions
	yamlExtension = ".yaml"

	// file modes
	dirMode  = 0755
	fileMode = 0644
)

// Save saves the catalog to YAML files in the embedded catalog directory structure
func (c *catalog) Save() error {
	// Create directories if they don't exist
	authorsDir := filepath.Join(baseEmbeddedCatalogPath, authorsPath)
	if err := os.MkdirAll(authorsDir, dirMode); err != nil {
		return fmt.Errorf("creating authors directory: %w", err)
	}
	providersDir := filepath.Join(baseEmbeddedCatalogPath, providersPath)
	if err := os.MkdirAll(providersDir, dirMode); err != nil {
		return fmt.Errorf("creating providers directory: %w", err)
	}

	// Save providers.yaml (without models)
	if err := c.saveProviders(baseEmbeddedCatalogPath); err != nil {
		return fmt.Errorf("saving providers: %w", err)
	}

	// Save authors.yaml (without models)
	if err := c.saveAuthors(baseEmbeddedCatalogPath); err != nil {
		return fmt.Errorf("saving authors: %w", err)
	}

	// Save endpoints.yaml
	if err := c.saveEndpoints(baseEmbeddedCatalogPath); err != nil {
		return fmt.Errorf("saving endpoints: %w", err)
	}

	// Save individual model files under authors/<author_id>/<model_id>.yaml
	if err := c.saveModelsToAuthors(baseEmbeddedCatalogPath); err != nil {
		return fmt.Errorf("saving models to authors: %w", err)
	}

	// Save individual model files under providers/<provider_id>/<model_id>.yaml
	if err := c.saveModelsToProviders(baseEmbeddedCatalogPath); err != nil {
		return fmt.Errorf("saving models to providers: %w", err)
	}

	return nil
}

// saveProviders saves the providers to providers.yaml without models
func (c *catalog) saveProviders(basePath string) error {
	providersMap := c.Providers().Map()

	// Create a slice without models to save to YAML
	providersToSave := make([]catalogs.Provider, 0, len(providersMap))
	for _, provider := range providersMap {
		// Create a copy without models
		providerCopy := *provider
		providerCopy.Models = nil
		providersToSave = append(providersToSave, providerCopy)
	}

	data, err := yaml.Marshal(providersToSave)
	if err != nil {
		return fmt.Errorf("marshaling providers: %w", err)
	}

	providersFilePath := filepath.Join(basePath, providersFile)
	return os.WriteFile(providersFilePath, data, fileMode)
}

// saveAuthors saves the authors to authors.yaml without models
func (c *catalog) saveAuthors(basePath string) error {
	authorsMap := c.Authors().Map()

	// Create a slice without models to save to YAML
	authorsToSave := make([]catalogs.Author, 0, len(authorsMap))
	for _, author := range authorsMap {
		// Create a copy without models
		authorCopy := *author
		authorCopy.Models = nil
		authorsToSave = append(authorsToSave, authorCopy)
	}

	data, err := yaml.Marshal(authorsToSave)
	if err != nil {
		return fmt.Errorf("marshaling authors: %w", err)
	}

	authorsFilePath := filepath.Join(basePath, authorsFile)
	return os.WriteFile(authorsFilePath, data, fileMode)
}

// saveEndpoints saves the endpoints to endpoints.yaml
func (c *catalog) saveEndpoints(basePath string) error {
	endpointsMap := c.Endpoints().Map()

	// Convert map to slice
	endpointsToSave := make([]catalogs.Endpoint, 0, len(endpointsMap))
	for _, endpoint := range endpointsMap {
		endpointsToSave = append(endpointsToSave, *endpoint)
	}

	data, err := yaml.Marshal(endpointsToSave)
	if err != nil {
		return fmt.Errorf("marshaling endpoints: %w", err)
	}

	endpointsFilePath := filepath.Join(basePath, endpointsFile)
	return os.WriteFile(endpointsFilePath, data, fileMode)
}

// saveModelsToAuthors saves individual model files under authors/<author_id>/<model_id>.yaml
func (c *catalog) saveModelsToAuthors(basePath string) error {
	modelsMap := c.Models().Map()

	for _, model := range modelsMap {
		for _, author := range model.Authors {
			authorDir := filepath.Join(basePath, authorsPath, string(author.ID))
			if err := os.MkdirAll(authorDir, dirMode); err != nil {
				return fmt.Errorf("creating author directory %s: %w", authorDir, err)
			}

			modelFile := filepath.Join(authorDir, model.ID+".yaml")
			data, err := yaml.Marshal(model)
			if err != nil {
				return fmt.Errorf("marshaling model %s: %w", model.ID, err)
			}

			if err := os.WriteFile(modelFile, data, fileMode); err != nil {
				return fmt.Errorf("writing model file %s: %w", modelFile, err)
			}
		}
	}

	return nil
}

// saveModelsToProviders saves individual model files under providers/<provider_id>/<model_id>.yaml
func (c *catalog) saveModelsToProviders(basePath string) error {
	// For each author, if they have a catalog (provider relationship), save their models to that provider
	authorsMap := c.Authors().Map()

	for _, author := range authorsMap {
		if author.Catalog != nil {
			providerID := string(author.Catalog.ProviderID)
			providerDir := filepath.Join(basePath, providersPath, providerID)
			if err := os.MkdirAll(providerDir, dirMode); err != nil {
				return fmt.Errorf("creating provider directory %s: %w", providerDir, err)
			}

			// Get models for this author and save them to the provider directory
			for modelID, model := range author.Models {
				modelFileName := modelID + yamlExtension
				modelFile := filepath.Join(providerDir, modelFileName)
				data, err := yaml.Marshal(model)
				if err != nil {
					return fmt.Errorf("marshaling model %s: %w", modelID, err)
				}

				if err := os.WriteFile(modelFile, data, fileMode); err != nil {
					return fmt.Errorf("writing model file %s: %w", modelFile, err)
				}
			}
		}
	}

	return nil
}
