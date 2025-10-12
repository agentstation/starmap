// Package appcontext provides the shared application context interface
// used by all commands. This eliminates interface duplication across
// command packages and provides a single source of truth for app dependencies.
package appcontext

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Interface defines the application context interface that commands need.
// The App struct from cmd/starmap/app automatically implements this interface,
// providing dependency injection for commands while maintaining testability.
//
// Commands should accept this interface rather than the concrete App type,
// allowing for easier testing with mock implementations.
type Interface interface {
	// Catalog returns a deep copy of the current catalog from the default starmap instance.
	// Per CLAUDE.md thread safety rules, this ALWAYS returns a deep copy to prevent data races.
	// This is a convenience method for commands that just need catalog access.
	Catalog() (catalogs.Catalog, error)

	// Starmap returns the default starmap instance, creating it lazily if needed.
	// This is thread-safe and ensures only one instance is created.
	Starmap() (starmap.Starmap, error)

	// StarmapWithOptions creates a new starmap instance with custom options.
	// Use this when a command needs specific configuration (e.g., update with --input).
	StarmapWithOptions(...starmap.Option) (starmap.Starmap, error)

	// Logger returns the configured logger instance.
	// Commands should use this for all logging operations.
	Logger() *zerolog.Logger

	// OutputFormat returns the configured output format (json, yaml, table, etc).
	// Commands that support different output formats should use this.
	OutputFormat() string

	// Version returns the application version string.
	Version() string

	// Commit returns the git commit hash.
	Commit() string

	// Date returns the build date.
	Date() string

	// BuiltBy returns the build system identifier.
	BuiltBy() string
}
