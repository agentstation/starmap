// Package context provides the application context interface for Starmap commands.
//
// The Context interface defines the contract between the application layer and
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
//	    stdctx "context"
//	    "github.com/agentstation/starmap/cmd/starmap/context"
//	)
//
//	func NewCommand(appCtx context.Context) *cobra.Command {
//	    return &cobra.Command{
//	        RunE: func(cmd *cobra.Command, args []string) error {
//	            ctx := cmd.Context() // stdctx.Context from cobra
//	            catalog, err := appCtx.Catalog()
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
//	mock := &context.MockContext{
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
package context

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Context provides the application context interface that commands need.
// The App struct from cmd/starmap/app automatically implements this interface,
// providing dependency injection for commands while maintaining testability.
//
// Commands should accept this interface rather than the concrete App type,
// allowing for easier testing with mock implementations.
//
// Thread Safety: All methods must be safe for concurrent access.
type Context interface {
	// Catalog returns a deep copy of the current catalog from the default starmap instance.
	// Per CLAUDE.md thread safety rules, this ALWAYS returns a deep copy to prevent data races.
	// This is a convenience method for commands that just need catalog access.
	Catalog() (catalogs.Catalog, error)

	// Starmap returns the starmap instance with optional configuration.
	// When called without options, returns the default cached instance (lazy-initialized, thread-safe).
	// When called with options, creates a new instance with custom configuration (no caching).
	//
	// Examples:
	//   sm, err := appCtx.Starmap()                    // default instance (cached)
	//   sm, err := appCtx.Starmap(opt1, opt2)          // custom instance (new)
	Starmap(opts ...starmap.Option) (starmap.Starmap, error)

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
