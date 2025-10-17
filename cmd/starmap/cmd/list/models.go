package list

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	cmdconstants "github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/filter"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/convert"
	"github.com/agentstation/starmap/pkg/errors"
)

// NewModelsCommand creates the list models subcommand using app context.
func NewModelsCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "models [model-id]",
		Short:   "List models from catalog",
		Aliases: []string{"model"},
		Args:    cobra.MaximumNArgs(1),
		Example: `  starmap list models                          # List all models
  starmap list models claude-3-5-sonnet        # Show specific model details
  starmap list models --provider openai        # List OpenAI models only
  starmap list models --search claude          # Search for models by name`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get logger from app
			logger := app.Logger()

			// Single model detail view
			if len(args) == 1 {
				return showModelDetails(cmd, app, logger, args[0])
			}

			// List view with filters
			resourceFlags := globals.ParseResources(cmd)
			showDetails, _ := cmd.Flags().GetBool("details")
			capability, _ := cmd.Flags().GetString("capability")
			minContext, _ := cmd.Flags().GetInt64("min-context")
			maxPrice, _ := cmd.Flags().GetFloat64("max-price")
			exportFormat, _ := cmd.Flags().GetString("export")

			return listModels(cmd, app, logger, resourceFlags, capability, minContext, maxPrice, showDetails, exportFormat)
		},
	}

	// Add resource-specific flags
	globals.AddResourceFlags(cmd)
	cmd.Flags().Bool("details", false,
		"Show detailed information for each model")
	cmd.Flags().String("capability", "",
		"Filter by capability (e.g., tool_calls, reasoning, vision)")
	cmd.Flags().Int64("min-context", 0,
		"Minimum context window size")
	cmd.Flags().Float64("max-price", 0,
		"Maximum price per 1M input tokens")
	cmd.Flags().String("export", "",
		"Export models in specified format (openai, openrouter)")

	return cmd
}

// listModels lists all models with optional filters using app context.
func listModels(cmd *cobra.Command, app application.Application, logger *zerolog.Logger, flags *globals.ResourceFlags, capability string, minContext int64, maxPrice float64, showDetails bool, exportFormat string) error {
	// Get catalog from app
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Get all models
	allModels := cat.Models().List()

	// Apply filters
	modelFilter := &filter.ModelFilter{
		Provider:   flags.Provider,
		Author:     flags.Author,
		Capability: capability,
		MinContext: minContext,
		MaxPrice:   maxPrice,
		Search:     flags.Search,
	}
	filtered := modelFilter.Apply(allModels)

	// Sort models
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	// Apply limit
	if flags.Limit > 0 && len(filtered) > flags.Limit {
		filtered = filtered[:flags.Limit]
	}

	// Handle export format if specified
	if exportFormat != "" {
		modelPointers := make([]*catalogs.Model, len(filtered))
		for i := range filtered {
			modelPointers[i] = &filtered[i]
		}
		return exportModels(modelPointers, exportFormat)
	}

	// Get global flags and format output
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case cmdconstants.FormatTable, cmdconstants.FormatWide, "":
		modelPointers := make([]*catalogs.Model, len(filtered))
		for i := range filtered {
			modelPointers[i] = &filtered[i]
		}
		tableData := table.ModelsToTableData(modelPointers, showDetails)
		outputData = output.Data{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		logger.Info().Msgf("Found %d models", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showModelDetails shows detailed information about a specific model using app context.
func showModelDetails(cmd *cobra.Command, app application.Application, _ *zerolog.Logger, modelID string) error {
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
			formatter := output.NewFormatter(output.Format(globalFlags.Output))

			// For table output, show detailed view
			if globalFlags.Output == cmdconstants.FormatTable || globalFlags.Output == "" {
				printModelDetails(model, provider)
				return nil
			}

			// For structured output, return the model
			return formatter.Format(os.Stdout, model)
		}
	}

	// Suppress usage display for not found errors
	cmd.SilenceUsage = true
	return &errors.NotFoundError{
		Resource: "model",
		ID:       modelID,
	}
}

// exportModels exports models in the specified format (openai or openrouter).
func exportModels(models []*catalogs.Model, format string) error {
	var output any
	switch strings.ToLower(format) {
	case "openai":
		openAIModels := make([]convert.OpenAIModel, 0, len(models))
		for _, model := range models {
			openAIModels = append(openAIModels, convert.ToOpenAIModel(model))
		}
		output = convert.OpenAIModelsResponse{
			Object: "list",
			Data:   openAIModels,
		}
	case "openrouter":
		openRouterModels := make([]convert.OpenRouterModel, 0, len(models))
		for _, model := range models {
			openRouterModels = append(openRouterModels, convert.ToOpenRouterModel(model))
		}
		output = convert.OpenRouterModelsResponse{
			Data: openRouterModels,
		}
	default:
		return &errors.ValidationError{
			Field:   "export",
			Value:   format,
			Message: "unsupported format (use 'openai' or 'openrouter')",
		}
	}

	// Pretty print JSON output
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(output); err != nil {
		return errors.WrapParse("json", "output", err)
	}

	return nil
}
