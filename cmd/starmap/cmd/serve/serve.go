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
  api   - REST API server [default: :8080]

Examples:
  starmap serve api --port 3000     # Start API server on :3000

Environment Variables:
  PORT                 - Default port for single services
  STARMAP_API_PORT     - API server port
  HOST                 - Bind address (default: localhost)`,
		Example: `  starmap serve api --cors
  starmap serve api --port 8080`,
	}

	// Add subcommands
	cmd.AddCommand(NewAPICommand())

	return cmd
}
