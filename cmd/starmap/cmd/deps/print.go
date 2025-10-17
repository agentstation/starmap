package deps

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/output"
)

// displayResults shows dependency check results in the requested format.
func displayResults(results *CheckResults, flags *globals.Flags, cmd *cobra.Command, _ application.Application) error {
	format := output.DetectFormat(flags.Output)
	formatter := output.NewFormatter(format)

	// For structured output (JSON/YAML), return the entire results object
	if format == output.FormatJSON || format == output.FormatYAML {
		return formatter.Format(os.Stdout, results)
	}

	// For table output, use custom display
	return displayTableResults(results, cmd, formatter)
}

// displayTableResults shows results in two-tier table format.
func displayTableResults(results *CheckResults, cmd *cobra.Command, _ output.Formatter) error {
	// Show status message first
	if err := displayStatusMessage(cmd, results); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Dependency Status Summary:")
	fmt.Println()

	// Display summary table
	if err := displaySummaryTable(results); err != nil {
		return err
	}

	// Display per-source detail tables
	displaySourceDetails(results)

	return nil
}

// displaySummaryTable shows high-level source status overview.
func displaySummaryTable(results *CheckResults) error {
	rows := buildSummaryTableRows(results)

	tableData := output.Data{
		Headers: []string{"Source", "Status", "Dependencies"},
		Rows:    rows,
	}

	formatter := output.NewFormatter(output.FormatTable)
	return formatter.Format(os.Stdout, tableData)
}

// buildSummaryTableRows creates summary table rows for all sources.
func buildSummaryTableRows(results *CheckResults) [][]string {
	rows := make([][]string, 0, len(results.Sources))

	for _, sourceStatus := range results.Sources {
		status := formatSourceStatus(sourceStatus)
		depsCount := formatDependencyCount(sourceStatus)

		rows = append(rows, []string{
			sourceStatus.SourceName,
			status,
			depsCount,
		})
	}

	return rows
}

// formatSourceStatus returns status string with emoji for a source.
func formatSourceStatus(sourceStatus SourceDepStatus) string {
	if len(sourceStatus.Dependencies) == 0 {
		return emoji.Success + " Ready"
	}

	availableCount := 0
	for _, dep := range sourceStatus.Dependencies {
		if dep.Status.Available {
			availableCount++
		}
	}

	if availableCount == len(sourceStatus.Dependencies) {
		return emoji.Success + " Ready"
	}

	if sourceStatus.IsOptional {
		return emoji.Warning + " Issues"
	}

	return emoji.Error + " Blocked"
}

// formatDependencyCount returns human-readable dependency count.
func formatDependencyCount(sourceStatus SourceDepStatus) string {
	if len(sourceStatus.Dependencies) == 0 {
		return "None"
	}

	availableCount := 0
	for _, dep := range sourceStatus.Dependencies {
		if dep.Status.Available {
			availableCount++
		}
	}

	return fmt.Sprintf("%d of %d", availableCount, len(sourceStatus.Dependencies))
}

// displaySourceDetails shows per-source dependency tables.
func displaySourceDetails(results *CheckResults) {
	hasDetails := false

	for _, sourceStatus := range results.Sources {
		// Skip sources with no dependencies
		if len(sourceStatus.Dependencies) == 0 {
			continue
		}

		if !hasDetails {
			fmt.Println()
			fmt.Println("Source Details:")
			fmt.Println()
			hasDetails = true
		}

		// Display source name
		fmt.Printf("%s:\n", sourceStatus.SourceName)

		// Build dependency table for this source
		rows := buildSourceDependencyRows(sourceStatus)
		tableData := output.Data{
			Headers: []string{"Dependency", "Status", "Version", "Path", "Purpose"},
			Rows:    rows,
		}

		formatter := output.NewFormatter(output.FormatTable)
		_ = formatter.Format(os.Stdout, tableData)

		// Show alternative source if available
		displayAlternativeSource(sourceStatus)

		// Show installation instructions for missing dependencies
		displayMissingDependencyInfo(sourceStatus)

		fmt.Println()
	}
}

// buildSourceDependencyRows creates table rows for a single source's dependencies.
func buildSourceDependencyRows(sourceStatus SourceDepStatus) [][]string {
	rows := make([][]string, 0, len(sourceStatus.Dependencies))

	for _, dep := range sourceStatus.Dependencies {
		statusIcon := emoji.Success
		statusText := "Available"
		if !dep.Status.Available {
			statusIcon = emoji.Error
			statusText = "Missing"
		}

		version := "-"
		if dep.Status.Version != "" {
			version = dep.Status.Version
		}

		path := "-"
		if dep.Status.Path != "" {
			path = dep.Status.Path
		}

		purpose := "-"
		if dep.Dependency.WhyNeeded != "" {
			purpose = dep.Dependency.WhyNeeded
		}

		rows = append(rows, []string{
			dep.Dependency.DisplayName,
			statusIcon + " " + statusText,
			version,
			path,
			purpose,
		})
	}

	return rows
}

// displayAlternativeSource shows alternative source information if dependencies are missing.
func displayAlternativeSource(sourceStatus SourceDepStatus) {
	if len(sourceStatus.Dependencies) == 0 {
		return
	}

	// Check if any dependencies are missing
	hasMissing := false
	for _, dep := range sourceStatus.Dependencies {
		if !dep.Status.Available {
			hasMissing = true
			break
		}
	}

	// Only show alternative if there's a problem to solve
	if !hasMissing {
		return
	}

	// All dependencies in a source share the same alternative
	dep := sourceStatus.Dependencies[0].Dependency
	if dep.AlternativeSource != "" {
		fmt.Println()
		fmt.Printf("Alternative: %s\n", dep.AlternativeSource)
	}
}

// displayMissingDependencyInfo shows installation instructions for missing dependencies.
func displayMissingDependencyInfo(sourceStatus SourceDepStatus) {
	for _, dep := range sourceStatus.Dependencies {
		if !dep.Status.Available {
			fmt.Println()
			fmt.Printf("Missing Dependency: %s\n", dep.Dependency.DisplayName)

			if dep.Dependency.Description != "" {
				fmt.Printf("  Description: %s\n", dep.Dependency.Description)
			}

			if dep.Dependency.InstallURL != "" {
				fmt.Printf("  Install: %s\n", dep.Dependency.InstallURL)
			}

			if dep.Dependency.AutoInstallCommand != "" {
				fmt.Printf("  Auto-install available: Use --auto-install-deps flag\n")
			}
		}
	}
}

// displayStatusMessage shows status message at the beginning of output.
func displayStatusMessage(_ *cobra.Command, results *CheckResults) error {
	fmt.Println()

	// Show appropriate message
	if results.MissingDeps > 0 {
		// Check if any missing are from required sources
		hasRequiredMissing := false
		for _, srcStatus := range results.Sources {
			if !srcStatus.IsOptional {
				for _, dep := range srcStatus.Dependencies {
					if !dep.Status.Available {
						hasRequiredMissing = true
						break
					}
				}
			}
			if hasRequiredMissing {
				break
			}
		}

		if hasRequiredMissing {
			fmt.Println(emoji.Error + " Required dependencies are missing. Install them or the sync will fail.")
		} else {
			fmt.Println(emoji.Warning + " Some optional dependencies are missing. Sources will be skipped if you use --skip-dep-prompts.")
		}
	} else {
		fmt.Println(emoji.Success + " All required dependencies are available.")
	}

	return nil
}
