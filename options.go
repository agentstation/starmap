package starmap

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/sources"
)

// ============================================================================
// Starmap Options
// ============================================================================

// options holds the configuration for a Starmap instance.
type options struct {
	// Remote server configuration
	remoteServerURL    *string
	remoteServerAPIKey *string
	remoteServerOnly   bool // If true (enabled), don't use any other sources for catalog updates including provider APIs

	// Update configuration
	autoUpdatesEnabled bool
	autoUpdateInterval time.Duration
	autoUpdateFunc     AutoUpdateFunc

	// local catalog path
	localPath string

	// embedded catalog
	embeddedCatalogEnabled bool

	sources []sources.ID // Configured sources for syncing
}

func defaults() *options {
	return &options{
		autoUpdatesEnabled:     true,                            // Default to auto-updates enabled
		autoUpdateInterval:     constants.DefaultUpdateInterval, // Default to hourly updates
		autoUpdateFunc:         nil,                             // Default to no auto-update function
		localPath:              "",                              // Default to no local path
		embeddedCatalogEnabled: false,                           // Default to no embedded catalog
		sources:                []sources.ID{},                  // Default to no sources
		remoteServerURL:        nil,                             // Default to no remote server
		remoteServerAPIKey:     nil,                             // Default to no remote server API key
		remoteServerOnly:       false,                           // Default to not only use remote server
	}
}

// Option is a function that configures a Starmap instance.
type Option func(*options) error

// apply applies the given options to the options.
func (o *options) apply(opts ...Option) *options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithRemoteServer configures the remote server for catalog updates.
// A url is required, an api key can be provided for authentication,
// otherwise use nil to skip Bearer token authentication.
func WithRemoteServer(url string, apiKey *string) Option {
	return func(o *options) error {
		o.remoteServerURL = &url
		o.remoteServerAPIKey = apiKey
		return nil
	}
}

// WithRemoteServerOnly configures whether to only use the remote server and not hit provider APIs.
func WithRemoteServerOnly() Option {
	return func(o *options) error {
		o.remoteServerOnly = true
		return nil
	}
}

// WithAutoUpdatesDisabled configures whether automatic updates are disabled.
func WithAutoUpdatesDisabled() Option {
	return func(o *options) error {
		o.autoUpdatesEnabled = false
		return nil
	}
}

// WithAutoUpdateInterval configures how often to automatically update the catalog.
func WithAutoUpdateInterval(interval time.Duration) Option {
	return func(o *options) error {
		o.autoUpdateInterval = interval
		return nil
	}
}

// AutoUpdateFunc is a function that updates the catalog.
type AutoUpdateFunc func(catalogs.Catalog) (catalogs.Catalog, error)

// WithAutoUpdateFunc configures a custom function for updating the catalog.
func WithAutoUpdateFunc(fn AutoUpdateFunc) Option {
	return func(o *options) error {
		o.autoUpdateFunc = fn
		return nil
	}
}

// // WithInitialCatalog configures the initial catalog to use.
// func WithInitialCatalog(catalog catalogs.Catalog) Option {
// 	return func(o *options) error {
// 		o.initialCatalog = &catalog
// 		return nil
// 	}
// }

// WithLocalPath configures the local source to use a specific catalog path.
func WithLocalPath(path string) Option {
	return func(o *options) error {
		o.localPath = path
		return nil
	}
}

// WithEmbeddedCatalog configures whether to use an embedded catalog.
// It defaults to false, but takes precedence over WithLocalPath if set.
func WithEmbeddedCatalog() Option {
	return func(o *options) error {
		o.embeddedCatalogEnabled = true
		return nil
	}
}
