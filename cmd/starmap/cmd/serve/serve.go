// Package serve provides HTTP server commands.
package serve

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the serve command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP servers for various resources",
		Long: `Serve starts HTTP servers for different starmap resources.

Available services:
  docs  - Documentation site (Hugo) [default: :1313]
  api   - REST API server [default: :8080]
  all   - Both docs and API servers

Examples:
  starmap serve docs                # Start docs server on :1313
  starmap serve api --port 3000     # Start API server on :3000
  starmap serve all                 # Start both services
  
Environment Variables:
  PORT                 - Default port for single services
  STARMAP_DOCS_PORT    - Documentation server port
  STARMAP_API_PORT     - API server port
  HOST                 - Bind address (default: localhost)`,
		Example: `  starmap serve docs
  starmap serve docs --port 8080
  starmap serve api --cors
  starmap serve all`,
	}

	// Add subcommands
	cmd.AddCommand(NewDocsCommand())
	cmd.AddCommand(NewAPICommand())
	cmd.AddCommand(NewAllCommand())

	return cmd
}
