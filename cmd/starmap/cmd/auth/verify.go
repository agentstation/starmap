package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/cmd/notify"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// VerificationResult represents the result of verifying a provider's credentials.
type VerificationResult struct {
	Provider     string
	Status       string
	ResponseTime string
	ModelsFound  string
	Error        string
}

// NewVerifyCommand creates the auth verify subcommand using app context.
func NewVerifyCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [provider]",
		Short: "Verify credentials work by making test API calls",
		Long: `Test that configured API keys actually work.

Unlike 'status' which only checks if keys are set, this command
makes actual API calls to verify the credentials are valid.

Examples:
  starmap auth verify           # Verify all configured providers
  starmap auth verify openai    # Verify only OpenAI
  starmap auth verify --verbose # Show detailed verification output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthVerify(cmd, args, app)
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show detailed verification output")
	cmd.Flags().Duration("timeout", 10*time.Second, "Timeout for API calls")

	return cmd
}

func runAuthVerify(cmd *cobra.Command, args []string, app application.Application) error {
	// Load catalog from app context
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// If a specific provider was requested
	if len(args) > 0 {
		providerID := args[0]
		return verifyProvider(cmd, cat, providerID, app)
	}

	// Verify all configured providers
	return verifyAllProviders(cmd, cat, app)
}

func verifyAllProviders(cmd *cobra.Command, cat catalogs.Catalog, app application.Application) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// Get output format from app context
	outputFormat := app.OutputFormat()

	fetcher := sources.NewProviderFetcher(cat.Providers())
	supportedProviders := fetcher.List()

	fmt.Println("Verifying provider credentials...")
	fmt.Println()

	results := make([]VerificationResult, 0, len(supportedProviders))
	var verified, failed, skipped int

	for _, providerID := range supportedProviders {
		// Get provider from catalog
		provider, err := cat.Provider(providerID)
		if err != nil {
			continue
		}

		result := VerificationResult{
			Provider: string(providerID),
		}

		// Check if API key is configured
		if provider.APIKey == nil || os.Getenv(provider.APIKey.Name) == "" {
			result.Status = emoji.Optional + " Skipped"
			result.ResponseTime = "-"
			result.ModelsFound = "-"
			result.Error = "No credentials configured"
			results = append(results, result)
			skipped++
			continue
		}

		// Test the API
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		fmt.Printf("Testing %s... ", providerID)

		start := time.Now()
		models, err := fetcher.FetchModels(ctx, &provider)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("%s Failed\n", emoji.Error)
			result.Status = emoji.Error + " Failed"
			result.ResponseTime = duration.Truncate(time.Millisecond).String()
			result.ModelsFound = "-"
			result.Error = err.Error()
			failed++
		} else {
			fmt.Printf("%s Success\n", emoji.Success)
			result.Status = emoji.Success + " Success"
			result.ResponseTime = duration.Truncate(time.Millisecond).String()
			result.ModelsFound = fmt.Sprintf("%d", len(models))
			verified++
		}

		results = append(results, result)
	}

	fmt.Println()

	// Display results in configured format
	detectedFormat := output.DetectFormat(outputFormat)
	if detectedFormat == output.FormatTable {
		displayVerificationTable(results, verbose)
	} else {
		// For non-table formats, output the raw results
		formatter := output.NewFormatter(detectedFormat)
		return formatter.Format(os.Stdout, results)
	}

	// Summary table
	displaySummaryTable(verified, failed, skipped)

	// Create notifier and show contextual hints
	notifier, err := notify.NewFromCommand(cmd)
	if err != nil {
		return err
	}

	// Create notification context for hints
	succeeded := failed == 0
	var errorType string
	if failed > 0 {
		errorType = "auth_failed"
	}
	notifyCtx := notify.Contexts.AuthVerify(succeeded, errorType)

	// Send appropriate notification
	if failed > 0 {
		message := fmt.Sprintf("%d provider(s) failed verification", failed)
		if err := notifier.Error(message, notifyCtx); err != nil {
			return err
		}
		return fmt.Errorf("%d provider(s) failed verification", failed)
	}

	if verified > 0 {
		// Just show hints, the verification table already shows success
		return notifier.Hints(notifyCtx)
	}
	return notifier.Warning("No providers to verify. Configure API keys first.", notifyCtx)
}

// displayVerificationTable shows verification results in a table format.
func displayVerificationTable(results []VerificationResult, verbose bool) {
	if len(results) == 0 {
		return
	}

	formatter := output.NewFormatter(output.FormatTable)

	// Prepare table data
	headers := []string{"Provider", "Status", "Response Time", "Models"}
	if verbose {
		headers = append(headers, "Error")
	}

	rows := make([][]string, 0, len(results))
	for _, result := range results {
		row := []string{
			result.Provider,
			result.Status,
			result.ResponseTime,
			result.ModelsFound,
		}
		if verbose {
			errorMsg := result.Error
			if errorMsg == "" {
				errorMsg = "-"
			}
			row = append(row, errorMsg)
		}
		rows = append(rows, row)
	}

	tableData := output.Data{
		Headers: headers,
		Rows:    rows,
	}

	fmt.Println("Provider Verification Results:")
	_ = formatter.Format(os.Stdout, tableData)
	fmt.Println()
}

// displaySummaryTable shows a summary of verification results.
func displaySummaryTable(verified, failed, skipped int) {
	formatter := output.NewFormatter(output.FormatTable)

	headers := []string{"Status", "Count"}
	rows := [][]string{}

	if verified > 0 {
		rows = append(rows, []string{emoji.Success + " Verified", fmt.Sprintf("%d", verified)})
	}
	if failed > 0 {
		rows = append(rows, []string{emoji.Error + " Failed", fmt.Sprintf("%d", failed)})
	}
	if skipped > 0 {
		rows = append(rows, []string{emoji.Optional + " Skipped", fmt.Sprintf("%d", skipped)})
	}

	if len(rows) > 0 {
		tableData := output.Data{
			Headers: headers,
			Rows:    rows,
		}

		fmt.Println("Summary:")
		_ = formatter.Format(os.Stdout, tableData)
		fmt.Println()
	}
}

func verifyProvider(cmd *cobra.Command, cat catalogs.Catalog, providerID string, app application.Application) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	fetcher := sources.NewProviderFetcher(cat.Providers())

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
	start := time.Now()
	models, err := fetcher.FetchModels(ctx, &provider)
	duration := time.Since(start)

	result := VerificationResult{
		Provider:     providerID,
		ResponseTime: duration.Truncate(time.Millisecond).String(),
	}

	if err != nil {
		fmt.Printf("%s Verification failed\n", emoji.Error)
		result.Status = emoji.Error + " Failed"
		result.ModelsFound = "-"
		result.Error = err.Error()

		// Display single result in configured format
		outputFormat := output.DetectFormat(app.OutputFormat())
		if outputFormat == output.FormatTable {
			displayVerificationTable([]VerificationResult{result}, verbose)
		} else {
			formatter := output.NewFormatter(outputFormat)
			_ = formatter.Format(os.Stdout, []VerificationResult{result})
		}

		return fmt.Errorf("failed to verify %s: %w", providerID, err)
	}

	fmt.Printf("%s Verification successful\n", emoji.Success)
	result.Status = emoji.Success + " Success"
	result.ModelsFound = fmt.Sprintf("%d", len(models))

	// Display single result in configured format
	outputFormat := output.DetectFormat(app.OutputFormat())
	if outputFormat == output.FormatTable {
		displayVerificationTable([]VerificationResult{result}, verbose)
	} else {
		formatter := output.NewFormatter(outputFormat)
		return formatter.Format(os.Stdout, []VerificationResult{result})
	}

	return nil
}
