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
func (cat *Builder) Save(opts ...save.Option) error {

	// Apply the options
	options := save.Defaults().Apply(opts...)

	writePath := cat.config.resolveWritePath(options.Path())
	if writePath == "" {
		return &errors.ConfigError{
			Component: catalogResourceCatalog,
			Message:   "no write path configured for saving",
		}
	}

	// Save to the configured path
	return cat.saveTo(writePath)
}

// saveTo saves the catalog to the specified path.
func (cat *Builder) saveTo(basePath string) error {
	// A save is a replacement of Starmap-managed records. Remove the prior
	// managed indexes/model trees first so deleted records cannot survive and be
	// loaded into the next catalog. Transactional callers should publish through
	// pkg/catalogstore, which preserves the previous generation on failure.
	if err := removeManagedCatalogData(basePath); err != nil {
		return err
	}

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
		yamlData, err := cat.providers.EncodeYAML()
		if err != nil {
			return err
		}
		if err := writeFile("providers.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "providers.yaml", err)
		}
	}

	// Save authors.yaml
	authors := cat.authors.List()
	if len(authors) > 0 {
		// Use FormatYAML for nicely formatted output with comments and sections
		yamlData, err := cat.authors.EncodeYAML()
		if err != nil {
			return err
		}
		if err := writeFile("authors.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "authors.yaml", err)
		}
	}

	// Save provenance.yaml
	if cat.provenance.Len() > 0 {
		yamlData, err := cat.provenance.EncodeYAML()
		if err != nil {
			return err
		}
		if err := writeFile("provenance.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "provenance.yaml", err)
		}
	}

	// Save model files to providers/<provider>/models/<model>.yaml or providers/<provider>/models/<org>/<model>.yaml
	for _, provider := range cat.providers.List() {
		if len(provider.Models) > 0 {
			// Debug: log provider with models
			logging.Debug().
				Str(catalogResourceProvider, string(provider.ID)).
				Int("model_count", len(provider.Models)).
				Msg("Saving provider models")

			for _, model := range provider.Models {
				var modelPath string
				if strings.Contains(model.ID, "/") {
					// Hierarchical ID like "meta-llama/llama-3" -> providers/groq/models/meta-llama/llama-3.yaml
					modelPath = filepath.Join("providers", string(provider.ID), catalogPathModels, model.ID+".yaml")
				} else {
					// Simple ID like "gpt-4" -> providers/openai/models/gpt-4.yaml
					modelPath = filepath.Join("providers", string(provider.ID), catalogPathModels, model.ID+".yaml")
				}

				// Use FormatYAML for nicely formatted output with comments
				formatted, err := model.EncodeYAML()
				if err != nil {
					return err
				}
				data := []byte(formatted)
				if err := writeFile(modelPath, data); err != nil {
					return errors.WrapIO("write", "model "+model.ID, err)
				}
			}
		}
	}

	// Save author models under authors/<author>/models/<model>.yaml
	// These are denormalized views - only save non-hierarchical model IDs
	for _, author := range cat.authors.List() {
		if author.Models == nil {
			continue
		}

		for _, model := range author.Models {
			// Skip hierarchical models (contain "/" in ID)
			// These are provider-specific (e.g., "meta-llama/llama-3" from Groq)
			// and should only exist in provider catalogs
			if strings.Contains(model.ID, "/") {
				logging.Debug().
					Str("model_id", model.ID).
					Str(catalogResourceAuthor, string(author.ID)).
					Msg("Skipping hierarchical model for author save")
				continue
			}

			// Simple ID -> authors/<author>/models/<model>.yaml
			modelPath := filepath.Join("authors", string(author.ID), catalogPathModels, model.ID+".yaml")

			// Use FormatYAML for nicely formatted output with comments
			formatted, err := model.EncodeYAML()
			if err != nil {
				return err
			}
			data := []byte(formatted)
			if err := writeFile(modelPath, data); err != nil {
				return errors.WrapIO("write", "model "+model.ID, err)
			}
		}
	}

	return nil
}

func removeManagedCatalogData(basePath string) error {
	for _, filename := range []string{"providers.yaml", "authors.yaml", "provenance.yaml"} {
		path := filepath.Join(basePath, filename)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return errors.WrapIO("remove", path, err)
		}
	}

	for _, collection := range []string{"providers", "authors"} {
		root := filepath.Join(basePath, collection)
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return errors.WrapIO("read", root, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			modelsPath := filepath.Join(root, entry.Name(), catalogPathModels)
			if err := os.RemoveAll(modelsPath); err != nil {
				return errors.WrapIO("remove", modelsPath, err)
			}
		}
	}
	return nil
}
