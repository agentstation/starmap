package providers

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/auth"
	"github.com/agentstation/starmap/internal/cmd/application"
)

// NewAuthCommand creates the auth subcommand for provider authentication.
func NewAuthCommand(app application.Application) *cobra.Command {
	// Create the status subcommand first so we can reference its RunE
	statusCmd := auth.NewStatusCommand(app)

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with AI providers",
		Long: `Authenticate with AI providers and manage authentication credentials.

Check authentication status, test credentials, and configure Google Cloud authentication.

When called without a subcommand, shows authentication status (same as 'auth status').`,
		Example: `  starmap providers auth             # Check authentication status (default)
  starmap providers auth status      # Check authentication status
  starmap providers auth test        # Test all configured providers
  starmap providers auth test openai # Test specific provider
  starmap providers auth gcloud      # Google Cloud authentication helper`,
		RunE: statusCmd.RunE, // Default to running status command
	}

	// Add auth subcommands from the auth package
	cmd.AddCommand(statusCmd)
	cmd.AddCommand(auth.NewTestCommand(app))
	cmd.AddCommand(auth.NewGCloudCommand())

	return cmd
}
