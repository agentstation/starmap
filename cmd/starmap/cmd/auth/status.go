// Package auth provides authentication management commands.
package auth

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// NewStatusCommand creates the auth status subcommand using app context.
func NewStatusCommand(app application.Application) *cobra.Command {
	return &cobra.Command{
		Use:   "status [provider]",
		Short: "Show authentication status for all providers",
		Long: `Display which providers have credentials configured.

This shows:
  - Which providers have API keys set
  - Which are missing required credentials
  - Google Cloud authentication status for Vertex AI
  - Optional configurations

The command checks environment variables and credential files
but does not make actual API calls to verify credentials work.
Use 'starmap providers auth verify' to test credentials.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(cmd, args, app)
		},
	}
}

func runAuthStatus(cmd *cobra.Command, args []string, app application.Application) error {
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Get list of supported providers
	fetcher := sources.NewProviderFetcher(cat.Providers())
	supportedProviders := fetcher.List()

	// Create a map for quick lookup
	supportedMap := make(map[string]bool)
	for _, pid := range supportedProviders {
		supportedMap[string(pid)] = true
	}

	checker := auth.NewChecker()

	// If a specific provider was requested
	if len(args) > 0 {
		return showSingleProviderStatus(args[0], cat, checker, supportedMap)
	}

	return showAllProvidersStatus(app, cat, checker, supportedMap, cmd)
}

func showSingleProviderStatus(providerName string, cat catalogs.Catalog, checker *auth.Checker, supportedMap map[string]bool) error {
	providerID := catalogs.ProviderID(providerName)
	provider, err := cat.Provider(providerID)
	if err != nil {
		return fmt.Errorf("provider %s not found", providerName)
	}

	status := checker.CheckProvider(&provider, supportedMap)

	// Show detailed authentication information based on provider type
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		// Google Cloud providers use ADC - detailed table view
		printGoogleCloudStatus(&provider, checker)
	} else if provider.APIKey != nil {
		// API key providers get detailed table view
		printAPIKeyDetails(&provider, status)
	} else {
		// Fallback for providers without detailed view (unsupported, optional, etc.)
		printProviderStatus(&provider, status)
	}

	return nil
}

func showAllProvidersStatus(app application.Application, cat catalogs.Catalog, checker *auth.Checker, supportedMap map[string]bool, cmd *cobra.Command) error {
	// Get output format from app context
	outputFormat := format.DetectFormat(app.OutputFormat())

	var configured, missing, optional, unsupported int
	logger := app.Logger()
	verbose := logger.GetLevel() <= 0 // Info level or below

	// Group providers by status and collect data
	providers := cat.Providers().List()

	// Prepare table data
	tableRows := make([][]string, 0, len(providers))
	for _, provider := range providers {
		status := checker.CheckProvider(&provider, supportedMap)

		// Skip unsupported unless verbose
		if status.State == auth.StateUnsupported && !verbose {
			unsupported++
			continue
		}

		// Create table row
		statusIcon, statusText := getStatusDisplay(status.State)
		keyVariable := getKeyVariable(&provider, status)
		source := getCredentialSource(&provider)

		row := []string{
			provider.Name,
			statusIcon + " " + statusText,
			keyVariable,
			source,
		}
		tableRows = append(tableRows, row)

		// Count by status
		switch status.State {
		case auth.StateConfigured:
			configured++
		case auth.StateMissing:
			missing++
		case auth.StateOptional:
			optional++
		case auth.StateUnsupported:
			unsupported++
		}
	}

	// For structured output (JSON/YAML), return data only
	if outputFormat != format.FormatTable {
		tableData := format.Data{
			Headers: []string{"Provider", "Status", "Key Variable", "Source"},
			Rows:    tableRows,
		}

		formatter := format.NewFormatter(outputFormat)
		return formatter.Format(os.Stdout, tableData)
	}

	// For table output, show full UI with headers
	fmt.Println()
	fmt.Println("Provider Authentication Status:")

	// Create and display table
	tableData := format.Data{
		Headers: []string{"Provider", "Status", "Key Variable", "Source"},
		Rows:    tableRows,
	}

	formatter := format.NewFormatter(outputFormat)
	if err := formatter.Format(os.Stdout, tableData); err != nil {
		return err
	}

	// Print summary
	if err := printAuthSummary(cmd, app, verbose, configured, missing, optional, unsupported); err != nil {
		return err
	}

	if configured == 0 && missing > 0 {
		return &errors.ConfigError{
			Component: "auth",
			Message:   "no providers configured",
		}
	}

	return nil
}
