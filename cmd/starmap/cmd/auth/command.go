// Package auth provides cloud provider authentication helpers for Starmap.
package auth

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the top-level auth command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth",
		GroupID: "setup",
		Short:   "Cloud provider authentication helpers",
		Long: `Helpers for cloud provider authentication setup.

Use this command to configure authentication for cloud providers like Google Cloud,
AWS, Azure, and others. Each cloud provider has its own authentication mechanism:
- Google Cloud uses Application Default Credentials (ADC)
- AWS uses credentials and config files
- Azure uses Azure CLI authentication

Note: To view authentication status for all AI providers, use 'starmap providers'.
To test provider credentials, use 'starmap providers --test'.`,
		Example: `  starmap auth gcloud                  # Google Cloud authentication
  starmap providers                    # View auth status for all providers
  starmap providers --test             # Test provider credentials`,
	}

	// Add subcommands
	cmd.AddCommand(NewGCloudCommand())

	return cmd
}
