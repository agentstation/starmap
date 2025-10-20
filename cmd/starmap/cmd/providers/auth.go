package providers

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/auth"
	"github.com/agentstation/starmap/internal/cmd/application"
)

// NewAuthCommand creates the auth subcommand for provider authentication.
func NewAuthCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with AI providers",
		Long: `Authenticate with AI providers and manage authentication credentials.

Check authentication status, verify credentials, and configure Google Cloud authentication.`,
		Example: `  starmap providers auth status        # Check authentication status
  starmap providers auth verify        # Verify all configured providers
  starmap providers auth verify openai # Verify specific provider
  starmap providers auth gcloud        # Google Cloud authentication helper`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add auth subcommands from the auth package
	cmd.AddCommand(auth.NewStatusCommand(app))
	cmd.AddCommand(auth.NewVerifyCommand(app))
	cmd.AddCommand(auth.NewGCloudCommand())

	return cmd
}
