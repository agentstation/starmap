package files

import (
	"fmt"

	"github.com/agentstation/starmap/internal/catalogs/files"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Option is a function that configures a FilesCatalog
type Option func(*config) error

// WithAutoLoad configures the FilesCatalog to auto-load
func WithAutoLoad(enabled bool) Option {
	return func(cfg *config) error {
		cfg.autoLoad = enabled
		return nil
	}
}

// WithReadOnly configures the FilesCatalog to be read-only
func WithReadOnly(readOnly bool) Option {
	return func(cfg *config) error {
		cfg.readOnly = readOnly
		return nil
	}
}

// WithNoAutoLoad configures the FilesCatalog to not auto-load
func WithNoAutoLoad() Option {
	return WithAutoLoad(false)
}

// New creates a file-based catalog from a directory
// path is the path to the directory containing the catalog files
// opts are the options for the FilesCatalog
func New(path string, opts ...Option) (catalogs.Catalog, error) {
	if path == "" {
		return nil, fmt.Errorf("path is required for files catalog")
	}

	cfg := &config{
		path:     path,
		autoLoad: true,
		readOnly: false,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("applying files option: %w", err)
		}
	}

	catalog := files.NewCatalog(path)

	if cfg.autoLoad {
		if err := catalog.Load(); err != nil {
			return nil, fmt.Errorf("loading files catalog from %s: %w", path, err)
		}
	}

	return catalog, nil
}

// config is the configuration for a FilesCatalog
type config struct {
	path     string
	autoLoad bool
	readOnly bool
}
