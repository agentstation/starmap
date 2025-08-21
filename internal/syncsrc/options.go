package syncsrc

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// Option configures source building
type Option func(*Config)

// WithProvider sets the provider for source building
func WithProvider(provider *catalogs.Provider) Option {
	return func(cfg *Config) {
		cfg.Provider = provider
	}
}

// WithSyncOptions sets sync options for source building
func WithSyncOptions(opts *sources.SyncOptions) Option {
	return func(cfg *Config) {
		if opts != nil {
			cfg.SyncOptions = opts
		}
	}
}

// WithModelsDev is deprecated - models.dev source now self-initializes
// This option is kept for backward compatibility but does nothing
func WithModelsDev(api, client interface{}) Option {
	return func(cfg *Config) {
		// No-op - models.dev source self-initializes
	}
}
