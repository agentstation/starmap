package starmap

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Option is a function that configures a Starmap instance
type Option func(*config) error

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
