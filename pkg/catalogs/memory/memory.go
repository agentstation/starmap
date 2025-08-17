package memory

import (
	"fmt"

	"github.com/agentstation/starmap/internal/catalogs/memory"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Option is a function that configures a MemoryCatalog
type Option func(*config) error

// WithReadOnly configures the MemoryCatalog to be read-only
func WithReadOnly(readOnly bool) Option {
	return func(cfg *config) error {
		cfg.readOnly = readOnly
		return nil
	}
}

// WithPreload configures the MemoryCatalog to preload data
func WithPreload(data []byte) Option {
	return func(cfg *config) error {
		if len(data) == 0 {
			return fmt.Errorf("preload data cannot be empty")
		}
		cfg.preloadData = make([]byte, len(data))
		copy(cfg.preloadData, data)
		return nil
	}
}

// New creates an in-memory catalog
func New(opts ...Option) (catalogs.Catalog, error) {
	cfg := &config{
		readOnly: false,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("applying memory option: %w", err)
		}
	}

	catalog := memory.NewCatalogWithConfig(
		cfg.readOnly,
		cfg.preloadData,
	)
	return catalog, nil
}

// config is the configuration for a MemoryCatalog
type config struct {
	readOnly    bool
	preloadData []byte
}
