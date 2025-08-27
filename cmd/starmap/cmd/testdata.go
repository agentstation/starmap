package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/spf13/cobra"
)

var (
	testdataProvider string
	testdataUpdate   bool
	testdataVerify   bool
	testdataVerbose  bool
)

// testdataCmd represents the testdata command
var testdataCmd = &cobra.Command{
	Use:   "testdata",
	Short: "Manage test data files for provider API responses",
	Long: `The testdata command helps manage test fixture files for provider API responses.
It can fetch live data from provider APIs and save it as testdata files for use in tests.

This command is useful for:
- Updating testdata files with latest API responses
- Verifying testdata files are current
- Generating testdata for new providers

Testdata files are stored in internal/sources/providers/<provider>/testdata/ and
are checked into version control to enable offline testing.`,
	Example: `  starmap testdata
  starmap testdata --update
  starmap testdata --provider groq --update
  starmap testdata --verify
  starmap testdata --provider openai --verbose`,
	RunE: runTestdata,
}

func init() {
	rootCmd.AddCommand(testdataCmd)

	testdataCmd.Flags().StringVarP(&testdataProvider, "provider", "p", "", "Provider to update testdata for (default: all providers)")
	testdataCmd.Flags().BoolVar(&testdataUpdate, "update", false, "Update testdata files with fresh API responses")
	testdataCmd.Flags().BoolVar(&testdataVerify, "verify", false, "Verify testdata files exist and are valid")
	testdataCmd.Flags().BoolVarP(&testdataVerbose, "verbose", "v", false, "Verbose output")
}

func runTestdata(cmd *cobra.Command, args []string) error {
	// Show help if no flags are provided
	if !testdataUpdate && !testdataVerify && testdataProvider == "" {
		return cmd.Help()
	}

	if testdataVerify && testdataUpdate {
		return fmt.Errorf("--verify and --update flags are mutually exclusive")
	}

	// Get catalog
	catalog, err := catalogs.New(catalogs.WithEmbedded())
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	// Get all providers from catalog
	allProviders := catalog.Providers().List()

	// Filter by specific provider if requested
	if testdataProvider != "" {
		filtered := []*catalogs.Provider{}
		for _, p := range allProviders {
			if string(p.ID) == testdataProvider {
				filtered = append(filtered, p)
				break
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("provider %s not found", testdataProvider)
		}
		allProviders = filtered
	}

	// Filter by API key availability
	var providers []catalogs.Provider
	for _, p := range allProviders {
		if p.HasAPIKey() {
			providers = append(providers, *p)
		}
	}
	if len(providers) == 0 {
		return fmt.Errorf("no API keys found for requested provider(s)")
	}

	if testdataVerbose {
		providerNames := make([]string, len(providers))
		for i, provider := range providers {
			providerNames[i] = string(provider.ID)
		}
		fmt.Printf("Processing %d provider(s): %s\n", len(providers), strings.Join(providerNames, ", "))
	}

	// Process each provider
	var errs error
	for _, provider := range providers {
		var err error
		switch {
		case testdataVerify:
			err = verifyProviderTestdata(provider)
		case testdataUpdate:
			err = updateProviderTestdata(provider)
		default:
			err = listProviderTestdata(provider)
		}

		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", provider.ID, err))
			if testdataVerbose {
				fmt.Printf("❌ Error processing %s: %v\n", provider.ID, err)
			}
		} else {
			if testdataVerbose {
				fmt.Printf("✅ Successfully processed %s\n", provider.ID)
			}
		}
	}

	if errs != nil {
		return fmt.Errorf("testdata processing completed with errors:\n%w", errs)
	}

	fmt.Printf("✅ Successfully processed testdata for %d provider(s)\n", len(providers))
	return nil
}

func verifyProviderTestdata(provider catalogs.Provider) error {
	testdataDir := getProviderTestdataDir(provider.ID)

	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		return fmt.Errorf("testdata directory does not exist: %s", testdataDir)
	}

	// Check for required files
	requiredFiles := []string{"models_list.json"}

	for _, file := range requiredFiles {
		filePath := filepath.Join(testdataDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("required testdata file missing: %s", filePath)
		}

		// Validate JSON
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		var jsonData any
		if err := json.Unmarshal(data, &jsonData); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", filePath, err)
		}
	}

	if testdataVerbose {
		fmt.Printf("✅ Testdata for %s is valid\n", provider.ID)
	}
	return nil
}

func updateProviderTestdata(provider catalogs.Provider) error {
	// Create testdata directory
	testdataDir := getProviderTestdataDir(provider.ID)
	if err := os.MkdirAll(testdataDir, 0755); err != nil {
		return fmt.Errorf("creating testdata directory: %w", err)
	}

	// Get API URL from provider configuration
	if provider.Catalog == nil || provider.Catalog.APIURL == nil {
		return fmt.Errorf("provider %s has no catalog API URL configured", provider.ID)
	}

	baseURL := *provider.Catalog.APIURL
	return fetchAndSaveRawResponse(baseURL, &provider, "models_list.json", testdataDir)
}

func listProviderTestdata(provider catalogs.Provider) error {
	testdataDir := getProviderTestdataDir(provider.ID)

	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		fmt.Printf("No testdata directory for %s\n", provider.ID)
		return nil
	}

	files, err := os.ReadDir(testdataDir)
	if err != nil {
		return fmt.Errorf("reading testdata directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Printf("No testdata files for %s\n", provider.ID)
		return nil
	}

	fmt.Printf("Testdata files for %s:\n", provider.ID)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fmt.Printf("  • %s\n", file.Name())
	}

	return nil
}

func getProviderTestdataDir(pid catalogs.ProviderID) string {
	return filepath.Join("internal", "sources", "providers", string(pid), "testdata")
}

// Helper function to make raw API requests and save responses
func fetchAndSaveRawResponse(url string, provider *catalogs.Provider, filename string, testdataDir string) error {
	// Create transport client configured for this provider
	transportClient := transport.NewForProvider(provider)

	// Make the raw request
	ctx := context.Background()
	resp, err := transportClient.Get(ctx, url, provider)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	// Read raw response body
	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	// Validate it's valid JSON and format it nicely
	var jsonData any
	if err := json.Unmarshal(rawData, &jsonData); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}

	// Re-marshal with proper formatting
	formattedData, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return fmt.Errorf("formatting JSON: %w", err)
	}

	// Save to file
	filePath := filepath.Join(testdataDir, filename)
	if err := os.WriteFile(filePath, formattedData, 0644); err != nil {
		return fmt.Errorf("writing file %s: %w", filePath, err)
	}

	if testdataVerbose {
		fmt.Printf("📝 Saved raw API response to %s\n", filePath)
	}
	return nil
}
