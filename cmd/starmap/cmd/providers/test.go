package providers

import (
	"context"
	"fmt"
	"os"
	"sync"
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

// testResult represents the result of testing a provider's credentials.
type testResult struct {
	Provider     string
	Status       string
	ResponseTime string
	ModelsFound  string
	Error        string
}

// runTest executes the test command logic (called by providers --test flag).
func runTest(cmd *cobra.Command, args []string, app application.Application) error {
	// Load catalog from app context
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// If a specific provider was requested
	if len(args) > 0 {
		providerID := args[0]
		return testSingleProvider(cmd, cat, providerID, app)
	}

	// Test all configured providers
	return testAllProviders(cmd, cat, app)
}

// testAllProviders tests all configured providers.
func testAllProviders(cmd *cobra.Command, cat catalogs.Catalog, app application.Application) error {
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

	// Check if we should use silent mode (TTY + table format)
	isTTY := isatty.IsTerminal(os.Stdout.Fd()) && detectedFormat == format.FormatTable

	// Initialize results array
	results := make([]testResult, len(supportedProviders))
	for i := range results {
		results[i] = testResult{
			Provider:     string(supportedProviders[i]),
			Status:       "-",
			ResponseTime: "-",
			ModelsFound:  "-",
		}
	}

	var verified, failed, skipped int

	// For TTY mode, show simple progress message
	if isTTY {
		fmt.Println()
		fmt.Println("Testing provider credentials...")
	} else {
		// For non-TTY output, print traditional header
		fmt.Println("Testing provider credentials...")
		fmt.Println()
	}

	if isTTY {
		// TTY mode: Use concurrent testing for speed
		testProvidersConcurrent(cmd, cat, supportedProviders, fetcher, checker, supportedMap, timeout, results, &verified, &failed, &skipped)
	} else {
		// Non-TTY mode: Keep sequential for clear line-by-line output
		testProvidersSequential(cmd, cat, supportedProviders, fetcher, checker, supportedMap, timeout, results, &verified, &failed, &skipped)
	}

	// For TTY mode, clear the progress message and show final table
	if isTTY {
		// Move cursor up 1 line and clear from cursor to end of screen
		fmt.Print("\033[A\r\033[J")
		fmt.Println("Provider Test Results:")
		displayTestTableWithTitle(results, verbose, false)
	}

	// Display final results for non-TTY mode
	if !isTTY {
		fmt.Println()
		if detectedFormat == format.FormatTable {
			displayTestTable(results, verbose)
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
	notifyCtx := notify.Contexts.AuthTest(succeeded, errorType)

	// Send appropriate notification
	if failed > 0 {
		message := fmt.Sprintf("%d provider(s) failed testing", failed)
		if err := notifier.Error(message, notifyCtx); err != nil {
			return err
		}
		return fmt.Errorf("%d provider(s) failed testing", failed)
	}

	if verified > 0 {
		// Just show hints, the test results table already shows success
		return notifier.Hints(notifyCtx)
	}
	return notifier.Warning("No providers to test. Configure API keys first.", notifyCtx)
}

// apiTestWork represents a provider that passed pre-flight checks and needs API testing.
type apiTestWork struct {
	index      int
	providerID catalogs.ProviderID
	provider   catalogs.Provider
}

// apiTestResult represents the result of an API test for a provider.
type apiTestResult struct {
	index        int
	status       string
	responseTime string
	modelsFound  string
	errorMsg     string
	succeeded    bool
}

// testProvidersSequential tests providers one at a time (for non-TTY output).
func testProvidersSequential(cmd *cobra.Command, cat catalogs.Catalog, supportedProviders []catalogs.ProviderID,
	fetcher *sources.ProviderFetcher, checker *auth.Checker, supportedMap map[string]bool,
	timeout time.Duration, results []testResult, verified, failed, skipped *int) {

	for i, providerID := range supportedProviders {
		// Get provider from catalog
		provider, err := cat.Provider(providerID)
		if err != nil {
			continue
		}

		// Show individual provider status
		fmt.Printf("Testing %s... ", providerID)

		// Special handling for Google Cloud providers (use ADC)
		if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
			status := checker.CheckProvider(&provider, supportedMap)

			// Check if ADC is missing or invalid
			if status.State == auth.StateMissing {
				results[i].Status = emoji.Error + " Failed"
				results[i].Error = "ADC not configured - run 'gcloud auth application-default login'"
				*failed++
				fmt.Printf("%s Failed\n", emoji.Error)
				continue
			} else if status.State == auth.StateInvalid {
				results[i].Status = emoji.Error + " Failed"
				results[i].Error = "ADC invalid - check 'gcloud auth application-default login'"
				*failed++
				fmt.Printf("%s Failed\n", emoji.Error)
				continue
			}

			// Check if project is configured
			if os.Getenv("GOOGLE_VERTEX_PROJECT") == "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
				results[i].Status = emoji.Warning + " Skipped"
				results[i].Error = "No project configured - set GOOGLE_VERTEX_PROJECT or GOOGLE_CLOUD_PROJECT"
				*skipped++
				fmt.Printf("%s Skipped\n", emoji.Warning)
				continue
			}
		} else {
			// Check if API key is configured for non-Google Cloud providers
			if provider.APIKey == nil || os.Getenv(provider.APIKey.Name) == "" {
				results[i].Status = emoji.Optional + " Skipped"
				results[i].Error = "No credentials configured"
				*skipped++
				fmt.Printf("%s Skipped\n", emoji.Optional)
				continue
			}
		}

		// Test the API with timeout (use cmd context for signal handling)
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)

		start := time.Now()
		var models []catalogs.Model
		var fetchErr error

		// Suppress stderr to hide SDK warnings
		_ = suppressStderr(func() error {
			models, fetchErr = fetcher.FetchModels(ctx, &provider)
			return nil
		})

		duration := time.Since(start)
		cancel()

		if fetchErr != nil {
			results[i].Status = emoji.Error + " Failed"
			results[i].ResponseTime = duration.Truncate(time.Millisecond).String()
			results[i].Error = fetchErr.Error()
			*failed++
			fmt.Printf("%s Failed\n", emoji.Error)
		} else {
			results[i].Status = emoji.Success + " Success"
			results[i].ResponseTime = duration.Truncate(time.Millisecond).String()
			results[i].ModelsFound = fmt.Sprintf("%d", len(models))
			*verified++
			fmt.Printf("%s Success\n", emoji.Success)
		}
	}
}

// testProvidersConcurrent tests providers concurrently using a three-phase approach (for TTY output).
func testProvidersConcurrent(cmd *cobra.Command, cat catalogs.Catalog, supportedProviders []catalogs.ProviderID,
	fetcher *sources.ProviderFetcher, checker *auth.Checker, supportedMap map[string]bool,
	timeout time.Duration, results []testResult, verified, failed, skipped *int) {

	// Phase 1: Pre-flight checks (sequential, fast)
	// Check credentials and ADC status, build list of providers to actually test
	providersToTest := make([]apiTestWork, 0, len(supportedProviders))

	for i, providerID := range supportedProviders {
		provider, err := cat.Provider(providerID)
		if err != nil {
			continue
		}

		// Special handling for Google Cloud providers (use ADC)
		if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
			status := checker.CheckProvider(&provider, supportedMap)

			// Check if ADC is missing or invalid
			if status.State == auth.StateMissing {
				results[i].Status = emoji.Error + " Failed"
				results[i].Error = "ADC not configured - run 'gcloud auth application-default login'"
				*failed++
				continue
			} else if status.State == auth.StateInvalid {
				results[i].Status = emoji.Error + " Failed"
				results[i].Error = "ADC invalid - check 'gcloud auth application-default login'"
				*failed++
				continue
			}

			// Check if project is configured
			if os.Getenv("GOOGLE_VERTEX_PROJECT") == "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
				results[i].Status = emoji.Warning + " Skipped"
				results[i].Error = "No project configured - set GOOGLE_VERTEX_PROJECT or GOOGLE_CLOUD_PROJECT"
				*skipped++
				continue
			}
		} else {
			// Check if API key is configured for non-Google Cloud providers
			if provider.APIKey == nil || os.Getenv(provider.APIKey.Name) == "" {
				results[i].Status = emoji.Optional + " Skipped"
				results[i].Error = "No credentials configured"
				*skipped++
				continue
			}
		}

		// Provider passed pre-flight checks, add to test queue
		providersToTest = append(providersToTest, apiTestWork{
			index:      i,
			providerID: providerID,
			provider:   provider,
		})
	}

	// Phase 2: API testing (concurrent)
	// Launch goroutines to test each provider's API
	if len(providersToTest) > 0 {
		var wg sync.WaitGroup
		resultChan := make(chan apiTestResult, len(providersToTest))

		// Suppress stderr once for all concurrent operations
		_ = suppressStderr(func() error {
			for _, work := range providersToTest {
				wg.Add(1)
				go func(w apiTestWork) {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							// Handle panics gracefully
							resultChan <- apiTestResult{
								index:     w.index,
								status:    emoji.Error + " Failed",
								errorMsg:  fmt.Sprintf("panic during test: %v", r),
								succeeded: false,
							}
						}
					}()

					// Test the API with timeout
					ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
					defer cancel()

					start := time.Now()
					models, fetchErr := fetcher.FetchModels(ctx, &w.provider)
					duration := time.Since(start)

					if fetchErr != nil {
						resultChan <- apiTestResult{
							index:        w.index,
							status:       emoji.Error + " Failed",
							responseTime: duration.Truncate(time.Millisecond).String(),
							errorMsg:     fetchErr.Error(),
							succeeded:    false,
						}
					} else {
						resultChan <- apiTestResult{
							index:        w.index,
							status:       emoji.Success + " Success",
							responseTime: duration.Truncate(time.Millisecond).String(),
							modelsFound:  fmt.Sprintf("%d", len(models)),
							succeeded:    true,
						}
					}
				}(work)
			}

			// Wait for all goroutines to complete
			wg.Wait()
			return nil
		})

		close(resultChan)

		// Phase 3: Result collection (sequential)
		// Collect results from channel and update results array
		for result := range resultChan {
			results[result.index].Status = result.status
			results[result.index].ResponseTime = result.responseTime
			results[result.index].ModelsFound = result.modelsFound
			results[result.index].Error = result.errorMsg

			if result.succeeded {
				*verified++
			} else {
				*failed++
			}
		}
	}
}

// testSingleProvider tests a single provider.
func testSingleProvider(cmd *cobra.Command, cat catalogs.Catalog, providerID string, app application.Application) error {
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

	fmt.Printf("Testing %s credentials...\n", providerID)

	// Use cmd context for signal handling
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	// Try to fetch models as a test
	start := time.Now()
	models, err := fetcher.FetchModels(ctx, &provider)
	duration := time.Since(start)

	result := testResult{
		Provider:     providerID,
		ResponseTime: duration.Truncate(time.Millisecond).String(),
	}

	if err != nil {
		fmt.Printf("%s Test failed\n", emoji.Error)
		result.Status = emoji.Error + " Failed"
		result.ModelsFound = "-"
		result.Error = err.Error()

		// Display single result in configured format
		outputFormat := format.DetectFormat(app.OutputFormat())
		if outputFormat == format.FormatTable {
			displayTestTable([]testResult{result}, verbose)
		} else {
			formatter := format.NewFormatter(outputFormat)
			_ = formatter.Format(os.Stdout, []testResult{result})
		}

		return fmt.Errorf("failed to test %s: %w", providerID, err)
	}

	fmt.Printf("%s Test successful\n", emoji.Success)
	result.Status = emoji.Success + " Success"
	result.ModelsFound = fmt.Sprintf("%d", len(models))

	// Display single result in configured format
	outputFormat := format.DetectFormat(app.OutputFormat())
	if outputFormat == format.FormatTable {
		displayTestTable([]testResult{result}, verbose)
	} else {
		formatter := format.NewFormatter(outputFormat)
		return formatter.Format(os.Stdout, []testResult{result})
	}

	return nil
}

// displayTestTable shows test results in a table format.
func displayTestTable(results []testResult, verbose bool) {
	displayTestTableWithTitle(results, verbose, true)
}

// displayTestTableWithTitle shows test results with optional title.
func displayTestTableWithTitle(results []testResult, verbose bool, showTitle bool) {
	if len(results) == 0 {
		return
	}

	formatter := format.NewFormatter(format.FormatTable)

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

	tableData := format.Data{
		Headers: headers,
		Rows:    rows,
	}

	if showTitle {
		fmt.Println("Provider Test Results:")
	}
	_ = formatter.Format(os.Stdout, tableData)
	if showTitle {
		fmt.Println()
	}
}
