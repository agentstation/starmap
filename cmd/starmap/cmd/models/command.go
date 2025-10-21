// Package models provides the models resource command and subcommands.
package models

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/pkg/errors"
)

// NewCommand creates the models resource command.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "models [model-id]",
		GroupID: "catalog",
		Short:   "Manage AI models",
		Long: `Manage AI models in the catalog.

List models with filtering options, or show detailed information about specific models.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  starmap models list                       # List all models
  starmap models list --provider openai     # List OpenAI models only
  starmap models claude-3-5-sonnet          # Show specific model details
  starmap models list --search claude       # Search for models by name`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Single model detail view
			if len(args) == 1 {
				return showModelDetails(cmd, app, args[0])
			}

			// Default behavior - show help
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand(app))
	cmd.AddCommand(NewHistoryCommand(app))

	return cmd
}

// showModelDetails shows detailed information about a specific model.
func showModelDetails(cmd *cobra.Command, app application.Application, modelID string) error {
	// Get catalog from app
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Find specific model across all providers
	providers := cat.Providers().List()
	for _, provider := range providers {
		if model, exists := provider.Models[modelID]; exists {
			globalFlags, err := globals.Parse(cmd)
			if err != nil {
				return err
			}
			formatter := format.NewFormatter(format.Format(globalFlags.Output))

			// For table output, show detailed view
			if globalFlags.Output == constants.FormatTable || globalFlags.Output == "" {
				printModelDetails(model, provider)
				return nil
			}

			// For structured output, return the model
			return formatter.Format(cmd.OutOrStdout(), model)
		}
	}

	// Suppress usage display for not found errors
	cmd.SilenceUsage = true
	return &errors.NotFoundError{
		Resource: "model",
		ID:       modelID,
	}
}
