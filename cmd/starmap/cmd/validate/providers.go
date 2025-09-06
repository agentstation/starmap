// Package validate provides catalog validation commands.
package validate

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ProvidersCmd validates providers.yaml structure.
var ProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Validate providers.yaml structure",
	Long: `Validate that providers.yaml has all required fields and follows the schema.

This checks:
  - Required fields (id, name)
  - API key configuration consistency
  - Catalog configuration validity
  - URL formats and patterns`,
	RunE: runValidateProviders,
}

func runValidateProviders(cmd *cobra.Command, args []string) error {
	// This command doesn't take positional arguments yet
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument: %s", args[0])
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	return validateProvidersStructure(verbose)
}

func validateProvidersStructure(verbose bool) error {
	// Load providers from embedded catalog
	cat, err := catalogs.New(catalogs.WithEmbedded())
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

		// Validate API key configuration if present
		if provider.APIKey != nil {
			if err := validateAPIKeyConfig(provider); err != nil {
				validationErrors = append(validationErrors,
					fmt.Sprintf("provider %s API key config: %v", provider.ID, err))
			}
		}

		// Validate catalog section
		if provider.Catalog != nil {
			if err := validateCatalogConfig(provider); err != nil {
				validationErrors = append(validationErrors,
					fmt.Sprintf("provider %s catalog config: %v", provider.ID, err))
			}
		}

		// Validate URLs
		if err := validateProviderURLs(provider); err != nil {
			validationErrors = append(validationErrors,
				fmt.Sprintf("provider %s URLs: %v", provider.ID, err))
		}

		if verbose {
			fmt.Printf("  ✓ Validated provider: %s\n", provider.Name)
		}
	}

	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			fmt.Printf("  ❌ %s\n", err)
		}
		return fmt.Errorf("found %d validation errors", len(validationErrors))
	}

	fmt.Printf("✅ Validated %d providers successfully\n", len(providers))
	return nil
}

func validateAPIKeyConfig(provider *catalogs.Provider) error {
	if provider.APIKey.Name == "" {
		return fmt.Errorf("missing 'name' field")
	}

	// Check that auth method is specified (header or query_param)
	// Scheme is optional and works with header (e.g., "Authorization: Bearer token")
	if provider.APIKey.Header == "" && provider.APIKey.QueryParam == "" {
		return fmt.Errorf("no auth method specified (header or query_param)")
	}
	if provider.APIKey.Header != "" && provider.APIKey.QueryParam != "" {
		return fmt.Errorf("cannot specify both header and query_param")
	}

	return nil
}

func validateCatalogConfig(provider *catalogs.Provider) error {
	catalog := provider.Catalog

	// Check API key requirement consistency
	if catalog.APIKeyRequired != nil && *catalog.APIKeyRequired && provider.APIKey == nil {
		return fmt.Errorf("api_key_required is true but no api_key configuration")
	}

	// Validate URLs are present if specified
	if catalog.APIURL != nil && *catalog.APIURL != "" && !isValidURL(*catalog.APIURL) {
		return fmt.Errorf("invalid api_url format")
	}

	if catalog.DocsURL != nil && *catalog.DocsURL != "" && !isValidURL(*catalog.DocsURL) {
		return fmt.Errorf("invalid docs_url format")
	}

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

	return nil
}

func isValidURL(url string) bool {
	// Basic URL validation
	if len(url) < 8 {
		return false
	}
	return url[:7] == "http://" || url[:8] == "https://"
}
