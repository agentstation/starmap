package catalogs

import (
	stderrors "errors"
	"io/fs"
	"os"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// LoadIssue describes one malformed model file quarantined during a catalog load.
type LoadIssue struct {
	// Path identifies the model file relative to the catalog root.
	Path string
	// Err is the typed parse or validation failure.
	Err error
	// Limit reports that the collection budget, rather than record syntax,
	// caused the quarantine.
	Limit bool
}

// LoadReport describes bounded model-file loading. Structural catalog files
// remain fail-closed and are not represented here.
type LoadReport struct {
	// Accepted is the number of model files loaded successfully.
	Accepted int
	// Rejected includes malformed and excess model files.
	Rejected int
	// Issues contains bounded typed diagnostics.
	Issues []LoadIssue
	// Truncated reports that excess model files were not read.
	Truncated bool
}

// Err joins quarantined record failures for callers that require a fully valid
// catalog, such as embedded bootstrap and atomic projection validation.
func (r LoadReport) Err() error {
	if len(r.Issues) == 0 {
		return nil
	}
	errs := make([]error, 0, len(r.Issues))
	for _, issue := range r.Issues {
		if issue.Err != nil {
			errs = append(errs, issue.Err)
		}
	}
	return stderrors.Join(errs...)
}

// LoadReport returns a caller-owned copy of the builder's load diagnostics.
func (cat *Builder) LoadReport() LoadReport {
	report := cat.loadReport
	report.Issues = append([]LoadIssue(nil), report.Issues...)
	return report
}

// Load loads the catalog from the configured filesystem.
func (cat *Builder) Load() error {
	if cat.config.readFilesystem() == nil {
		return nil // Memory catalog - nothing to load
	}
	cat.loadReport = LoadReport{}

	// Load providers.yaml
	if err := cat.loadProvidersYAML(); err != nil {
		return err
	}

	// Load authors.yaml
	if err := cat.loadAuthorsYAML(); err != nil {
		return err
	}

	// Load provenance.yaml
	if err := cat.loadProvenanceYAML(); err != nil {
		return err
	}

	// Load model files from providers/
	if err := cat.loadProviderModelFiles(); err != nil {
		return err
	}

	// Load provider-independent model files from authors/.
	if err := cat.loadAuthorModelFiles(); err != nil {
		return err
	}

	return nil
}

// loadProvidersYAML loads providers from providers.yaml file.
func (cat *Builder) loadProvidersYAML() error {
	data, err := fs.ReadFile(cat.config.readFilesystem(), "providers.yaml")
	if err != nil {
		if stderrors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return errors.WrapIO("read", "providers.yaml", err)
	}

	var providers []Provider
	if err := yaml.UnmarshalWithOptions(data, &providers, yaml.Strict()); err != nil {
		return errors.WrapParse("yaml", "providers.yaml", err)
	}

	for _, p := range providers {
		if err := cat.SetProvider(p); err != nil {
			return errors.WrapResource("load", "provider", string(p.ID), err)
		}
	}
	return nil
}

// loadAuthorsYAML loads authors from authors.yaml file.
func (cat *Builder) loadAuthorsYAML() error {
	data, err := fs.ReadFile(cat.config.readFilesystem(), "authors.yaml")
	if err != nil {
		if stderrors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return errors.WrapIO("read", "authors.yaml", err)
	}

	var authors []Author
	if err := yaml.Unmarshal(data, &authors); err != nil {
		return errors.WrapParse("yaml", "authors.yaml", err)
	}

	for _, a := range authors {
		if err := cat.SetAuthor(a); err != nil {
			return errors.WrapResource("load", "author", string(a.ID), err)
		}
	}
	return nil
}

// loadProvenanceYAML loads provenance from provenance.yaml file.
func (cat *Builder) loadProvenanceYAML() error {
	data, err := fs.ReadFile(cat.config.readFilesystem(), "provenance.yaml")
	if err != nil {
		if stderrors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return errors.WrapIO("read", "provenance.yaml", err)
	}

	var pf provenance.File
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return errors.WrapParse("yaml", "provenance.yaml", err)
	}

	cat.provenance.Set(pf.Provenance)
	return nil
}

// loadProviderModel loads a model into a provider's Models map.
func (cat *Builder) loadProviderModel(pathParts []string, model *Model) error {
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

// loadAuthorModel loads one provider-independent model under its owning author.
func (cat *Builder) loadAuthorModel(pathParts []string, model *Model) error {
	if len(pathParts) != 4 || pathParts[0] != "authors" || pathParts[2] != "models" {
		return nil
	}
	return cat.SetAuthorModel(AuthorID(pathParts[1]), *model)
}

// loadModelFile parses and loads a model file.
func (cat *Builder) loadModelFile(path string, data []byte) error {
	var model Model
	if err := yaml.Unmarshal(data, &model); err != nil {
		return errors.WrapParse("yaml", path, err)
	}

	pathParts := strings.Split(path, "/")

	switch pathParts[0] {
	case "providers":
		return cat.loadProviderModel(pathParts, &model)
	case "authors":
		return cat.loadAuthorModel(pathParts, &model)
	default:
		return nil
	}
}

func (cat *Builder) loadModelRecord(path string, data []byte) {
	if err := cat.loadModelFile(path, data); err != nil {
		cat.loadReport.Rejected++
		cat.loadReport.Issues = append(cat.loadReport.Issues, LoadIssue{Path: path, Err: err})
		return
	}
	cat.loadReport.Accepted++
}

func (cat *Builder) modelLoadLimitReached(path string) bool {
	if cat.loadReport.Accepted+cat.loadReport.Rejected < constants.MaxCatalogModels {
		return false
	}
	if !cat.loadReport.Truncated {
		cat.loadReport.Truncated = true
		cat.loadReport.Rejected++
		cat.loadReport.Issues = append(cat.loadReport.Issues, LoadIssue{
			Path:  path,
			Limit: true,
			Err: &errors.ValidationError{
				Field: "catalog.models", Value: constants.MaxCatalogModels,
				Message: "model file count exceeds maximum; excess records quarantined",
			},
		})
	}
	return true
}

// loadProviderModelFiles walks the providers directory and loads all model files.
func (cat *Builder) loadProviderModelFiles() error {
	err := fs.WalkDir(cat.config.readFilesystem(), "providers", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		if cat.modelLoadLimitReached(path) {
			return fs.SkipAll
		}

		data, err := fs.ReadFile(cat.config.readFilesystem(), path)
		if err != nil {
			return errors.WrapIO("read", path, err)
		}

		cat.loadModelRecord(path, data)
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return errors.WrapIO("walk", "providers directory", err)
	}
	return nil
}

// loadAuthorModelFiles walks the human-authored model tree.
func (cat *Builder) loadAuthorModelFiles() error {
	err := fs.WalkDir(cat.config.readFilesystem(), "authors", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") || !strings.Contains(path, "/models/") {
			return nil
		}
		if cat.modelLoadLimitReached(path) {
			return fs.SkipAll
		}

		data, err := fs.ReadFile(cat.config.readFilesystem(), path)
		if err != nil {
			return errors.WrapIO("read", path, err)
		}
		cat.loadModelRecord(path, data)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return errors.WrapIO("walk", "authors directory", err)
	}
	return nil
}
