// Package auth provides authentication management commands.
package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

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
	if providerID == "google-vertex" {
		printGoogleCloudStatus(checker)
	}
	return nil
}

func showAllProvidersStatus(cmd *cobra.Command, cat catalogs.Catalog, checker *auth.Checker, supportedMap map[string]bool) error {
	fmt.Println("Provider Authentication Status:")
	fmt.Println()

	var configured, missing, optional, unsupported int
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Group providers by status
	providers := cat.Providers().List()
	for _, provider := range providers {
		status := checker.CheckProvider(provider, supportedMap)

		switch status.State {
		case auth.StateConfigured:
			printProviderStatus(provider, status)
			configured++
		case auth.StateMissing:
			printProviderStatus(provider, status)
			missing++
		case auth.StateOptional:
			printProviderStatus(provider, status)
			optional++
		case auth.StateUnsupported:
			if verbose {
				printProviderStatus(provider, status)
			}
			unsupported++
		}
	}

	// Special section for Google Cloud authentication
	gcloudStatus := checker.CheckGCloud()
	if gcloudStatus != nil && (gcloudStatus.HasVertexProvider || gcloudStatus.Authenticated) {
		printGoogleCloudStatus(checker)
	}

	// Print summary
	printAuthSummary(cmd, configured, missing, optional, unsupported)

	if configured == 0 && missing > 0 {
		return &errors.ConfigError{
			Component: "auth",
			Message:   "no providers configured",
		}
	}

	return nil
}

func printGoogleCloudStatus(checker *auth.Checker) {
	fmt.Println("\nGoogle Cloud Authentication:")
	gcloudStatus := checker.CheckGCloud()
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

func printAuthSummary(cmd *cobra.Command, configured, missing, optional, unsupported int) {
	fmt.Println("\nSummary:")
	if configured > 0 {
		fmt.Printf("  ✅ %d provider(s) configured\n", configured)
	}
	if missing > 0 {
		fmt.Printf("  ❌ %d provider(s) missing required credentials\n", missing)
	}
	if optional > 0 {
		fmt.Printf("  ⚪ %d provider(s) with optional configuration\n", optional)
	}
	if unsupported > 0 && cmd.Flags().Changed("verbose") {
		fmt.Printf("  ⚫ %d provider(s) without implementation\n", unsupported)
	}

	if configured == 0 && missing > 0 {
		fmt.Println("\n⚠️  No providers configured. Set API keys to enable provider access.")
	} else if configured > 0 {
		fmt.Println("\n✨ Run 'starmap auth verify' to test that credentials work.")
	}
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
