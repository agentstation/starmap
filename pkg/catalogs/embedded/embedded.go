package embedded

import (
	"fmt"

	"github.com/agentstation/starmap/internal/catalogs/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Option is a function that configures a EmbeddedCatalog
type Option func(*config) error

// WithAutoLoad configures the EmbeddedCatalog to auto-load
func WithAutoLoad(enabled bool) Option {
	return func(cfg *config) error {
		cfg.autoLoad = enabled
		return nil
	}
}

// WithEmbeddedNoAutoLoad configures the EmbeddedCatalog to not auto-load
func WithNoAutoLoad() Option {
	return WithAutoLoad(false)
}

// New creates an embedded catalog with compiled-in YAML files
func New(opts ...Option) (catalogs.Catalog, error) {
	cfg := &config{
		autoLoad: true,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("applying embedded option: %w", err)
		}
	}

	catalog := embedded.NewCatalog()

	if cfg.autoLoad {
		if err := catalog.Load(); err != nil {
			return nil, fmt.Errorf("loading embedded catalog: %w", err)
		}
	}

	return catalog, nil
}

// config is the configuration for a EmbeddedCatalog
type config struct {
	autoLoad bool // If true, the catalog will be loaded automatically
}
