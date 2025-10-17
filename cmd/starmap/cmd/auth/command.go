package auth

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
)

// NewCommand creates the auth command using app context.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication for AI providers",
		Long: `Manage authentication credentials for AI provider APIs.

This command helps you check, verify, and configure authentication
for various AI providers including OpenAI, Anthropic, Google AI,
Google Vertex AI, Groq, DeepSeek, and Cerebras.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands with app context
	cmd.AddCommand(NewStatusCommand(app))
	cmd.AddCommand(NewVerifyCommand(app))
	cmd.AddCommand(NewGCloudCommand())

	return cmd
}
