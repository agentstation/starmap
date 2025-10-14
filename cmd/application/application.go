// Package application provides the application interface for Starmap commands.
//
// The Application interface defines the contract between the application layer and
// command implementations, enabling dependency injection and testability.
//
// Design Principles:
//   - Accept interfaces, return structs (Go proverb)
//   - Define interfaces where they're used, not where they're implemented
//   - Keep interfaces small and focused
//
// Usage in Commands:
//
//	import (
//	    "context"
//	    "github.com/agentstation/starmap/cmd/application"
//	)
//
//	func NewCommand(app application.Application) *cobra.Command {
//	    return &cobra.Command{
//	        RunE: func(cmd *cobra.Command, args []string) error {
//	            ctx := cmd.Context() // context.Context from cobra
//	            catalog, err := app.Catalog()
//	            if err != nil {
//	                return err
//	            }
//	            // ... use catalog
//	            return nil
//	        },
//	    }
//	}
//
// Testing with Mocks:
//
//	mock := &application.Mock{
//	    CatalogFunc: func() (catalogs.Catalog, error) {
//	        return testCatalog, nil
//	    },
//	    LoggerFunc: func() *zerolog.Logger {
//	        logger := zerolog.Nop()
//	        return &logger
//	    },
//	}
//	cmd := NewCommand(mock)
//	// ... test command behavior
package application

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Application provides the application interface that commands need.
// The App struct from cmd/starmap/app automatically implements this interface,
// providing dependency injection for commands while maintaining testability.
//
// Commands should accept this interface rather than the concrete App type,
// allowing for easier testing with mock implementations.
//
// Thread Safety: All methods must be safe for concurrent access.
type Application interface {
	// Catalog returns a deep copy of the current catalog from the default starmap instance.
	// Per CLAUDE.md thread safety rules, this ALWAYS returns a deep copy to prevent data races.
	// This is a convenience method for commands that just need catalog access.
	Catalog() (catalogs.Catalog, error)

	// Starmap returns the starmap instance with optional configuration.
	// When called without options, returns the default cached instance (lazy-initialized, thread-safe).
	// When called with options, creates a new instance with custom configuration (no caching).
	//
	// Examples:
	//   sm, err := app.Starmap()                    // default instance (cached)
	//   sm, err := app.Starmap(opt1, opt2)          // custom instance (new)
	Starmap(opts ...starmap.Option) (starmap.Client, error)

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
