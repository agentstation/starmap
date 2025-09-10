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
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/filter"
	"github.com/agentstation/starmap/internal/cmd/globals"
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
		resourceFlags := globals.ParseResources(cmd)
		showDetails, _ := cmd.Flags().GetBool("details")
		capability, _ := cmd.Flags().GetString("capability")
		minContext, _ := cmd.Flags().GetInt64("min-context")
		maxPrice, _ := cmd.Flags().GetFloat64("max-price")
		exportFormat, _ := cmd.Flags().GetString("export")

		return listModels(cmd, resourceFlags, capability, minContext, maxPrice, showDetails, exportFormat)
	},
}

func init() {
	// Add resource-specific flags
	globals.AddResourceFlags(ModelsCmd)
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
func listModels(cmd *cobra.Command, flags *globals.ResourceFlags, capability string, minContext int64, maxPrice float64, showDetails bool, exportFormat string) error {
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

	// Get global flags and format output
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		tableData := table.ModelsToTableData(filtered, showDetails)
		// Convert to output.Data for formatter compatibility
		outputData = output.Data{
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
			globalFlags, err := globals.Parse(cmd)
			if err != nil {
				return err
			}
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

// printModelDetails prints detailed model information using table format.
func printModelDetails(model catalogs.Model, provider catalogs.Provider) {
	formatter := output.NewFormatter(output.FormatTable)

	fmt.Printf("Model: %s\n\n", model.ID)

	printBasicInfo(model, provider, formatter)
	printLimitsInfo(model, formatter)
	printPricingInfo(model, formatter)
	printFeaturesInfo(model, formatter)
	printArchitectureInfo(model, formatter)
}

func printBasicInfo(model catalogs.Model, provider catalogs.Provider, formatter output.Formatter) {
	basicRows := [][]string{
		{"Model ID", model.ID},
		{"Name", model.Name},
		{"Provider", fmt.Sprintf("%s (%s)", provider.Name, provider.ID)},
	}

	// Show authors
	if len(model.Authors) > 0 {
		authorNames := make([]string, len(model.Authors))
		for i, author := range model.Authors {
			authorNames[i] = author.Name
		}
		basicRows = append(basicRows, []string{"Authors", strings.Join(authorNames, ", ")})
	} else {
		basicRows = append(basicRows, []string{"Authors", "Unknown"})
	}

	if model.Description != "" {
		description := model.Description
		if len(description) > 80 {
			description = description[:77] + "..."
		}
		basicRows = append(basicRows, []string{"Description", description})
	}

	basicTable := output.Data{
		Headers: []string{"Property", "Value"},
		Rows:    basicRows,
	}

	fmt.Println("Basic Information:")
	_ = formatter.Format(os.Stdout, basicTable)
	fmt.Println()
}

func printLimitsInfo(model catalogs.Model, formatter output.Formatter) {
	if model.Limits == nil {
		return
	}

	var limitRows [][]string
	if model.Limits.ContextWindow > 0 {
		limitRows = append(limitRows, []string{"Context Window", fmt.Sprintf("%s tokens", table.FormatNumber(model.Limits.ContextWindow))})
	}
	if model.Limits.OutputTokens > 0 {
		limitRows = append(limitRows, []string{"Max Output", fmt.Sprintf("%s tokens", table.FormatNumber(model.Limits.OutputTokens))})
	}

	if len(limitRows) > 0 {
		limitsTable := output.Data{
			Headers: []string{"Limit", "Value"},
			Rows:    limitRows,
		}
		fmt.Println("Limits:")
		_ = formatter.Format(os.Stdout, limitsTable)
		fmt.Println()
	}
}

func printPricingInfo(model catalogs.Model, formatter output.Formatter) {
	if model.Pricing == nil || model.Pricing.Tokens == nil {
		return
	}

	var pricingRows [][]string
	if model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Input.Per1M > 0 {
		pricingRows = append(pricingRows, []string{"Input", fmt.Sprintf("$%.6f per 1M tokens", model.Pricing.Tokens.Input.Per1M)})
	}
	if model.Pricing.Tokens.Output != nil && model.Pricing.Tokens.Output.Per1M > 0 {
		pricingRows = append(pricingRows, []string{"Output", fmt.Sprintf("$%.6f per 1M tokens", model.Pricing.Tokens.Output.Per1M)})
	}

	if len(pricingRows) > 0 {
		pricingTable := output.Data{
			Headers: []string{"Type", "Price"},
			Rows:    pricingRows,
		}
		fmt.Println("Pricing:")
		_ = formatter.Format(os.Stdout, pricingTable)
		fmt.Println()
	}
}

func printFeaturesInfo(model catalogs.Model, formatter output.Formatter) {
	if model.Features == nil {
		return
	}

	var featureRows [][]string

	// Check modality features
	featureRows = addModalityFeatures(featureRows, model.Features)

	// Other features
	if model.Features.ToolCalls {
		featureRows = append(featureRows, []string{"Function Calling", "✅ Supported"})
	}
	if model.Features.WebSearch {
		featureRows = append(featureRows, []string{"Web Search", "✅ Supported"})
	}
	if model.Features.Reasoning {
		featureRows = append(featureRows, []string{"Reasoning", "✅ Supported"})
	}

	if len(featureRows) > 0 {
		featuresTable := output.Data{
			Headers: []string{"Feature", "Status"},
			Rows:    featureRows,
		}
		fmt.Println("Features:")
		_ = formatter.Format(os.Stdout, featuresTable)
		fmt.Println()
	}
}

func addModalityFeatures(featureRows [][]string, features *catalogs.ModelFeatures) [][]string {
	// Check for vision capability
	for _, modality := range features.Modalities.Input {
		if modality == "image" {
			featureRows = append(featureRows, []string{"Vision", "✅ Supported"})
			break
		}
	}

	// Check for audio capabilities
	hasAudioInput := false
	hasAudioOutput := false
	for _, modality := range features.Modalities.Input {
		if modality == "audio" {
			hasAudioInput = true
			break
		}
	}
	for _, modality := range features.Modalities.Output {
		if modality == "audio" {
			hasAudioOutput = true
			break
		}
	}
	if hasAudioInput {
		featureRows = append(featureRows, []string{"Audio Input", "✅ Supported"})
	}
	if hasAudioOutput {
		featureRows = append(featureRows, []string{"Audio Output", "✅ Supported"})
	}

	return featureRows
}

func printArchitectureInfo(model catalogs.Model, formatter output.Formatter) {
	if model.Metadata == nil || model.Metadata.Architecture == nil {
		return
	}

	var archRows [][]string
	if model.Metadata.Architecture.ParameterCount != "" {
		archRows = append(archRows, []string{"Size", model.Metadata.Architecture.ParameterCount})
	}
	if model.Metadata.Architecture.Tokenizer != "" {
		archRows = append(archRows, []string{"Tokenizer", model.Metadata.Architecture.Tokenizer.String()})
	}

	if len(archRows) > 0 {
		archTable := output.Data{
			Headers: []string{"Property", "Value"},
			Rows:    archRows,
		}
		fmt.Println("Architecture:")
		_ = formatter.Format(os.Stdout, archTable)
		fmt.Println()
	}
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
