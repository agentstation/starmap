package starmap

import (
	"reflect"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// ============================================================================
// Starmap Options
// ============================================================================

// options holds the configuration for a Starmap instance.
type options struct {
	// optional human-editable provider YAML workspace
	catalogPath string

	// durable generation store required by every non-dry mutation path
	catalogStore storage.Store

	// embedded bootstrap policy
	embeddedBootstrapMaxAge       time.Duration
	embeddedBootstrapMaxSizeBytes int64
}

func defaults() *options {
	return &options{
		catalogPath:                   "",  // Default to no filesystem workspace
		catalogStore:                  nil, // Mutation requires an explicit writable store
		embeddedBootstrapMaxAge:       0,   // Disabled until explicitly configured
		embeddedBootstrapMaxSizeBytes: 0,   // Disabled until explicitly configured
	}
}

// WithCatalogStore configures the writable generation store used by non-dry
// sync, manual, remote, and scheduled catalog updates. Read-only access and dry
// runs do not require a store. Starmap provides memory, filesystem, and
// conditional object-storage implementations; embedding applications own and
// inject any database-backed implementation.
func WithCatalogStore(store storage.Store) Option {
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

func isNilCatalogStore(store storage.Store) bool {
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

// WithCatalogPath configures the human-editable provider YAML workspace used
// for both local observation and post-commit materialization. Immutable
// generation state remains in the separately supplied CatalogStore.
func WithCatalogPath(path string) Option {
	return func(o *options) error {
		o.catalogPath = path
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
