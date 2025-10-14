package serve

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
)

// NewCommand creates the serve command using app context.
func NewCommand(app application.Application) *cobra.Command {
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

	// Add subcommands with app context
	cmd.AddCommand(NewAPICommand(app))

	return cmd
}
