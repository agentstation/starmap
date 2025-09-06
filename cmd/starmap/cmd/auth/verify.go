package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// VerifyCmd verifies credentials by making test API calls.
var VerifyCmd = &cobra.Command{
	Use:   "verify [provider]",
	Short: "Verify credentials work by making test API calls",
	Long: `Test that configured API keys actually work.

Unlike 'status' which only checks if keys are set, this command
makes actual API calls to verify the credentials are valid.

Examples:
  starmap auth verify           # Verify all configured providers
  starmap auth verify openai    # Verify only OpenAI
  starmap auth verify --verbose # Show detailed verification output`,
	RunE: runAuthVerify,
}

func init() {
	VerifyCmd.Flags().BoolP("verbose", "v", false, "Show detailed verification output")
	VerifyCmd.Flags().Duration("timeout", 10*time.Second, "Timeout for API calls")
}

func runAuthVerify(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// Load catalog
	cat, err := catalogs.NewEmbedded()
	if err != nil {
		return err
	}

	// If a specific provider was requested
	if len(args) > 0 {
		providerID := args[0]
		return verifyProvider(cmd, cat, providerID, timeout, verbose)
	}

	// Verify all configured providers
	return verifyAllProviders(cmd, cat, timeout, verbose)
}

func verifyAllProviders(_ *cobra.Command, cat catalogs.Catalog, timeout time.Duration, verbose bool) error {
	fetcher := sources.NewProviderFetcher()
	supportedProviders := fetcher.List()

	fmt.Println("Verifying provider credentials...")
	fmt.Println()

	var verified, failed, skipped int

	for _, providerID := range supportedProviders {
		// Get provider from catalog
		provider, err := cat.Provider(providerID)
		if err != nil {
			continue
		}

		// Check if API key is configured
		if provider.APIKey == nil || os.Getenv(provider.APIKey.Name) == "" {
			if verbose {
				fmt.Printf("⚪ %s: No credentials configured\n", providerID)
			}
			skipped++
			continue
		}

		// Test the API
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		fmt.Printf("Testing %s... ", providerID)
		
		// Try to fetch models as a test
		_, err = fetcher.FetchModels(ctx, &provider)
		if err != nil {
			fmt.Printf("❌ Failed\n")
			if verbose {
				fmt.Printf("   Error: %v\n", err)
			}
			failed++
		} else {
			fmt.Printf("✅ Success\n")
			verified++
		}
	}

	// Summary
	fmt.Println("\nVerification Results:")
	if verified > 0 {
		fmt.Printf("  ✅ %d provider(s) verified\n", verified)
	}
	if failed > 0 {
		fmt.Printf("  ❌ %d provider(s) failed\n", failed)
	}
	if skipped > 0 {
		fmt.Printf("  ⚪ %d provider(s) skipped (no credentials)\n", skipped)
	}

	if failed > 0 {
		return fmt.Errorf("%d provider(s) failed verification", failed)
	}

	if verified > 0 {
		fmt.Println("\n✨ Credentials verified successfully!")
	} else {
		fmt.Println("\n⚠️  No providers to verify. Configure API keys first.")
	}

	return nil
}

func verifyProvider(_ *cobra.Command, cat catalogs.Catalog, providerID string, timeout time.Duration, verbose bool) error {
	fetcher := sources.NewProviderFetcher()
	
	// Convert string to ProviderID type
	pid := catalogs.ProviderID(providerID)
	
	// Check if provider is supported
	if !fetcher.HasClient(pid) {
		return fmt.Errorf("provider %s not found or not supported", providerID)
	}
	
	// Get provider from catalog
	provider, err := cat.Provider(pid)
	if err != nil {
		return fmt.Errorf("provider %s not found in catalog", providerID)
	}

	if provider.APIKey == nil || os.Getenv(provider.APIKey.Name) == "" {
		return fmt.Errorf("provider %s has no credentials configured", providerID)
	}

	fmt.Printf("Verifying %s credentials...\n", providerID)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Try to fetch models as a test
	models, err := fetcher.FetchModels(ctx, &provider)
	if err != nil {
		fmt.Printf("❌ Verification failed\n")
		if verbose {
			fmt.Printf("Error: %v\n", err)
		}
		return fmt.Errorf("failed to verify %s: %w", providerID, err)
	}

	fmt.Printf("✅ Verification successful\n")
	if verbose {
		fmt.Printf("   Found %d models\n", len(models))
	}

	return nil
}