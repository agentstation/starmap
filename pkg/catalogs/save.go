package catalogs

import (
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Save serializes a mutable builder to its configured construction path.
// It is intentionally not a publication primitive; committed catalogs are
// materialized atomically by the Starmap client.
func (cat *Builder) Save() error {
	return cat.save(cat.config.resolveWritePath(""))
}

// SaveTo serializes a mutable builder to path.
func (cat *Builder) SaveTo(path string) error {
	return cat.save(path)
}

func (cat *Builder) save(writePath string) error {
	if writePath == "" {
		return &errors.ConfigError{
			Component: "catalog",
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
	// pkg/catalogs/storage, which preserves the previous generation on failure.
	if err := removeManagedCatalogData(basePath); err != nil {
		return err
	}

	// Helper function to write a file
	writeFile := func(path string, data []byte) error {
		fullPath := filepath.Join(basePath, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, resourcepolicy.DirMode); err != nil {
			return errors.WrapIO("create", dir, err)
		}
		return os.WriteFile(fullPath, data, resourcepolicy.FileMode)
	}

	if err := cat.saveIndexFiles(writeFile); err != nil {
		return err
	}
	if err := cat.saveProviderModels(writeFile); err != nil {
		return err
	}
	return cat.saveAuthoredModels(writeFile)
}

type catalogFileWriter func(string, []byte) error

func (cat *Builder) saveIndexFiles(writeFile catalogFileWriter) error {
	providers := cat.providers.List()
	if len(providers) > 0 {
		yamlData, err := cat.providers.EncodeYAML()
		if err != nil {
			return err
		}
		if err := writeFile("providers.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "providers.yaml", err)
		}
	}

	authors := cat.authors.List()
	if len(authors) > 0 {
		yamlData, err := cat.authors.EncodeYAML()
		if err != nil {
			return err
		}
		if err := writeFile("authors.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "authors.yaml", err)
		}
	}

	if cat.provenance.Len() > 0 {
		yamlData, err := cat.provenance.EncodeYAML()
		if err != nil {
			return err
		}
		if err := writeFile("provenance.yaml", []byte(yamlData)); err != nil {
			return errors.WrapIO("write", "provenance.yaml", err)
		}
	}
	return nil
}

func (cat *Builder) saveProviderModels(writeFile catalogFileWriter) error {
	for _, provider := range cat.providers.List() {
		if len(provider.Models) == 0 {
			continue
		}
		logging.Debug().
			Str("provider", string(provider.ID)).
			Int("model_count", len(provider.Models)).
			Msg("Saving provider models")
		for _, model := range provider.Models {
			modelPath := filepath.Join(
				"providers", string(provider.ID), "models", model.ID+".yaml",
			)
			formatted, err := model.EncodeYAML()
			if err != nil {
				return err
			}
			if err := writeFile(modelPath, []byte(formatted)); err != nil {
				return errors.WrapIO("write", "model "+model.ID, err)
			}
		}
	}
	return nil
}

func (cat *Builder) saveAuthoredModels(writeFile catalogFileWriter) error {
	// Save provider-independent model records under their owning author. Unlike
	// the removed denormalized view, these records never copy provider price,
	// limits, status, modes, or provider extensions.
	for _, record := range cat.AuthoredModels() {
		if err := validateAuthoredModel(record.AuthorID, record.Model); err != nil {
			return err
		}
		modelPath := filepath.Join(
			"authors", string(record.AuthorID), "models", record.Model.ID+".yaml",
		)
		formatted, err := record.Model.EncodeYAML()
		if err != nil {
			return err
		}
		if err := writeFile(modelPath, []byte(formatted)); err != nil {
			return errors.WrapIO("write", "authored model "+string(record.ID()), err)
		}
	}
	return nil
}

func removeManagedCatalogData(basePath string) error {
	for _, filename := range []string{
		"providers.yaml", "authors.yaml", "endpoints.yaml", "provenance.yaml",
	} {
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
			modelsPath := filepath.Join(root, entry.Name(), "models")
			if err := os.RemoveAll(modelsPath); err != nil {
				return errors.WrapIO("remove", modelsPath, err)
			}
		}
	}
	return nil
}
