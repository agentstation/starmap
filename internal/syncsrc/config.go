package syncsrc

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// Config contains all configuration needed to build sources
// This avoids circular dependencies by not importing internal packages
type Config struct {
	Catalog       catalogs.Catalog
	Provider      *catalogs.Provider
	SyncOptions   *sources.SyncOptions
	ModelsDevData *ModelsDevData // Concrete type instead of interface{}
}

// ModelsDevData holds models.dev specific data
// This is a placeholder for the actual types from internal/sources/modelsdev
type ModelsDevData struct {
	API    interface{} // Will be *modelsdev.ModelsDevAPI
	Client interface{} // Will be *modelsdev.Client
}

// DefaultConfig creates a new config with sensible defaults
func DefaultConfig(catalog catalogs.Catalog) *Config {
	return &Config{
		Catalog:     catalog,
		SyncOptions: sources.DefaultSyncOptions(),
	}
}
