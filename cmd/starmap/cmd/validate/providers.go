// Package validate provides catalog validation commands.
package validate

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/internal/cli/emoji"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// NewProvidersCommand creates the validate providers subcommand using app context.
func NewProvidersCommand(app application.Application) *cobra.Command {
	return &cobra.Command{
		Use:   "providers",
		Short: "Validate providers.yaml structure",
		Long: `Validate that providers.yaml has all required fields and follows the schema.

This checks:
  - Required fields (id, name)
  - API key configuration consistency
  - Catalog configuration validity
  - URL formats and patterns`,
		RunE: func(_ *cobra.Command, args []string) error {
			// This command doesn't take positional arguments yet
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument: %s", args[0])
			}

			logger := app.Logger()
			verbose := logger.GetLevel() <= zerolog.InfoLevel
			return validateProvidersStructure(app, verbose)
		},
	}
}

func validateProvidersStructure(app application.Application, verbose bool) error {
	// Load catalog from app context
	cat, err := app.Catalog()
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	providers := cat.Providers().List()
	if len(providers) == 0 {
		return fmt.Errorf("no providers found in catalog")
	}

	var validationErrors []string

	for _, provider := range providers {
		// Check required fields
		if provider.ID == "" {
			validationErrors = append(validationErrors,
				"provider missing required field 'id'")
			continue
		}

		if provider.Name == "" {
			validationErrors = append(validationErrors,
				fmt.Sprintf("provider %s missing required field 'name'", provider.ID))
		}

		if err := provider.ValidateConfiguration(); err != nil {
			validationErrors = append(validationErrors,
				fmt.Sprintf("provider %s configuration: %v", provider.ID, err))
		}

		// Validate URLs
		if err := validateProviderURLs(&provider); err != nil {
			validationErrors = append(validationErrors,
				fmt.Sprintf("provider %s URLs: %v", provider.ID, err))
		}

		if verbose {
			fmt.Printf("  %s Validated provider: %s\n", emoji.Success, provider.Name)
		}
	}

	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			fmt.Printf("  %s %s\n", emoji.Error, err)
		}
		return fmt.Errorf("found %d validation errors", len(validationErrors))
	}

	fmt.Printf("%s Validated %d providers successfully\n", emoji.Success, len(providers))
	return nil
}

func validateProviderURLs(provider *catalogs.Provider) error {
	// Check various URL fields
	if provider.IconURL != nil && *provider.IconURL != "" && !isValidURL(*provider.IconURL) {
		return fmt.Errorf("invalid icon_url")
	}

	if provider.StatusPageURL != nil && *provider.StatusPageURL != "" && !isValidURL(*provider.StatusPageURL) {
		return fmt.Errorf("invalid status_page_url")
	}
	if provider.Catalog != nil {
		for _, source := range provider.Catalog.Sources {
			if source.Docs != "" && !isValidURL(source.Docs) {
				return fmt.Errorf("invalid docs URL for source %s", source.ID)
			}
		}
	}

	return nil
}

func isValidURL(url string) bool {
	// Basic URL validation
	if len(url) < 8 {
		return false
	}
	return url[:7] == "http://" || url[:8] == "https://"
}
