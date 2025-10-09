package catalogs

import (
	"io/fs"
	"os"

	"github.com/agentstation/starmap/internal/embedded"
)

// catalogOptions is a struct that contains the options for the catalog.
type catalogOptions struct {
	readFS        fs.FS  // For reading catalog files
	writePath     string // For writing catalog files (optional)
	mergeStrategy MergeStrategy
}

// apply applies the given options to the catalog options.
func (c *catalogOptions) apply(opts ...Option) *catalogOptions {
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// catalogDefaults returns the default options for a catalog.
func catalogDefaults() *catalogOptions {
	return &catalogOptions{
		readFS:        nil,
		writePath:     "",
		mergeStrategy: MergeEnrichEmpty,
	}
}

// Option configures a catalog.
type Option func(*catalogOptions)

// WithFS configures the catalog to use a custom fs.FS for reading.
func WithFS(fsys fs.FS) Option {
	return func(c *catalogOptions) {
		c.readFS = fsys
	}
}

// WithPath configures the catalog to use a directory path for reading
// This creates an os.DirFS under the hood.
func WithPath(path string) Option {
	return func(c *catalogOptions) {
		c.readFS = os.DirFS(path)
		c.writePath = path // Also set as write path
	}
}

// WithEmbedded configures the catalog to use embedded files.
func WithEmbedded() Option {
	return func(c *catalogOptions) {
		// Use fs.Sub to get the catalog subdirectory
		catalogFS, err := fs.Sub(embedded.FS, "catalog")
		if err != nil {
			// Fall back to using the full embedded FS
			c.readFS = embedded.FS
		} else {
			c.readFS = catalogFS
		}
		c.writePath = "internal/embedded/catalog"
	}
}

// WithWritePath sets a specific path for writing catalog files.
func WithWritePath(path string) Option {
	return func(c *catalogOptions) {
		c.writePath = path
	}
}

// WithMergeStrategy sets the default merge strategy.
func WithMergeStrategy(strategy MergeStrategy) Option {
	return func(c *catalogOptions) {
		c.mergeStrategy = strategy
	}
}
