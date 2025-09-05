// Package list provides commands for listing starmap resources.
package list

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/catalog"
	"github.com/agentstation/starmap/internal/cmd/cmdutil"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/filter"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/convert"
	"github.com/agentstation/starmap/pkg/errors"
)

// ModelsCmd represents the list models subcommand.
var ModelsCmd = &cobra.Command{
	Use:     "models [model-id]",
	Short:   "List models from catalog",
	Aliases: []string{"model"},
	Args:    cobra.MaximumNArgs(1),
	Example: `  starmap list models                          # List all models
  starmap list models claude-3-5-sonnet        # Show specific model details
  starmap list models --provider openai        # List OpenAI models only
  starmap list models --search claude          # Search for models by name`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Single model detail view
		if len(args) == 1 {
			return showModelDetails(cmd, args[0])
		}

		// List view with filters
		resourceFlags := getResourceFlags(cmd)
		showDetails, _ := cmd.Flags().GetBool("details")
		capability, _ := cmd.Flags().GetString("capability")
		minContext, _ := cmd.Flags().GetInt64("min-context")
		maxPrice, _ := cmd.Flags().GetFloat64("max-price")
		exportFormat, _ := cmd.Flags().GetString("export")

		return listModels(resourceFlags, capability, minContext, maxPrice, showDetails, exportFormat)
	},
}

func init() {
	// Add resource-specific flags
	cmdutil.AddResourceFlags(ModelsCmd)
	ModelsCmd.Flags().Bool("details", false,
		"Show detailed information for each model")
	ModelsCmd.Flags().String("capability", "",
		"Filter by capability (e.g., tool_calls, reasoning, vision)")
	ModelsCmd.Flags().Int64("min-context", 0,
		"Minimum context window size")
	ModelsCmd.Flags().Float64("max-price", 0,
		"Maximum price per 1M input tokens")
	ModelsCmd.Flags().String("export", "",
		"Export models in specified format (openai, openrouter)")
}

// listModels lists all models with optional filters.
func listModels(flags *cmdutil.ResourceFlags, capability string, minContext int64, maxPrice float64, showDetails bool, exportFormat string) error {
	// Get catalog
	cat, err := catalog.Load()
	if err != nil {
		return err
	}

	// Get all models
	allModels := cat.GetAllModels()

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
		return exportModels(filtered, exportFormat)
	}

	// Format output
	globalFlags := getGlobalFlags()
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		tableData := table.ModelsToTableData(filtered, showDetails)
		// Convert to output.TableData for formatter compatibility
		outputData = output.TableData{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "Found %d models\n", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showModelDetails shows detailed information about a specific model.
func showModelDetails(cmd *cobra.Command, modelID string) error {
	cat, err := catalog.Load()
	if err != nil {
		return err
	}

	// Find specific model across all providers
	providers := cat.Providers().List()
	for _, provider := range providers {
		if model, exists := provider.Models[modelID]; exists {
			globalFlags := getGlobalFlags()
			formatter := output.NewFormatter(output.Format(globalFlags.Output))

			// For table output, show detailed view
			if globalFlags.Output == constants.FormatTable || globalFlags.Output == "" {
				printModelDetails(model, *provider)
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

// printModelDetails prints detailed model information in a human-readable format.
func printModelDetails(model catalogs.Model, provider catalogs.Provider) {
	fmt.Printf("Model: %s\n", model.ID)
	fmt.Printf("Name: %s\n", model.Name)
	fmt.Printf("Provider: %s (%s)\n", provider.Name, provider.ID)

	// Show authors
	if len(model.Authors) > 0 {
		authorNames := make([]string, len(model.Authors))
		for i, author := range model.Authors {
			authorNames[i] = author.Name
		}
		fmt.Printf("Authors: %s\n", strings.Join(authorNames, ", "))
	} else {
		fmt.Printf("Authors: Unknown\n")
	}

	if model.Description != "" {
		fmt.Printf("Description: %s\n", model.Description)
	}

	// Context and limits
	if model.Limits != nil {
		fmt.Printf("\nLimits:\n")
		if model.Limits.ContextWindow > 0 {
			fmt.Printf("  Context Window: %s tokens\n", table.FormatNumber(model.Limits.ContextWindow))
		}
		if model.Limits.OutputTokens > 0 {
			fmt.Printf("  Max Output: %s tokens\n", table.FormatNumber(model.Limits.OutputTokens))
		}
	}

	// Pricing
	if model.Pricing != nil && model.Pricing.Tokens != nil {
		fmt.Printf("\nPricing:\n")
		if model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Input.Per1M > 0 {
			fmt.Printf("  Input: $%.6f per 1M tokens\n", model.Pricing.Tokens.Input.Per1M)
		}
		if model.Pricing.Tokens.Output != nil && model.Pricing.Tokens.Output.Per1M > 0 {
			fmt.Printf("  Output: $%.6f per 1M tokens\n", model.Pricing.Tokens.Output.Per1M)
		}
	}

	// Features
	if model.Features != nil {
		fmt.Printf("\nFeatures:\n")

		// Check for vision capability in modalities
		for _, modality := range model.Features.Modalities.Input {
			if modality == "image" {
				fmt.Printf("  - Vision\n")
				break
			}
		}

		// Check for audio capability in modalities
		hasAudioInput := false
		hasAudioOutput := false
		for _, modality := range model.Features.Modalities.Input {
			if modality == "audio" {
				hasAudioInput = true
				break
			}
		}
		for _, modality := range model.Features.Modalities.Output {
			if modality == "audio" {
				hasAudioOutput = true
				break
			}
		}
		if hasAudioInput {
			fmt.Printf("  - Audio Input\n")
		}
		if hasAudioOutput {
			fmt.Printf("  - Audio Output\n")
		}

		// Tool calling
		if model.Features.ToolCalls {
			fmt.Printf("  - Function Calling\n")
		}

		// Web search
		if model.Features.WebSearch {
			fmt.Printf("  - Web Search\n")
		}

		// Reasoning
		if model.Features.Reasoning {
			fmt.Printf("  - Reasoning\n")
		}
	}

	// Architecture
	if model.Metadata != nil && model.Metadata.Architecture != nil {
		fmt.Printf("\nArchitecture:\n")
		if model.Metadata.Architecture.ParameterCount != "" {
			fmt.Printf("  Size: %s\n", model.Metadata.Architecture.ParameterCount)
		}
		if model.Metadata.Architecture.Tokenizer != "" {
			fmt.Printf("  Tokenizer: %s\n", model.Metadata.Architecture.Tokenizer)
		}
	}

	fmt.Println()
}

// exportModels exports models in the specified format (openai or openrouter).
func exportModels(models []catalogs.Model, format string) error {
	// Convert models to pointers for compatibility with convert package
	modelPtrs := make([]*catalogs.Model, len(models))
	for i := range models {
		modelPtrs[i] = &models[i]
	}

	// Convert models to specified format
	var output any
	switch strings.ToLower(format) {
	case "openai":
		openAIModels := make([]convert.OpenAIModel, 0, len(modelPtrs))
		for _, model := range modelPtrs {
			openAIModels = append(openAIModels, convert.ToOpenAIModel(model))
		}
		output = convert.OpenAIModelsResponse{
			Object: "list",
			Data:   openAIModels,
		}
	case "openrouter":
		openRouterModels := make([]convert.OpenRouterModel, 0, len(modelPtrs))
		for _, model := range modelPtrs {
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
