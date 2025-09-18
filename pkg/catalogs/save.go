package catalogs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/save"
)

// Save saves the catalog to the configured filesystem.
func (cat *catalog) Save(opts ...save.Option) error {

	// Apply the options
	options := save.Defaults().Apply(opts...)

	// Check if a write path or options path is configured
	if cat.options.writePath == "" && options.Path() == "" {
		return &errors.ConfigError{
			Component: "catalog",
			Message:   "no write path configured for saving",
		}
	}

	// If the options path is configured, use it
	if cat.options.writePath == "" && options.Path() != "" {
		cat.options.writePath = options.Path()
	}

	// Save to the configured path
	return cat.saveTo(cat.options.writePath)
}

// saveTo saves the catalog to the specified path.
func (cat *catalog) saveTo(basePath string) error {
	// Helper function to write a file
	writeFile := func(path string, data []byte) error {
		fullPath := filepath.Join(basePath, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
			return errors.WrapIO("create", dir, err)
		}
		return os.WriteFile(fullPath, data, constants.FilePermissions)
	}

	// Save providers.yaml
	providers := cat.providers.List()
	if len(providers) > 0 {
		// Use FormatYAML if available
		yamlData := cat.providers.FormatYAML()
		if err := writeFile("providers.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "providers.yaml", err)
		}
	}

	// Save authors.yaml
	authors := cat.authors.List()
	if len(authors) > 0 {
		// Use FormatYAML for nicely formatted output with comments and sections
		yamlData := cat.authors.FormatYAML()
		if err := writeFile("authors.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "authors.yaml", err)
		}
	}

	// Save model files to providers/<provider>/models/<model>.yaml or providers/<provider>/models/<org>/<model>.yaml
	for _, provider := range cat.providers.List() {
		if len(provider.Models) > 0 {
			// Debug: log provider with models
			logging.Debug().
				Str("provider", string(provider.ID)).
				Int("model_count", len(provider.Models)).
				Msg("Saving provider models")

			for _, model := range provider.Models {
				var modelPath string
				if strings.Contains(model.ID, "/") {
					// Hierarchical ID like "meta-llama/llama-3" -> providers/groq/models/meta-llama/llama-3.yaml
					modelPath = filepath.Join("providers", string(provider.ID), "models", model.ID+".yaml")
				} else {
					// Simple ID like "gpt-4" -> providers/openai/models/gpt-4.yaml
					modelPath = filepath.Join("providers", string(provider.ID), "models", model.ID+".yaml")
				}

				// Use FormatYAML for nicely formatted output with comments
				data := []byte(model.FormatYAML())
				if err := writeFile(modelPath, data); err != nil {
					return errors.WrapIO("write", "model "+model.ID, err)
				}
			}
		}
	}

	// Save author models under authors/<author>/models/<model>.yaml
	for _, author := range cat.authors.List() {
		if author.Models != nil {
			for _, model := range author.Models {
				var modelPath string
				if strings.Contains(model.ID, "/") {
					// Hierarchical ID -> authors/meta/models/meta-llama/llama-3.yaml
					modelPath = filepath.Join("authors", string(author.ID), "models", model.ID+".yaml")
				} else {
					// Simple ID -> authors/openai/models/gpt-4.yaml
					modelPath = filepath.Join("authors", string(author.ID), "models", model.ID+".yaml")
				}

				// Use FormatYAML for nicely formatted output with comments
				data := []byte(model.FormatYAML())
				if err := writeFile(modelPath, data); err != nil {
					return errors.WrapIO("write", "model "+model.ID, err)
				}
			}
		}
	}

	return nil
}
