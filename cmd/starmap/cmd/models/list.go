package models

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/filter"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/convert"
	"github.com/agentstation/starmap/pkg/errors"
)

// NewListCommand creates the list subcommand for models.
func NewListCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List models from catalog",
		Example: `  starmap models list                          # List all models
  starmap models list --provider openai        # List OpenAI models only
  starmap models list --search claude          # Search for models by name
  starmap models list --capability vision      # Filter by capability
  starmap models list --min-context 100000     # Filter by context window
  starmap models list --max-price 0.50         # Filter by price
  starmap models list --details                # Show detailed information`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get logger from app
			logger := app.Logger()

			// Parse flags
			resourceFlags := globals.ParseResources(cmd)
			showDetails := mustGetBool(cmd, "details")
			capability := mustGetString(cmd, "capability")
			minContext := mustGetInt64(cmd, "min-context")
			maxPrice := mustGetFloat64(cmd, "max-price")
			exportFormat := mustGetString(cmd, "export")

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

// listModels lists all models with optional filters.
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
	formatter := format.NewFormatter(format.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		modelPointers := make([]*catalogs.Model, len(filtered))
		for i := range filtered {
			modelPointers[i] = &filtered[i]
		}
		tableData := table.ModelsToTableData(modelPointers, showDetails)
		outputData = format.Data{
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
