package starmap

import (
	"fmt"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

var (
	defaultConfig = &config{
		autoUpdatesEnabled: true,          // Default to auto-updates enabled
		autoUpdateInterval: 1 * time.Hour, // Default to hourly updates
		autoUpdateFunc:     nil,           // Default to no auto-update function
		initialCatalog:     nil,           // Default to no initial catalog
		remoteServerURL:    nil,           // Default to no remote server
		remoteServerAPIKey: nil,           // Default to no remote server API key
		remoteServerOnly:   false,         // Default to not only use remote server
	}
)

// config holds the configuration for a Starmap instance
type config struct {
	// Remote server configuration
	remoteServerURL    *string
	remoteServerAPIKey *string
	remoteServerOnly   bool // If true (enabled), don't use any other sources for catalog updates including provider APIs

	// Update configuration
	autoUpdatesEnabled bool
	autoUpdateInterval time.Duration
	autoUpdateFunc     AutoUpdateFunc

	// Initial catalog
	initialCatalog *catalogs.Catalog
}

// Option is a function that configures a Starmap instance
type Option func(*config) error

// options applies the given options to the config
func (s *starmap) options(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(s.config); err != nil {
			return fmt.Errorf("applying option: %w", err)
		}
	}
	return nil
}

// WithRemoteServer configures the remote server for catalog updates.
// A url is required, an api key can be provided for authentication,
// otherwise use nil to skip Bearer token authentication.
func WithRemoteServer(url string, apiKey *string) Option {
	return func(c *config) error {
		c.remoteServerURL = &url
		c.remoteServerAPIKey = apiKey
		return nil
	}
}

// WithRemoteServerOnly configures whether to only use the remote server and not hit provider APIs
func WithRemoteServerOnly(enabled bool) Option {
	return func(c *config) error {
		c.remoteServerOnly = enabled
		return nil
	}
}

// WithAutoUpdates configures whether automatic updates are enabled
func WithAutoUpdates(enabled bool) Option {
	return func(c *config) error {
		c.autoUpdatesEnabled = enabled
		return nil
	}
}

// WithAutoUpdateInterval configures how often to automatically update the catalog
func WithAutoUpdateInterval(interval time.Duration) Option {
	return func(c *config) error {
		c.autoUpdateInterval = interval
		return nil
	}
}

// AutoUpdateFunc is a function that updates the catalog
type AutoUpdateFunc func(catalogs.Catalog) (catalogs.Catalog, error)

// WithAutoUpdateFunc configures a custom function for updating the catalog
func WithAutoUpdateFunc(fn AutoUpdateFunc) Option {
	return func(c *config) error {
		c.autoUpdateFunc = fn
		return nil
	}
}

// WithInitialCatalog configures the initial catalog to use
func WithInitialCatalog(catalog catalogs.Catalog) Option {
	return func(c *config) error {
		c.initialCatalog = &catalog
		return nil
	}
}
