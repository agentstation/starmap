package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/notify"
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
  starmap providers auth verify           # Verify all configured providers
  starmap providers auth verify openai    # Verify only OpenAI
  starmap providers auth verify --verbose # Show detailed verification output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthVerify(cmd, args, app)
		},
	}

	cmd.Flags().Bool("verbose", false, "Show detailed verification output")
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
	verbose := mustGetBool(cmd, "verbose")
	timeout := mustGetDuration(cmd, "timeout")

	// Get output format from app context
	outputFormat := app.OutputFormat()
	detectedFormat := format.DetectFormat(outputFormat)

	fetcher := sources.NewProviderFetcher(cat.Providers())
	supportedProviders := fetcher.List()

	// Create auth checker for credential validation
	checker := auth.NewChecker()
	supportedMap := make(map[string]bool)
	for _, pid := range supportedProviders {
		supportedMap[string(pid)] = true
	}

	// Check if we should use dynamic table updates (TTY + table format)
	useDynamicTable := isatty.IsTerminal(os.Stdout.Fd()) && detectedFormat == format.FormatTable

	// Initialize results with pending status for all providers
	results := make([]VerificationResult, len(supportedProviders))
	for i, providerID := range supportedProviders {
		results[i] = VerificationResult{
			Provider:     string(providerID),
			Status:       "⏸️ Pending",
			ResponseTime: "-",
			ModelsFound:  "-",
		}
	}

	var verified, failed, skipped int
	var tableLines int

	// For dynamic table, print title once and then initial table
	if useDynamicTable {
		fmt.Println()
		fmt.Println("Provider Verification Results:")
		tableLines = printDynamicTable(results, verbose)
	} else {
		// For non-dynamic output, print traditional header
		fmt.Println("Verifying provider credentials...")
		fmt.Println()
	}

	// Process each provider
	for i, providerID := range supportedProviders {
		// Get provider from catalog
		provider, err := cat.Provider(providerID)
		if err != nil {
			continue
		}

		// Update status to "Testing..." if dynamic
		if useDynamicTable {
			results[i].Status = "⏳ Testing..."
			clearTableLines(tableLines)
			tableLines = printDynamicTable(results, verbose)
		} else {
			fmt.Printf("Testing %s... ", providerID)
		}

		// Special handling for Google Cloud providers (use ADC)
		if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
			status := checker.CheckProvider(&provider, supportedMap)

			// Check if ADC is missing or invalid
			if status.State == auth.StateMissing {
				results[i].Status = emoji.Error + " Failed"
				results[i].Error = "ADC not configured - run 'gcloud auth application-default login'"
				failed++
				if useDynamicTable {
					clearTableLines(tableLines)
					tableLines = printDynamicTable(results, verbose)
				} else {
					fmt.Printf("%s Failed\n", emoji.Error)
				}
				continue
			} else if status.State == auth.StateInvalid {
				results[i].Status = emoji.Error + " Failed"
				results[i].Error = "ADC invalid - check 'gcloud auth application-default login'"
				failed++
				if useDynamicTable {
					clearTableLines(tableLines)
					tableLines = printDynamicTable(results, verbose)
				} else {
					fmt.Printf("%s Failed\n", emoji.Error)
				}
				continue
			}

			// Check if project is configured
			if os.Getenv("GOOGLE_VERTEX_PROJECT") == "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
				results[i].Status = emoji.Warning + " Skipped"
				results[i].Error = "No project configured - set GOOGLE_VERTEX_PROJECT or GOOGLE_CLOUD_PROJECT"
				skipped++
				if useDynamicTable {
					clearTableLines(tableLines)
					tableLines = printDynamicTable(results, verbose)
				} else {
					fmt.Printf("%s Skipped\n", emoji.Warning)
				}
				continue
			}
		} else {
			// Check if API key is configured for non-Google Cloud providers
			if provider.APIKey == nil || os.Getenv(provider.APIKey.Name) == "" {
				results[i].Status = emoji.Optional + " Skipped"
				results[i].Error = "No credentials configured"
				skipped++
				if useDynamicTable {
					clearTableLines(tableLines)
					tableLines = printDynamicTable(results, verbose)
				} else {
					fmt.Printf("%s Skipped\n", emoji.Optional)
				}
				continue
			}
		}

		// Test the API with timeout (use cmd context for signal handling)
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()

		start := time.Now()
		var models []catalogs.Model
		var fetchErr error

		// Suppress stderr to hide SDK warnings
		_ = suppressStderr(func() error {
			models, fetchErr = fetcher.FetchModels(ctx, &provider)
			return nil
		})

		duration := time.Since(start)

		if fetchErr != nil {
			results[i].Status = emoji.Error + " Failed"
			results[i].ResponseTime = duration.Truncate(time.Millisecond).String()
			results[i].Error = fetchErr.Error()
			failed++
			if !useDynamicTable {
				fmt.Printf("%s Failed\n", emoji.Error)
			}
		} else {
			results[i].Status = emoji.Success + " Success"
			results[i].ResponseTime = duration.Truncate(time.Millisecond).String()
			results[i].ModelsFound = fmt.Sprintf("%d", len(models))
			verified++
			if !useDynamicTable {
				fmt.Printf("%s Success\n", emoji.Success)
			}
		}

		// Update table with result
		if useDynamicTable {
			clearTableLines(tableLines)
			tableLines = printDynamicTable(results, verbose)
		}
	}

	// Display final results
	if !useDynamicTable {
		fmt.Println()
		if detectedFormat == format.FormatTable {
			displayVerificationTable(results, verbose)
		} else {
			// For non-table formats, output the raw results
			formatter := format.NewFormatter(detectedFormat)
			return formatter.Format(os.Stdout, results)
		}
	}

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

func verifyProvider(cmd *cobra.Command, cat catalogs.Catalog, providerID string, app application.Application) error {
	verbose := mustGetBool(cmd, "verbose")
	timeout := mustGetDuration(cmd, "timeout")

	fetcher := sources.NewProviderFetcher(cat.Providers())

	// Convert string to ProviderID type
	pid := catalogs.ProviderID(providerID)

	// Get provider from catalog (supports aliases via Resolve)
	provider, err := cat.Provider(pid)
	if err != nil {
		return fmt.Errorf("provider %s not found in catalog", providerID)
	}

	// Check if provider is supported using canonical ID
	if !fetcher.HasClient(provider.ID) {
		return fmt.Errorf("provider %s not found or not supported", providerID)
	}

	if provider.APIKey == nil || os.Getenv(provider.APIKey.Name) == "" {
		return fmt.Errorf("provider %s has no credentials configured", providerID)
	}

	fmt.Printf("Verifying %s credentials...\n", providerID)

	// Use cmd context for signal handling
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
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
		outputFormat := format.DetectFormat(app.OutputFormat())
		if outputFormat == format.FormatTable {
			displayVerificationTable([]VerificationResult{result}, verbose)
		} else {
			formatter := format.NewFormatter(outputFormat)
			_ = formatter.Format(os.Stdout, []VerificationResult{result})
		}

		return fmt.Errorf("failed to verify %s: %w", providerID, err)
	}

	fmt.Printf("%s Verification successful\n", emoji.Success)
	result.Status = emoji.Success + " Success"
	result.ModelsFound = fmt.Sprintf("%d", len(models))

	// Display single result in configured format
	outputFormat := format.DetectFormat(app.OutputFormat())
	if outputFormat == format.FormatTable {
		displayVerificationTable([]VerificationResult{result}, verbose)
	} else {
		formatter := format.NewFormatter(outputFormat)
		return formatter.Format(os.Stdout, []VerificationResult{result})
	}

	return nil
}

// suppressStderr temporarily redirects stderr to /dev/null to suppress noisy SDK warnings.
func suppressStderr(fn func() error) error {
	// Save original stderr
	origStderr := os.Stderr

	// Open /dev/null
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		// If we can't open /dev/null, just run the function normally
		return fn()
	}
	defer devNull.Close()

	// Redirect stderr
	os.Stderr = devNull
	defer func() { os.Stderr = origStderr }()

	return fn()
}

// clearTableLines moves cursor up and clears the specified number of lines.
func clearTableLines(numLines int) {
	if numLines <= 0 {
		return
	}

	// Move cursor up numLines and clear each line
	for i := 0; i < numLines; i++ {
		fmt.Print("\033[A")    // Move cursor up one line
		fmt.Print("\033[2K")   // Clear entire line
		fmt.Print("\r")        // Move cursor to start of line
	}
}

// printDynamicTable prints the verification table without title and returns the number of lines printed.
func printDynamicTable(results []VerificationResult, verbose bool) int {
	// Print the table without title (title is printed once before dynamic updates start)
	displayVerificationTableWithTitle(results, verbose, false)

	// Count lines in the output
	// Table structure:
	// - Top border: 1 line
	// - Header row: 1 line
	// - Separator: 1 line
	// - Data rows: len(results) lines
	// - Bottom border: 1 line
	// Total: len(results) + 4
	return len(results) + 4
}
