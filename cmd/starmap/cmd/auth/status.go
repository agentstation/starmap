// Package auth provides authentication management commands.
package auth

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/cmd/notify"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const googleVertexProviderID = "google-vertex"

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
Use 'starmap auth verify' to test credentials.`,
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
	if providerID == googleVertexProviderID {
		// Google Vertex AI uses ADC - detailed table view
		printGoogleCloudStatus(checker, cat)
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
	outputFormat := output.DetectFormat(app.OutputFormat())

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

func printGoogleCloudStatus(checker *auth.Checker, cat catalogs.Catalog) {
	// Get google-vertex provider
	provider, err := cat.Provider("google-vertex")
	if err != nil {
		return // Provider not found
	}

	// Check status using local inspection
	status := checker.CheckProvider(&provider, map[string]bool{"google-vertex": true})

	fmt.Println()
	fmt.Println("Google Cloud Authentication Details:")
	fmt.Println()

	// Try to extract detailed information
	details, ok := status.Extra.(*auth.GoogleVertexDetails)
	if !ok || details == nil {
		// Fallback to simple display
		switch status.State {
		case auth.StateConfigured:
			fmt.Printf("%s %s\n", emoji.Success, status.Details)
		case auth.StateMissing:
			fmt.Printf("%s %s\n", emoji.Error, status.Details)
		case auth.StateInvalid:
			fmt.Printf("%s %s\n", emoji.Warning, status.Details)
		default:
			fmt.Printf("%s %s\n", emoji.Unknown, status.Details)
		}
		return
	}

	// Display detailed information based on state
	switch details.State {
	case auth.StateConfigured:
		// Build table rows - start with Status
		rows := [][]string{
			{"Status", emoji.Success + " Configured"},
			{"Credential Type", details.Type},
		}

		if details.Account != "" {
			rows = append(rows, []string{"Account", details.Account})
		}

		if details.Project != "" {
			rows = append(rows, []string{"Project", fmt.Sprintf("%s (%s)", details.Project, details.ProjectSource)})
		} else {
			rows = append(rows, []string{"Project", fmt.Sprintf("not set (%s)", details.ProjectSource)})
		}

		rows = append(rows, []string{"Location", fmt.Sprintf("%s (%s)", details.Location, details.LocationSource)})

		if details.UniverseDomain != "" {
			rows = append(rows, []string{"Universe Domain", details.UniverseDomain})
		}

		if details.ADCPath != "" {
			rows = append(rows, []string{"ADC Path", details.ADCPath})
		}

		if !details.LastAuth.IsZero() {
			rows = append(rows, []string{"Last Authenticated", details.LastAuth.Format("2006-01-02 15:04:05")})
		}

		// Display table
		tableData := output.Data{
			Headers: []string{"PROPERTY", "VALUE"},
			Rows:    rows,
		}

		formatter := output.NewFormatter(output.FormatTable)
		_ = formatter.Format(os.Stdout, tableData)

	case auth.StateMissing:
		rows := [][]string{
			{"Status", emoji.Error + " Missing"},
			{"Error Message", details.ErrorMessage},
		}

		tableData := output.Data{
			Headers: []string{"PROPERTY", "VALUE"},
			Rows:    rows,
		}

		formatter := output.NewFormatter(output.FormatTable)
		_ = formatter.Format(os.Stdout, tableData)

	case auth.StateInvalid:
		rows := [][]string{
			{"Status", emoji.Warning + " Invalid"},
		}

		if details.ADCPath != "" {
			rows = append(rows, []string{"ADC Path", details.ADCPath})
		}

		if details.ErrorMessage != "" {
			rows = append(rows, []string{"Error", details.ErrorMessage})
		}

		tableData := output.Data{
			Headers: []string{"PROPERTY", "VALUE"},
			Rows:    rows,
		}

		formatter := output.NewFormatter(output.FormatTable)
		_ = formatter.Format(os.Stdout, tableData)

	default:
		fmt.Printf("%s %s\n", emoji.Unknown, status.Details)
	}
}

// printAPIKeyDetails displays detailed authentication information for API key providers.
func printAPIKeyDetails(provider *catalogs.Provider, status *auth.Status) {
	fmt.Println()
	fmt.Printf("%s Authentication Details:\n", provider.Name)
	fmt.Println()

	// Build table rows based on status
	var rows [][]string

	switch status.State {
	case auth.StateConfigured:
		envValue := os.Getenv(provider.APIKey.Name)

		rows = [][]string{
			{"Status", emoji.Success + " Configured"},
			{"Variable", provider.APIKey.Name},
			{"Source", "Environment Variable"},
		}

		// Add key preview and length
		if envValue != "" {
			rows = append(rows, []string{"Key Preview", maskAPIKey(envValue)})
			rows = append(rows, []string{"Key Length", fmt.Sprintf("%d characters", len(envValue))})
		}

		// Add pattern validation if pattern exists
		if provider.APIKey.Pattern != "" && provider.APIKey.Pattern != ".*" {
			rows = append(rows, []string{"Pattern", provider.APIKey.Pattern})

			// Validate against pattern
			matched, err := regexp.MatchString(provider.APIKey.Pattern, envValue)
			if err == nil {
				if matched {
					rows = append(rows, []string{"Validation", emoji.Success + " Matches pattern"})
				} else {
					rows = append(rows, []string{"Validation", emoji.Warning + " Does not match pattern"})
				}
			}
		} else if provider.APIKey.Pattern == ".*" {
			rows = append(rows, []string{"Pattern", ".* (any value)"})
			rows = append(rows, []string{"Validation", emoji.Success + " Matches pattern"})
		}

	case auth.StateMissing:
		rows = [][]string{
			{"Status", emoji.Error + " Missing"},
			{"Variable", provider.APIKey.Name},
			{"Source", "Environment Variable"},
			{"Action Required", fmt.Sprintf("Set %s environment variable", provider.APIKey.Name)},
		}

	case auth.StateInvalid:
		envValue := os.Getenv(provider.APIKey.Name)
		rows = [][]string{
			{"Status", emoji.Warning + " Invalid"},
			{"Variable", provider.APIKey.Name},
			{"Source", "Environment Variable"},
		}

		if envValue != "" {
			rows = append(rows, []string{"Key Preview", maskAPIKey(envValue)})
			rows = append(rows, []string{"Key Length", fmt.Sprintf("%d characters", len(envValue))})
		}

		if provider.APIKey.Pattern != "" {
			rows = append(rows, []string{"Expected Pattern", provider.APIKey.Pattern})
			rows = append(rows, []string{"Validation", emoji.Warning + " Does not match pattern"})
		}

	case auth.StateOptional:
		rows = [][]string{
			{"Status", emoji.Optional + " Optional"},
			{"Variable", provider.APIKey.Name},
			{"Source", "Environment Variable"},
			{"Details", status.Details},
		}

	default:
		rows = [][]string{
			{"Status", emoji.Unknown + " Unknown"},
			{"Details", status.Details},
		}
	}

	// Display table
	tableData := output.Data{
		Headers: []string{"PROPERTY", "VALUE"},
		Rows:    rows,
	}

	formatter := output.NewFormatter(output.FormatTable)
	_ = formatter.Format(os.Stdout, tableData)
}

// maskAPIKey masks an API key for safe display, showing only first 8 and last 4 characters.
func maskAPIKey(key string) string {
	if key == "" {
		return "(empty)"
	}

	// For very short keys, just show asterisks
	if len(key) <= 12 {
		return strings.Repeat("*", len(key))
	}

	// Show first 8 characters, mask middle, show last 4
	prefix := key[:8]
	suffix := key[len(key)-4:]
	masked := strings.Repeat("*", 24) // 24 asterisks in the middle

	return prefix + masked + suffix
}

func printAuthSummary(cmd *cobra.Command, app application.Application, verbose bool, configured, missing, optional, unsupported int) error {
	fmt.Println()

	// Create summary table
	var summaryRows [][]string
	if configured > 0 {
		summaryRows = append(summaryRows, []string{emoji.Success + " Configured", fmt.Sprintf("%d", configured)})
	}
	if missing > 0 {
		summaryRows = append(summaryRows, []string{emoji.Error + " Missing", fmt.Sprintf("%d", missing)})
	}
	if optional > 0 {
		summaryRows = append(summaryRows, []string{emoji.Optional + " Optional", fmt.Sprintf("%d", optional)})
	}
	if unsupported > 0 && verbose {
		summaryRows = append(summaryRows, []string{emoji.Unsupported + " Unsupported", fmt.Sprintf("%d", unsupported)})
	}

	if len(summaryRows) > 0 {
		summaryData := output.Data{
			Headers: []string{"Status", "Count"},
			Rows:    summaryRows,
		}

		// Get output format from app context
		outputFormat := output.DetectFormat(app.OutputFormat())
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
		return emoji.Success, "Configured"
	case auth.StateMissing:
		return emoji.Error, "Missing"
	case auth.StateInvalid:
		return emoji.Warning, "Invalid"
	case auth.StateOptional:
		return emoji.Optional, "Optional"
	case auth.StateUnsupported:
		return emoji.Unsupported, "Unsupported"
	default:
		return emoji.Unknown, "Unknown"
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
		fmt.Printf("%s %s (%s)\n", emoji.Success, provider.Name, provider.ID)
		if status.Details != "" {
			fmt.Printf("   %s\n", status.Details)
		}

	case auth.StateMissing:
		fmt.Printf("%s %s (%s)\n", emoji.Error, provider.Name, provider.ID)
		fmt.Printf("   %s\n", status.Details)

	case auth.StateOptional:
		fmt.Printf("%s %s (%s)\n", emoji.Optional, provider.Name, provider.ID)
		if status.Details != "" {
			fmt.Printf("   %s\n", status.Details)
		}

	case auth.StateUnsupported:
		fmt.Printf("%s %s (%s)\n", emoji.Unsupported, provider.Name, provider.ID)
		fmt.Printf("   No client implementation\n")
	}
}
