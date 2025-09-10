// Package auth provides authentication management commands.
package auth

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/notify"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const googleVertexProviderID = "google-vertex"

// StatusCmd shows authentication status for all providers.
var StatusCmd = &cobra.Command{
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
Use 'starmap auth verify' to test credentials.`,
	RunE: runAuthStatus,
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cat, err := catalogs.NewEmbedded()
	if err != nil {
		return err
	}

	// Get list of supported providers
	fetcher := sources.NewProviderFetcher()
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

	return showAllProvidersStatus(cmd, cat, checker, supportedMap)
}

func showSingleProviderStatus(providerName string, cat catalogs.Catalog, checker *auth.Checker, supportedMap map[string]bool) error {
	providerID := catalogs.ProviderID(providerName)
	provider, err := cat.Provider(providerID)
	if err != nil {
		return fmt.Errorf("provider %s not found", providerName)
	}

	status := checker.CheckProvider(&provider, supportedMap)
	printProviderStatus(&provider, status)

	// Check Google Cloud if it's the vertex provider
	if providerID == googleVertexProviderID {
		printGoogleCloudStatus(checker, cat)
	}
	return nil
}

func showAllProvidersStatus(cmd *cobra.Command, cat catalogs.Catalog, checker *auth.Checker, supportedMap map[string]bool) error {
	// Get global flags for output format
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}

	outputFormat := output.DetectFormat(globalFlags.Output)

	var configured, missing, optional, unsupported int
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Group providers by status and collect data
	providers := cat.Providers().List()

	// Prepare table data
	tableRows := make([][]string, 0, len(providers))
	for _, provider := range providers {
		status := checker.CheckProvider(provider, supportedMap)

		// Skip unsupported unless verbose
		if status.State == auth.StateUnsupported && !verbose {
			unsupported++
			continue
		}

		// Create table row
		statusIcon, statusText := getStatusDisplay(status.State)
		keyVariable := getKeyVariable(provider, status)
		source := getCredentialSource(provider)

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
	if outputFormat != output.FormatTable {
		tableData := output.Data{
			Headers: []string{"Provider", "Status", "Key Variable", "Source"},
			Rows:    tableRows,
		}

		formatter := output.NewFormatter(outputFormat)
		return formatter.Format(os.Stdout, tableData)
	}

	// For table output, show full UI with headers
	fmt.Println()
	fmt.Println("Provider Authentication Status:")

	// Create and display table
	tableData := output.Data{
		Headers: []string{"Provider", "Status", "Key Variable", "Source"},
		Rows:    tableRows,
	}

	formatter := output.NewFormatter(outputFormat)
	if err := formatter.Format(os.Stdout, tableData); err != nil {
		return err
	}

	// Special section for Google Cloud authentication - only if relevant
	gcloudStatus := checker.CheckGCloud(cat)
	if gcloudStatus != nil && (gcloudStatus.HasVertexProvider || gcloudStatus.Authenticated) {
		printGoogleCloudStatus(checker, cat)
	}

	// Print summary
	if err := printAuthSummary(cmd, configured, missing, optional, unsupported); err != nil {
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

func printGoogleCloudStatus(checker *auth.Checker, cat catalogs.Catalog) {
	fmt.Println()
	fmt.Println("Google Cloud Authentication:")
	gcloudStatus := checker.CheckGCloud(cat)
	if gcloudStatus.Authenticated {
		fmt.Println("✅ Application Default Credentials configured")
		if gcloudStatus.Project != "" {
			fmt.Printf("   Project: %s\n", gcloudStatus.Project)
		}
		if gcloudStatus.Location != "" {
			fmt.Printf("   Location: %s\n", gcloudStatus.Location)
		}
	} else {
		fmt.Println("❌ Not authenticated")
		fmt.Println("   Run: starmap auth gcloud")
	}
}

func printAuthSummary(cmd *cobra.Command, configured, missing, optional, unsupported int) error {
	fmt.Println()

	// Create summary table
	var summaryRows [][]string
	if configured > 0 {
		summaryRows = append(summaryRows, []string{"✅ Configured", fmt.Sprintf("%d", configured)})
	}
	if missing > 0 {
		summaryRows = append(summaryRows, []string{"❌ Missing", fmt.Sprintf("%d", missing)})
	}
	if optional > 0 {
		summaryRows = append(summaryRows, []string{"⚪ Optional", fmt.Sprintf("%d", optional)})
	}
	if unsupported > 0 && cmd.Flags().Changed("verbose") {
		summaryRows = append(summaryRows, []string{"⚫ Unsupported", fmt.Sprintf("%d", unsupported)})
	}

	if len(summaryRows) > 0 {
		summaryData := output.Data{
			Headers: []string{"Status", "Count"},
			Rows:    summaryRows,
		}

		// Get global flags for output format
		globalFlags, err := globals.Parse(cmd)
		if err != nil {
			return err
		}

		outputFormat := output.DetectFormat(globalFlags.Output)
		formatter := output.NewFormatter(outputFormat)
		if err := formatter.Format(os.Stdout, summaryData); err != nil {
			return err
		}
		fmt.Println()
	}

	// Create notifier and show contextual hints
	notifier, err := notify.NewFromCommand(cmd)
	if err != nil {
		return err
	}

	// Determine success and create context
	succeeded := configured > 0 || missing == 0
	ctx := notify.Contexts.AuthStatus(succeeded, configured)

	if configured == 0 && missing > 0 {
		return notifier.Warning("No providers configured. Set API keys to enable provider access.", ctx)
	} else if configured > 0 {
		// Just show hints, no redundant success message
		return notifier.Hints(ctx)
	}

	return nil
}

// getStatusDisplay returns icon and text for a status state.
func getStatusDisplay(state auth.State) (string, string) {
	switch state {
	case auth.StateConfigured:
		return "✅", "Configured"
	case auth.StateMissing:
		return "❌", "Missing"
	case auth.StateOptional:
		return "⚪", "Optional"
	case auth.StateUnsupported:
		return "⚫", "Unsupported"
	default:
		return "❓", "Unknown"
	}
}

// getKeyVariable returns the key variable name or special message.
func getKeyVariable(provider *catalogs.Provider, status *auth.Status) string {
	if provider.ID == googleVertexProviderID {
		return "(gcloud auth required)"
	}

	if provider.APIKey != nil {
		return provider.APIKey.Name
	}

	if status.State == auth.StateUnsupported {
		return "(no implementation)"
	}

	return "(no key required)"
}

// getCredentialSource determines where credentials are sourced from.
func getCredentialSource(provider *catalogs.Provider) string {
	if provider.ID == googleVertexProviderID {
		return "gcloud"
	}

	if provider.APIKey != nil {
		// Check if environment variable is set
		envValue := os.Getenv(provider.APIKey.Name)
		if envValue != "" {
			return "env"
		}
		return "-"
	}

	return "-"
}

func printProviderStatus(provider *catalogs.Provider, status *auth.Status) {
	switch status.State {
	case auth.StateConfigured:
		fmt.Printf("✅ %s (%s)\n", provider.Name, provider.ID)
		if status.Details != "" {
			fmt.Printf("   %s\n", status.Details)
		}

	case auth.StateMissing:
		fmt.Printf("❌ %s (%s)\n", provider.Name, provider.ID)
		fmt.Printf("   %s\n", status.Details)

	case auth.StateOptional:
		fmt.Printf("⚪ %s (%s)\n", provider.Name, provider.ID)
		if status.Details != "" {
			fmt.Printf("   %s\n", status.Details)
		}

	case auth.StateUnsupported:
		fmt.Printf("⚫ %s (%s)\n", provider.Name, provider.ID)
		fmt.Printf("   No client implementation\n")
	}
}
