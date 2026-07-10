package starmap

import (
	"context"
	"reflect"
	"time"

	"github.com/agentstation/starmap/internal/utils/ptr"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
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

	// Explicit update injection; cadence belongs to the deployment layer.
	updateFunc UpdateFunc

	// local catalog path
	localPath string

	// durable generation store required by every non-dry mutation path
	catalogStore catalogstore.Store

	// embedded catalog
	embeddedCatalogEnabled        bool
	embeddedBootstrapMaxAge       time.Duration
	embeddedBootstrapMaxSizeBytes int64
}

func defaults() *options {
	return &options{
		updateFunc:                    nil,   // Default to pipeline-based updates
		localPath:                     "",    // Default to no local path
		catalogStore:                  nil,   // Mutation requires an explicit writable store
		embeddedCatalogEnabled:        false, // Default to no embedded catalog
		embeddedBootstrapMaxAge:       0,     // Disabled until explicitly configured
		embeddedBootstrapMaxSizeBytes: 0,     // Disabled until explicitly configured
		remoteServerURL:               nil,   // Default to no remote server
		remoteServerAPIKey:            nil,   // Default to no remote server API key
		remoteServerOnly:              false, // Default to not only use remote server
	}
}

// WithCatalogStore configures the writable generation store used by non-dry
// sync, manual, remote, and scheduled catalog updates. Read-only access and dry
// runs do not require a store.
func WithCatalogStore(store catalogstore.Store) Option {
	return func(o *options) error {
		if isNilCatalogStore(store) {
			return &errors.ConfigError{
				Component: "catalog store",
				Message:   "writable store is required",
			}
		}
		o.catalogStore = store
		return nil
	}
}

func isNilCatalogStore(store catalogstore.Store) bool {
	if store == nil {
		return true
	}
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// Option is a function that configures a Starmap instance.
type Option func(*options) error

// apply applies the given options to the options.
func (o *options) apply(opts ...Option) (*options, error) {
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

// WithRemoteServerURL configures a versioned remote API base URL, for example
// https://starmap.example.com/api/v1, without changing the update source. Use
// WithRemoteServerOnly to make Client.Update fetch exclusively from that server.
func WithRemoteServerURL(url string) Option {
	return func(o *options) error {
		o.remoteServerURL = ptr.String(url)
		return nil
	}
}

// WithRemoteServerAPIKey configures the remote server API key.
func WithRemoteServerAPIKey(apiKey string) Option {
	return func(o *options) error {
		o.remoteServerAPIKey = ptr.String(apiKey)
		return nil
	}
}

// WithRemoteServerOnly configures Client.Update to use only the versioned remote
// manifest and immutable generation snapshot contract at url.
func WithRemoteServerOnly(url string) Option {
	return func(o *options) error {
		o.remoteServerOnly = true
		o.remoteServerURL = ptr.String(url)
		return nil
	}
}

// UpdateFunc builds an explicit candidate catalog and must honor cancellation.
// Scheduling, retry, and high-availability ownership remain above Client.
type UpdateFunc func(context.Context, *catalogs.Builder) (*catalogs.Builder, error)

// WithUpdateFunc configures an explicit context-aware update implementation.
func WithUpdateFunc(fn UpdateFunc) Option {
	return func(o *options) error {
		o.updateFunc = fn
		return nil
	}
}

// // WithInitialCatalog configures the initial catalog to use.
// func WithInitialCatalog(catalog *catalogs.Builder) Option {
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

// WithEmbeddedBootstrapMaxAge fails readiness while the active catalog is the
// embedded bootstrap and its generation age exceeds maxAge.
func WithEmbeddedBootstrapMaxAge(maxAge time.Duration) Option {
	return func(o *options) error {
		if maxAge <= 0 {
			return &errors.ValidationError{Field: "embeddedBootstrapMaxAge", Value: maxAge, Message: "must be positive"}
		}
		o.embeddedBootstrapMaxAge = maxAge
		return nil
	}
}

// WithEmbeddedBootstrapMaxSizeBytes fails readiness while the active embedded
// bootstrap canonical payload exceeds maxSizeBytes.
func WithEmbeddedBootstrapMaxSizeBytes(maxSizeBytes int64) Option {
	return func(o *options) error {
		if maxSizeBytes <= 0 {
			return &errors.ValidationError{Field: "embeddedBootstrapMaxSizeBytes", Value: maxSizeBytes, Message: "must be positive"}
		}
		o.embeddedBootstrapMaxSizeBytes = maxSizeBytes
		return nil
	}
}
