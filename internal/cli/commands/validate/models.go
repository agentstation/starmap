package validate

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cli/emoji"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// NewModelsCommand creates the validate models subcommand using app context.
func NewModelsCommand(app application) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Validate model definitions",
		Long: `Validate model definitions in the catalog.

This checks:
  - Required fields (id, name, provider)
  - Provider references exist
  - Author references exist (if specified)
  - Data consistency and formats`,
		RunE: func(_ *cobra.Command, args []string) error {
			// This command doesn't take positional arguments yet
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument: %s", args[0])
			}

			logger := app.Logger()
			verbose := logger.GetLevel() <= zerolog.InfoLevel
			return validateModelConsistency(app, verbose)
		},
	}
}

func validateModelConsistency(app application, verbose bool) error {
	// Load catalog from app context
	cat, err := app.Catalog()
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	providers := cat.Providers().List()
	var validationErrors []string
	totalModels := 0

	for _, provider := range providers {
		count, issues := validateProviderModels(cat, provider, verbose)
		totalModels += count
		validationErrors = append(validationErrors, issues...)
	}

	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			fmt.Printf("  %s %s\n", emoji.Error, err)
		}
		return fmt.Errorf("found %d validation errors", len(validationErrors))
	}

	if totalModels > 0 {
		fmt.Printf("%s Validated %d models successfully\n", emoji.Success, totalModels)
	}
	return nil
}

func validateProviderModels(
	catalog *catalogs.Catalog,
	provider catalogs.Provider,
	verbose bool,
) (int, []string) {
	if provider.Models == nil {
		return 0, nil
	}
	seenIDs := make(map[string]bool)
	issues := make([]string, 0)
	for _, model := range provider.Models {
		if model.ID == "" {
			issues = append(
				issues,
				fmt.Sprintf("model in provider '%s' missing required field 'id'", provider.ID),
			)
			continue
		}
		if seenIDs[model.ID] {
			issues = append(
				issues,
				fmt.Sprintf("duplicate model ID '%s' in provider '%s'", model.ID, provider.ID),
			)
		}
		seenIDs[model.ID] = true
		issues = append(issues, modelConsistencyIssues(catalog, model)...)
		if verbose {
			fmt.Printf("  %s Validated model: %s\n", emoji.Success, model.Name)
		}
	}
	return len(provider.Models), issues
}

func modelConsistencyIssues(catalog *catalogs.Catalog, model *catalogs.Model) []string {
	issues := make([]string, 0)
	if model.Name == "" {
		issues = append(issues, fmt.Sprintf("model %s missing required field 'name'", model.ID))
	}
	for _, author := range model.Authors {
		if _, found := catalog.Authors().Resolve(author.ID); !found {
			issues = append(
				issues,
				fmt.Sprintf("model %s references unknown author: %s", model.ID, author.ID),
			)
		}
	}
	if model.Limits == nil {
		return issues
	}
	if model.Limits.ContextWindow < 0 {
		issues = append(
			issues,
			fmt.Sprintf("model %s has invalid context_window: %d", model.ID, model.Limits.ContextWindow),
		)
	}
	if model.Limits.InputTokens < 0 {
		issues = append(
			issues,
			fmt.Sprintf("model %s has invalid input_tokens: %d", model.ID, model.Limits.InputTokens),
		)
	}
	if model.Limits.OutputTokens < 0 {
		issues = append(
			issues,
			fmt.Sprintf("model %s has invalid output_tokens: %d", model.ID, model.Limits.OutputTokens),
		)
	}
	return issues
}
