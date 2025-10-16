package catalogs

import (
	"io/fs"
	"os"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/goccy/go-yaml"
)

// Load loads the catalog from the configured filesystem.
func (cat *catalog) Load() error {
	if cat.options.readFS == nil {
		return nil // Memory catalog - nothing to load
	}

	// Load providers.yaml
	if err := cat.loadProvidersYAML(); err != nil {
		return err
	}

	// Load authors.yaml
	if err := cat.loadAuthorsYAML(); err != nil {
		return err
	}

	// Load model files from providers/
	if err := cat.loadModelFiles(); err != nil {
		return err
	}

	// Post-process: Populate author.Models from model.Authors field
	if err := cat.populateAuthorModels(); err != nil {
		return err
	}

	return nil
}

// loadProvidersYAML loads providers from providers.yaml file.
func (cat *catalog) loadProvidersYAML() error {
	data, err := fs.ReadFile(cat.options.readFS, "providers.yaml")
	if err != nil {
		return nil // File doesn't exist is okay
	}

	var providers []Provider
	if err := yaml.Unmarshal(data, &providers); err != nil {
		return errors.WrapParse("yaml", "providers.yaml", err)
	}

	for _, p := range providers {
		_ = cat.SetProvider(p)
	}
	return nil
}

// loadAuthorsYAML loads authors from authors.yaml file.
func (cat *catalog) loadAuthorsYAML() error {
	data, err := fs.ReadFile(cat.options.readFS, "authors.yaml")
	if err != nil {
		return nil // File doesn't exist is okay
	}

	var authors []Author
	if err := yaml.Unmarshal(data, &authors); err != nil {
		return errors.WrapParse("yaml", "authors.yaml", err)
	}

	for _, a := range authors {
		_ = cat.SetAuthor(a)
	}
	return nil
}

// loadProviderModel loads a model into a provider's Models map.
func (cat *catalog) loadProviderModel(pathParts []string, model *Model) error {
	if len(pathParts) < 4 || pathParts[0] != "providers" || pathParts[2] != "models" {
		return nil // Not a provider model path
	}

	providerID := ProviderID(pathParts[1])
	provider, err := cat.Provider(providerID)
	if err != nil {
		return nil // Provider doesn't exist, skip
	}

	if provider.Models == nil {
		provider.Models = make(map[string]*Model)
	}
	provider.Models[model.ID] = model
	return cat.SetProvider(provider)
}

// loadAuthorModel loads a model into an author's Models map.
func (cat *catalog) loadAuthorModel(pathParts []string, model *Model) error {
	if len(pathParts) < 4 || pathParts[0] != "authors" || pathParts[2] != "models" {
		return nil // Not an author model path
	}

	authorID := AuthorID(pathParts[1])
	author, err := cat.Author(authorID)
	if err != nil {
		return nil // Author doesn't exist, skip
	}

	if author.Models == nil {
		author.Models = make(map[string]*Model)
	}
	author.Models[model.ID] = model
	return cat.SetAuthor(author)
}

// loadModelFile parses and loads a model file.
func (cat *catalog) loadModelFile(path string, data []byte) error {
	var model Model
	if err := yaml.Unmarshal(data, &model); err != nil {
		return nil // Skip invalid YAML
	}

	pathParts := strings.Split(path, "/")

	// Handle providers/[provider-id]/models/[model].yaml
	if err := cat.loadProviderModel(pathParts, &model); err != nil {
		return err
	}

	// Handle authors/[author-id]/models/[model].yaml
	if err := cat.loadAuthorModel(pathParts, &model); err != nil {
		return err
	}

	return nil
}

// loadModelFiles walks the providers directory and loads all model files.
func (cat *catalog) loadModelFiles() error {
	err := fs.WalkDir(cat.options.readFS, "providers", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := fs.ReadFile(cat.options.readFS, path)
		if err != nil {
			return nil // Skip files we can't read
		}

		return cat.loadModelFile(path, data)
	})

	if err != nil && !os.IsNotExist(err) {
		return errors.WrapIO("walk", "providers directory", err)
	}
	return nil
}

// addModelToAuthor adds a model to an author's Models map.
func (cat *catalog) addModelToAuthor(authorID AuthorID, model *Model) error {
	author, err := cat.Author(authorID)
	if err != nil {
		return err
	}

	if author.Models == nil {
		author.Models = make(map[string]*Model)
	}
	author.Models[model.ID] = model
	return cat.SetAuthor(author)
}

// populateAuthorModels populates author.Models from model.Authors field.
// This ensures bidirectional relationship between models and authors.
func (cat *catalog) populateAuthorModels() error {
	providers := cat.providers.List()

	for _, provider := range providers {
		if provider.Models == nil {
			continue
		}

		for _, model := range provider.Models {
			if len(model.Authors) == 0 {
				continue
			}

			for _, modelAuthor := range model.Authors {
				if err := cat.addModelToAuthor(modelAuthor.ID, model); err != nil {
					// Continue on error - not fatal
					continue
				}
			}
		}
	}
	return nil
}
