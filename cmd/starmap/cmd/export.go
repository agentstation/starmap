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
			return fmt.Errorf("creating starmap: %w", err)
		}

		catalog, err := sm.Catalog()
		if err != nil {
			return fmt.Errorf("getting catalog: %w", err)
		}
		// Get provider from catalog
		provider, found := catalog.Providers().Get(pid)
		if !found {
			return fmt.Errorf("provider %s not found in catalog", exportFlagProvider)
		}

		// Load API key and environment variables from environment
		provider.LoadAPIKey()
		provider.LoadEnvVars()

		// Get client for provider
		client, err := provider.Client()
		if err != nil {
			return fmt.Errorf("getting client for %s: %w", exportFlagProvider, err)
		}

		ctx := context.Background()
		modelValues, err := client.ListModels(ctx)
		if err != nil {
			return fmt.Errorf("fetching models from %s: %w", exportFlagProvider, err)
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
			return fmt.Errorf("creating starmap: %w", err)
		}

		catalog, err := sm.Catalog()
		if err != nil {
			return fmt.Errorf("getting catalog: %w", err)
		}
		// Get all models from the catalog
		models = catalog.Models().List()
	}

	if len(models) == 0 {
		fmt.Fprintln(os.Stderr, "No models to export")
		return nil
	}

	// Convert models to requested format
	var output interface{}
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
		return fmt.Errorf("unsupported format: %s (use 'openai' or 'openrouter')", exportFlagFormat)
	}

	// Create encoder
	var encoder *json.Encoder
	if exportFlagOutput != "" {
		file, err := os.Create(exportFlagOutput)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
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
		return fmt.Errorf("encoding output: %w", err)
	}

	if exportFlagOutput != "" {
		fmt.Fprintf(os.Stderr, "Exported %d models to %s\n", len(models), exportFlagOutput)
	}

	return nil
}
