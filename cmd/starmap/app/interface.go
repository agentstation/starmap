package app

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Interface defines the interface that commands need from the app.
// The App struct automatically implements this interface, providing
// dependency injection for commands while maintaining testability.
//
// Commands should accept this interface rather than the concrete App type,
// allowing for easier testing with mock implementations.
type Interface interface {
	// Catalog returns the current catalog from the default starmap instance.
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

	// Config returns the application configuration.
	// Commands can use this to check flags and settings.
	Config() *Config

	// Version returns the application version string.
	Version() string

	// Commit returns the git commit hash.
	Commit() string

	// Date returns the build date.
	Date() string

	// BuiltBy returns the build system identifier.
	BuiltBy() string
}

// Ensure App implements Interface at compile time.
var _ Interface = (*App)(nil)
