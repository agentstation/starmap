package auth

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/auth/adc"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/notify"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// printGoogleCloudStatus displays detailed authentication information for Google Cloud providers.
func printGoogleCloudStatus(provider *catalogs.Provider, checker *auth.Checker) {
	// Check status using local inspection
	status := checker.CheckProvider(provider, map[string]bool{string(provider.ID): true})

	fmt.Println()
	fmt.Println("Google Cloud Authentication Details:")
	fmt.Println()

	// Extract Google Cloud details (type assert from interface{})
	details, ok := status.GoogleCloud.(*adc.Details)
	if !ok || details == nil {
		// Fallback to simple display
		switch status.State {
		case auth.StateConfigured:
			fmt.Printf("%s %s\n", emoji.Success, status.Summary)
		case auth.StateMissing:
			fmt.Printf("%s %s\n", emoji.Error, status.Summary)
		case auth.StateInvalid:
			fmt.Printf("%s %s\n", emoji.Warning, status.Summary)
		default:
			fmt.Printf("%s %s\n", emoji.Unknown, status.Summary)
		}
		return
	}

	// Display detailed information based on state
	switch details.State {
	case adc.StateConfigured:
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
		tableData := format.Data{
			Headers: []string{"PROPERTY", "VALUE"},
			Rows:    rows,
		}

		formatter := format.NewFormatter(format.FormatTable)
		_ = formatter.Format(os.Stdout, tableData)

	case adc.StateMissing:
		rows := [][]string{
			{"Status", emoji.Error + " Missing"},
			{"Error Message", details.ErrorMessage},
		}

		tableData := format.Data{
			Headers: []string{"PROPERTY", "VALUE"},
			Rows:    rows,
		}

		formatter := format.NewFormatter(format.FormatTable)
		_ = formatter.Format(os.Stdout, tableData)

	case adc.StateInvalid:
		rows := [][]string{
			{"Status", emoji.Warning + " Invalid"},
		}

		if details.ADCPath != "" {
			rows = append(rows, []string{"ADC Path", details.ADCPath})
		}

		if details.ErrorMessage != "" {
			rows = append(rows, []string{"Error", details.ErrorMessage})
		}

		tableData := format.Data{
			Headers: []string{"PROPERTY", "VALUE"},
			Rows:    rows,
		}

		formatter := format.NewFormatter(format.FormatTable)
		_ = formatter.Format(os.Stdout, tableData)

	default:
		fmt.Printf("%s %s\n", emoji.Unknown, status.Summary)
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
			{"Details", status.Summary},
		}

	default:
		rows = [][]string{
			{"Status", emoji.Unknown + " Unknown"},
			{"Details", status.Summary},
		}
	}

	// Display table
	tableData := format.Data{
		Headers: []string{"PROPERTY", "VALUE"},
		Rows:    rows,
	}

	formatter := format.NewFormatter(format.FormatTable)
	_ = formatter.Format(os.Stdout, tableData)
}

// printProviderStatus displays status for providers without detailed views.
func printProviderStatus(provider *catalogs.Provider, status *auth.Status) {
	switch status.State {
	case auth.StateConfigured:
		fmt.Printf("%s %s (%s)\n", emoji.Success, provider.Name, provider.ID)
		if status.Summary != "" {
			fmt.Printf("   %s\n", status.Summary)
		}

	case auth.StateMissing:
		fmt.Printf("%s %s (%s)\n", emoji.Error, provider.Name, provider.ID)
		fmt.Printf("   %s\n", status.Summary)

	case auth.StateOptional:
		fmt.Printf("%s %s (%s)\n", emoji.Optional, provider.Name, provider.ID)
		if status.Summary != "" {
			fmt.Printf("   %s\n", status.Summary)
		}

	case auth.StateUnsupported:
		fmt.Printf("%s %s (%s)\n", emoji.Unsupported, provider.Name, provider.ID)
		fmt.Printf("   No client implementation\n")
	}
}

// printAuthSummary displays contextual hints after authentication status.
func printAuthSummary(cmd *cobra.Command, configured, missing int) error {
	fmt.Println()

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

// getKeyVariable returns the key variable name or special message for display.
func getKeyVariable(provider *catalogs.Provider, status *auth.Status) string {
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
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

// getCredentialSource determines where credentials are sourced from for display.
func getCredentialSource(provider *catalogs.Provider) string {
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
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

// getKeyPreview returns a masked preview of the API key for display in tables.
func getKeyPreview(provider *catalogs.Provider, status *auth.Status) string {
	// Google Cloud providers use gcloud auth, not API keys
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return "(gcloud auth)"
	}

	// Unsupported providers
	if status.State == auth.StateUnsupported {
		return "(n/a)"
	}

	// API key providers
	if provider.APIKey != nil {
		envValue := os.Getenv(provider.APIKey.Name)
		if envValue != "" {
			return maskAPIKey(envValue)
		}
		return "(not set)"
	}

	return "-"
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

// displayTestTable shows test results in a table format.
func displayTestTable(results []TestResult, verbose bool) {
	displayTestTableWithTitle(results, verbose, true)
}

// displayTestTableWithTitle shows test results with optional title.
func displayTestTableWithTitle(results []TestResult, verbose bool, showTitle bool) {
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
