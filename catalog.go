package starmap

import (
	"fmt"

	"github.com/agentstation/starmap/internal/catalogs/embedded"
	"github.com/agentstation/starmap/internal/catalogs/files"
	"github.com/agentstation/starmap/internal/catalogs/memory"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// NewEmbeddedCatalog creates an embedded catalog with compiled-in YAML files
func NewEmbeddedCatalog(opts ...EmbeddedOption) (catalogs.Catalog, error) {
	cfg := &embeddedConfig{
		AutoLoad: true,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("applying embedded option: %w", err)
		}
	}

	catalog := embedded.NewCatalog()

	if cfg.AutoLoad {
		if err := catalog.Load(); err != nil {
			return nil, fmt.Errorf("loading embedded catalog: %w", err)
		}
	}

	return catalog, nil
}

// NewFilesCatalog creates a file-based catalog from a directory
func NewFilesCatalog(path string, opts ...FilesOption) (catalogs.Catalog, error) {
	if path == "" {
		return nil, fmt.Errorf("path is required for files catalog")
	}

	cfg := &filesConfig{
		Path:     path,
		AutoLoad: true,
		ReadOnly: false,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("applying files option: %w", err)
		}
	}

	catalog := files.NewCatalog(path)

	if cfg.AutoLoad {
		if err := catalog.Load(); err != nil {
			return nil, fmt.Errorf("loading files catalog from %s: %w", path, err)
		}
	}

	return catalog, nil
}

// NewMemoryCatalog creates an in-memory catalog
func NewMemoryCatalog(opts ...MemoryOption) (catalogs.Catalog, error) {
	cfg := &memoryConfig{
		ReadOnly: false,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("applying memory option: %w", err)
		}
	}

	catalog := memory.NewCatalogWithConfig(cfg.ReadOnly, cfg.PreloadData)
	return catalog, nil
}
