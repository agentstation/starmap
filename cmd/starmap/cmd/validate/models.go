package validate

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/context"
)

// NewModelsCommand creates the validate models subcommand using app context.
func NewModelsCommand(appCtx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Validate model definitions",
		Long: `Validate model definitions in the catalog.

This checks:
  - Required fields (id, name, provider)
  - Provider references exist
  - Author references exist (if specified)
  - Data consistency and formats`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// This command doesn't take positional arguments yet
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument: %s", args[0])
			}

			logger := appCtx.Logger()
			verbose := logger.GetLevel() <= zerolog.InfoLevel
			return validateModelConsistency(appCtx, verbose)
		},
	}
}

func validateModelConsistency(appCtx context.Context, verbose bool) error {
	// Load catalog from app context
	cat, err := appCtx.Catalog()
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
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
		// Add aliases to the map
		for _, alias := range a.Aliases {
			authorMap[string(alias)] = true
		}
	}

	var validationErrors []string
	totalModels := 0

	// Validate models per provider (proper scoping)
	for _, provider := range providers {
		if provider.Models == nil {
			continue
		}

		seenIDs := make(map[string]bool)

		for _, model := range provider.Models {
			totalModels++

			// Check required fields
			if model.ID == "" {
				validationErrors = append(validationErrors,
					fmt.Sprintf("model in provider '%s' missing required field 'id'", provider.ID))
				continue
			}

			// Check for duplicate IDs within this provider
			if seenIDs[model.ID] {
				validationErrors = append(validationErrors,
					fmt.Sprintf("duplicate model ID '%s' in provider '%s'", model.ID, provider.ID))
			}
			seenIDs[model.ID] = true

			if model.Name == "" {
				validationErrors = append(validationErrors,
					fmt.Sprintf("model %s missing required field 'name'", model.ID))
			}

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
	}

	// Also validate models from authors
	for _, author := range cat.Authors().List() {
		if author.Models == nil {
			continue
		}

		for _, model := range author.Models {
			totalModels++

			// Check required fields
			if model.ID == "" {
				validationErrors = append(validationErrors,
					fmt.Sprintf("model in author '%s' missing required field 'id'", author.ID))
				continue
			}

			if model.Name == "" {
				validationErrors = append(validationErrors,
					fmt.Sprintf("model %s missing required field 'name'", model.ID))
			}

			// Check author references if specified
			for _, modelAuthor := range model.Authors {
				if !authorMap[string(modelAuthor.ID)] {
					validationErrors = append(validationErrors,
						fmt.Sprintf("model %s references unknown author: %s", model.ID, modelAuthor.ID))
				}
			}

			if verbose {
				fmt.Printf("  ✓ Validated model: %s (from author %s)\n", model.Name, author.ID)
			}
		}
	}

	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			fmt.Printf("  ❌ %s\n", err)
		}
		return fmt.Errorf("found %d validation errors", len(validationErrors))
	}

	if totalModels > 0 {
		fmt.Printf("✅ Validated %d models successfully\n", totalModels)
	}
	return nil
}
