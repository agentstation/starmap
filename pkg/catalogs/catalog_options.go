package catalogs

import (
	"io/fs"
	"os"
	"sync"

	"github.com/agentstation/starmap/internal/embedded"
)

// options is a struct that contains the options for the catalog.
type options struct {
	mu            sync.RWMutex
	readFS        fs.FS  // For reading catalog files
	writePath     string // For writing catalog files (optional)
	mergeStrategy MergeStrategy
}

func (c *options) copy() *options {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &options{
		readFS:        c.readFS,
		writePath:     c.writePath,
		mergeStrategy: c.mergeStrategy,
	}
}

func (c *options) readFilesystem() fs.FS {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.readFS
}

func (c *options) resolveWritePath(override string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writePath == "" && override != "" {
		c.writePath = override
	}
	return c.writePath
}

func (c *options) strategy() MergeStrategy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mergeStrategy
}

func (c *options) setStrategy(strategy MergeStrategy) {
	c.mu.Lock()
	c.mergeStrategy = strategy
	c.mu.Unlock()
}

// apply applies the given options to the catalog options.
func (c *options) apply(opts ...Option) *options {
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// defaults returns the default options for a catalog.
func defaults() *options {
	return &options{
		readFS:        nil,
		writePath:     "",
		mergeStrategy: MergeEnrichEmpty,
	}
}

// Option configures a catalog.
type Option func(*options)

// WithFS configures the catalog to use a custom fs.FS for reading.
func WithFS(fsys fs.FS) Option {
	return func(c *options) {
		c.readFS = fsys
	}
}

// WithPath configures the catalog to use a directory path for reading
// This creates an os.DirFS under the hood.
func WithPath(path string) Option {
	return func(c *options) {
		c.readFS = os.DirFS(path)
		c.writePath = path // Also set as write path
	}
}

// WithEmbedded configures the catalog to use embedded files.
func WithEmbedded() Option {
	return func(c *options) {
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
	return func(c *options) {
		c.writePath = path
	}
}

// WithMergeStrategy sets the default merge strategy.
func WithMergeStrategy(strategy MergeStrategy) Option {
	return func(c *options) {
		c.mergeStrategy = strategy
	}
}
