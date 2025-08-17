package base

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/goccy/go-yaml"
)

// WalkFunc is called for each file during directory traversal
type WalkFunc func(path string, data []byte, err error) error

// FileReader abstracts file system operations for different storage backends
type FileReader interface {
	ReadFile(path string) ([]byte, error)
	WalkDir(root string, fn WalkFunc) error
}

// EmbeddedFileReader implements FileReader for embed.FS
type EmbeddedFileReader struct {
	FS embed.FS
}

func (e *EmbeddedFileReader) ReadFile(path string) ([]byte, error) {
	return e.FS.ReadFile(path)
}

func (e *EmbeddedFileReader) WalkDir(root string, fn WalkFunc) error {
	return fs.WalkDir(e.FS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Check if it's a "file does not exist" error and handle gracefully
			if strings.Contains(err.Error(), "file does not exist") {
				return fn(path, nil, err)
			}
			return err
		}

		// Skip directories and non-YAML files
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := e.FS.ReadFile(path)
		return fn(path, data, err)
	})
}

// FilesystemFileReader implements FileReader for regular filesystem
type FilesystemFileReader struct {
	BasePath string
}

func (f *FilesystemFileReader) ReadFile(path string) ([]byte, error) {
	fullPath := filepath.Join(f.BasePath, path)
	return os.ReadFile(fullPath)
}

func (f *FilesystemFileReader) WalkDir(root string, fn WalkFunc) error {
	fullRoot := filepath.Join(f.BasePath, root)

	// Check if directory exists
	if _, err := os.Stat(fullRoot); os.IsNotExist(err) {
		return fn(root, nil, err)
	}

	return filepath.Walk(fullRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-YAML files
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// Convert absolute path back to relative for consistency
		relPath, err := filepath.Rel(f.BasePath, path)
		if err != nil {
			relPath = path
		}

		data, err := os.ReadFile(path)
		return fn(relPath, data, err)
	})
}

// Loader provides common loading functionality for catalogs
type Loader struct {
	reader  FileReader
	catalog *BaseCatalog
}

// NewLoader creates a new loader with the given file reader and catalog
func NewLoader(reader FileReader, catalog *BaseCatalog) *Loader {
	return &Loader{
		reader:  reader,
		catalog: catalog,
	}
}

// LoadProviders loads providers from providers.yaml
func (l *Loader) LoadProviders() error {
	data, err := l.reader.ReadFile("providers.yaml")
	if err != nil {
		// If file doesn't exist, that's okay
		if strings.Contains(err.Error(), "file does not exist") || os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading providers.yaml: %w", err)
	}

	var providers []catalogs.Provider
	if err := yaml.Unmarshal(data, &providers); err != nil {
		return fmt.Errorf("unmarshaling providers: %w", err)
	}

	for _, provider := range providers {
		// Load API key from environment for this provider
		provider.LoadAPIKey()

		if err := l.catalog.providers.Add(&provider); err != nil {
			return fmt.Errorf("adding provider %s: %w", provider.ID, err)
		}
	}

	return nil
}

// LoadAuthors loads authors from authors.yaml
func (l *Loader) LoadAuthors() error {
	data, err := l.reader.ReadFile("authors.yaml")
	if err != nil {
		// If file doesn't exist, that's okay
		if strings.Contains(err.Error(), "file does not exist") || os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading authors.yaml: %w", err)
	}

	var authors []catalogs.Author
	if err := yaml.Unmarshal(data, &authors); err != nil {
		return fmt.Errorf("unmarshaling authors: %w", err)
	}

	for _, author := range authors {
		if err := l.catalog.authors.Add(&author); err != nil {
			return fmt.Errorf("adding author %s: %w", author.ID, err)
		}
	}

	return nil
}

// LoadEndpoints loads endpoints from endpoints.yaml
func (l *Loader) LoadEndpoints() error {
	data, err := l.reader.ReadFile("endpoints.yaml")
	if err != nil {
		// If file doesn't exist, that's okay
		if strings.Contains(err.Error(), "file does not exist") || os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading endpoints.yaml: %w", err)
	}

	// Skip if file is empty
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var endpoints []catalogs.Endpoint
	if err := yaml.Unmarshal(data, &endpoints); err != nil {
		return fmt.Errorf("unmarshaling endpoints: %w", err)
	}

	for _, endpoint := range endpoints {
		if err := l.catalog.endpoints.Add(&endpoint); err != nil {
			return fmt.Errorf("adding endpoint %s: %w", endpoint.ID, err)
		}
	}

	return nil
}

// LoadModelsFromAuthors loads models from authors/<author_id>/<model_id>.yaml files
func (l *Loader) LoadModelsFromAuthors() error {
	return l.reader.WalkDir("authors", func(path string, data []byte, err error) error {
		if err != nil {
			// If authors directory doesn't exist, that's okay
			if strings.Contains(err.Error(), "file does not exist") || os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if data == nil {
			return nil // Skip this file
		}

		model, err := ParseModel(data, path)
		if err != nil {
			return err
		}

		return l.addModelToCollections(model)
	})
}

// LoadModelsFromProviders loads models from providers/<provider_id>/**/<model_id>.yaml files
func (l *Loader) LoadModelsFromProviders() error {
	return l.reader.WalkDir("providers", func(path string, data []byte, err error) error {
		if err != nil {
			// If providers directory doesn't exist, that's okay
			if strings.Contains(err.Error(), "file does not exist") || os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if data == nil {
			return nil // Skip this file
		}

		model, err := ParseModel(data, path)
		if err != nil {
			return err
		}

		// Extract provider ID from path
		providerID := l.extractProviderID(path)
		if providerID != "" {
			// Add the model to the provider if it exists
			if providerData, ok := l.catalog.providers.Get(providerID); ok {
				if providerData.Models == nil {
					providerData.Models = make(map[string]catalogs.Model)
				}
				providerData.Models[model.ID] = *model
			}
		}

		return l.addModelToCollections(model)
	})
}

// Load loads all catalog data using the configured file reader
func (l *Loader) Load() error {
	// Load providers from providers.yaml
	if err := l.LoadProviders(); err != nil {
		return fmt.Errorf("loading providers: %w", err)
	}

	// Load authors from authors.yaml
	if err := l.LoadAuthors(); err != nil {
		return fmt.Errorf("loading authors: %w", err)
	}

	// Load endpoints from endpoints.yaml
	if err := l.LoadEndpoints(); err != nil {
		return fmt.Errorf("loading endpoints: %w", err)
	}

	// Load models from authors/<author_id>/<model_id>.yaml files
	if err := l.LoadModelsFromAuthors(); err != nil {
		return fmt.Errorf("loading models from authors: %w", err)
	}

	// Load models from providers/<provider_id>/**/<model_id>.yaml files
	if err := l.LoadModelsFromProviders(); err != nil {
		return fmt.Errorf("loading models from providers: %w", err)
	}

	return nil
}

// addModelToCollections adds the model to the catalog and cross-references
func (l *Loader) addModelToCollections(model *catalogs.Model) error {
	if model.ID == "" {
		return fmt.Errorf("model has no ID")
	}

	// Add the model to the catalog (or update if it already exists)
	if err := l.catalog.models.Add(model); err != nil {
		// If model already exists, update it instead
		if err := l.catalog.models.Set(model.ID, model); err != nil {
			return fmt.Errorf("updating existing model %s: %w", model.ID, err)
		}
	}

	// Also add the model to its authors
	for _, author := range model.Authors {
		if authorData, ok := l.catalog.authors.Get(author.ID); ok {
			if authorData.Models == nil {
				authorData.Models = make(map[string]catalogs.Model)
			}
			authorData.Models[model.ID] = *model
		}
	}

	return nil
}

// extractProviderID extracts the provider ID from a file path like "providers/<provider_id>/**/<model_id>.yaml"
func (l *Loader) extractProviderID(path string) catalogs.ProviderID {
	pathParts := strings.Split(path, "/")
	// Find the "providers" part and take the next part as provider ID
	for i, part := range pathParts {
		if part == "providers" && i+1 < len(pathParts) {
			return catalogs.ProviderID(pathParts[i+1])
		}
	}
	return ""
}
