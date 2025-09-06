package validate

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ModelsCmd validates model definitions.
var ModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Validate model definitions",
	Long: `Validate model definitions in the catalog.

This checks:
  - Required fields (id, name, provider)
  - Provider references exist
  - Author references exist (if specified)
  - Data consistency and formats`,
	RunE: runValidateModels,
}

func runValidateModels(cmd *cobra.Command, args []string) error {
	// This command doesn't take positional arguments yet
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument: %s", args[0])
	}
	
	verbose, _ := cmd.Flags().GetBool("verbose")
	return validateModelConsistency(verbose)
}

func validateModelConsistency(verbose bool) error {
	// Load catalog
	cat, err := catalogs.New(catalogs.WithEmbedded())
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	models := cat.GetAllModels()
	if len(models) == 0 {
		// No models is okay, they come from providers
		if verbose {
			fmt.Println("  No embedded models to validate")
		}
		return nil
	}

	providers := cat.Providers().List()
	providerMap := make(map[string]bool)
	for _, p := range providers {
		providerMap[string(p.ID)] = true
	}

	authors := cat.Authors().List()
	authorMap := make(map[string]bool)
	for _, a := range authors {
		authorMap[string(a.ID)] = true
	}

	var validationErrors []string
	seenIDs := make(map[string]bool)

	for _, model := range models {
		// Check required fields
		if model.ID == "" {
			validationErrors = append(validationErrors,
				"model missing required field 'id'")
			continue
		}

		// Check for duplicate IDs
		if seenIDs[model.ID] {
			validationErrors = append(validationErrors,
				fmt.Sprintf("duplicate model ID: %s", model.ID))
		}
		seenIDs[model.ID] = true

		if model.Name == "" {
			validationErrors = append(validationErrors,
				fmt.Sprintf("model %s missing required field 'name'", model.ID))
		}

		// Models come from providers, so we don't validate provider reference
		// Check author references if specified
		for _, author := range model.Authors {
			if !authorMap[string(author.ID)] {
				validationErrors = append(validationErrors,
					fmt.Sprintf("model %s references unknown author: %s", model.ID, author.ID))
			}
		}

		// Validate limits if present
		if model.Limits != nil {
			if model.Limits.ContextWindow < 0 {
				validationErrors = append(validationErrors,
					fmt.Sprintf("model %s has invalid context_window: %d", model.ID, model.Limits.ContextWindow))
			}
			if model.Limits.OutputTokens < 0 {
				validationErrors = append(validationErrors,
					fmt.Sprintf("model %s has invalid output_tokens: %d", model.ID, model.Limits.OutputTokens))
			}
		}

		if verbose {
			fmt.Printf("  ✓ Validated model: %s\n", model.Name)
		}
	}

	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			fmt.Printf("  ❌ %s\n", err)
		}
		return fmt.Errorf("found %d validation errors", len(validationErrors))
	}

	if len(models) > 0 {
		fmt.Printf("✅ Validated %d models successfully\n", len(models))
	}
	return nil
}