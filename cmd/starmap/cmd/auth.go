package cmd

import (
	"github.com/spf13/cobra"

	authcmd "github.com/agentstation/starmap/cmd/starmap/cmd/auth"
)

// authCmd represents the auth command.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication for AI providers",
	Long: `Check and manage authentication status for AI model providers.

Commands:
  status - Show which providers have configured credentials
  verify - Test that credentials actually work
  gcloud - Manage Google Cloud authentication

Examples:
  starmap auth                 # Show authentication status
  starmap auth status          # Explicit status command
  starmap auth verify          # Test all credentials
  starmap auth gcloud          # Authenticate with Google Cloud
  starmap auth gcloud --check  # Check Google Cloud auth status`,
	RunE: authcmd.StatusCmd.RunE, // Default to status
}

func init() {
	rootCmd.AddCommand(authCmd)

	// Add subcommands
	authCmd.AddCommand(authcmd.StatusCmd)
	authCmd.AddCommand(authcmd.VerifyCmd)
	authCmd.AddCommand(authcmd.GCloudCmd)
}