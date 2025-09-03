package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/convert"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/spf13/cobra"
)

var (
	exportFlagFormat   string
	exportFlagProvider string
	exportFlagOutput   string
	exportFlagPretty   bool
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export models in OpenAI or OpenRouter format",
	Long: `Export converts the model catalog to different API response formats
for compatibility with various tools and services.

Supported formats:
  - openai: OpenAI Models API response format
  - openrouter: OpenRouter Models API response format

Models can be exported from the embedded catalog or fetched live from
a provider's API if specified.`,
	Example: `  starmap export --format openai
  starmap export --format openrouter --output models.json
  starmap export --format openai --provider anthropic
  starmap export -f openai -p groq --pretty`,
	RunE: runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVarP(&exportFlagFormat, "format", "f", "openai", "Export format: openai or openrouter")
	exportCmd.Flags().StringVarP(&exportFlagProvider, "provider", "p", "", "Provider to fetch models from (optional)")
	exportCmd.Flags().StringVarP(&exportFlagOutput, "output", "o", "", "Output file (default: stdout)")
	exportCmd.Flags().BoolVar(&exportFlagPretty, "pretty", true, "Pretty print JSON output")
}

func runExport(cmd *cobra.Command, args []string) error {
	var models []*catalogs.Model

	if exportFlagProvider != "" {
		// Fetch models from specific provider
		pid := catalogs.ProviderID(exportFlagProvider)
		sm, err := starmap.New()
		if err != nil {
			return errors.WrapResource("create", "starmap", "", err)
		}

		catalog, err := sm.Catalog()
		if err != nil {
			return errors.WrapResource("get", "catalog", "", err)
		}
		// Get provider from catalog
		provider, found := catalog.Providers().Get(pid)
		if !found {
			return &errors.NotFoundError{
				Resource: "provider",
				ID:       exportFlagProvider,
			}
		}

		// Create provider fetcher using public API
		fetcher := sources.NewProviderFetcher()

		ctx := context.Background()
		modelValues, err := fetcher.FetchModels(ctx, provider)
		if err != nil {
			return &errors.SyncError{
				Provider: exportFlagProvider,
				Err:      err,
			}
		}
		// Convert values to pointers
		models = make([]*catalogs.Model, len(modelValues))
		for i := range modelValues {
			models[i] = &modelValues[i]
		}
	} else {
		// Use embedded catalog
		sm, err := starmap.New()
		if err != nil {
			return errors.WrapResource("create", "starmap", "", err)
		}

		catalog, err := sm.Catalog()
		if err != nil {
			return errors.WrapResource("get", "catalog", "", err)
		}
		// Get all models from the catalog
		allModels := catalog.GetAllModels()
		models = make([]*catalogs.Model, len(allModels))
		for i := range allModels {
			models[i] = &allModels[i]
		}
	}

	if len(models) == 0 {
		fmt.Fprintln(os.Stderr, "No models to export")
		return nil
	}

	// Convert models to requested format
	var output any
	switch strings.ToLower(exportFlagFormat) {
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
			Field:   "format",
			Value:   exportFlagFormat,
			Message: "unsupported format (use 'openai' or 'openrouter')",
		}
	}

	// Create encoder
	var encoder *json.Encoder
	if exportFlagOutput != "" {
		file, err := os.Create(exportFlagOutput)
		if err != nil {
			return errors.WrapIO("create", exportFlagOutput, err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Printf("Warning: failed to close file: %v\n", err)
			}
		}()
		encoder = json.NewEncoder(file)
	} else {
		encoder = json.NewEncoder(os.Stdout)
	}

	// Configure encoder
	if exportFlagPretty {
		encoder.SetIndent("", "  ")
	}

	// Write output
	if err := encoder.Encode(output); err != nil {
		return errors.WrapParse("json", "output", err)
	}

	if exportFlagOutput != "" {
		fmt.Fprintf(os.Stderr, "Exported %d models to %s\n", len(models), exportFlagOutput)
	}

	return nil
}
