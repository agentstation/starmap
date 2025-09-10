package validate

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/notify"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// ValidationResult represents the result of validating a catalog component.
type ValidationResult struct {
	Component string
	Status    string
	Issues    string
	Details   string
}

// CatalogCmd validates the entire embedded catalog.
var CatalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Validate entire embedded catalog",
	Long: `Validate the structure and completeness of the entire embedded catalog.

This validates:
  - providers.yaml structure and required fields
  - authors.yaml structure and required fields
  - model definitions and consistency
  - cross-references between resources`,
	RunE: RunCatalog,
}

// RunCatalog validates the entire embedded catalog.
func RunCatalog(cmd *cobra.Command, args []string) error {
	// This command doesn't take positional arguments
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument: %s", args[0])
	}

	verbose, _ := cmd.Flags().GetBool("verbose")

	// Get global flags for output format
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}

	var results []ValidationResult
	var hasErrors bool

	fmt.Println("Validating catalog components...")
	fmt.Println()

	// Validate providers.yaml
	fmt.Print("Validating providers.yaml... ")
	if err := validateProvidersStructure(verbose); err != nil {
		fmt.Println("❌ Failed")
		results = append(results, ValidationResult{
			Component: "Providers",
			Status:    "❌ Failed",
			Issues:    "1",
			Details:   err.Error(),
		})
		hasErrors = true
	} else {
		fmt.Println("✅ Success")
		results = append(results, ValidationResult{
			Component: "Providers",
			Status:    "✅ Success",
			Issues:    "0",
			Details:   "Structure valid",
		})
	}

	// Validate authors.yaml
	fmt.Print("Validating authors.yaml... ")
	if err := validateAuthorsStructure(verbose); err != nil {
		fmt.Println("❌ Failed")
		results = append(results, ValidationResult{
			Component: "Authors",
			Status:    "❌ Failed",
			Issues:    "1",
			Details:   err.Error(),
		})
		hasErrors = true
	} else {
		fmt.Println("✅ Success")
		results = append(results, ValidationResult{
			Component: "Authors",
			Status:    "✅ Success",
			Issues:    "0",
			Details:   "Structure valid",
		})
	}

	// Validate model consistency
	fmt.Print("Validating model definitions... ")
	if err := validateModelConsistency(verbose); err != nil {
		fmt.Println("❌ Failed")
		results = append(results, ValidationResult{
			Component: "Models",
			Status:    "❌ Failed",
			Issues:    "1+",
			Details:   err.Error(),
		})
		hasErrors = true
	} else {
		fmt.Println("✅ Success")
		results = append(results, ValidationResult{
			Component: "Models",
			Status:    "✅ Success",
			Issues:    "0",
			Details:   "Definitions valid",
		})
	}

	// Check cross-references
	fmt.Print("Validating cross-references... ")
	if err := validateCrossReferences(verbose); err != nil {
		fmt.Println("❌ Failed")
		results = append(results, ValidationResult{
			Component: "Cross-references",
			Status:    "❌ Failed",
			Issues:    "1+",
			Details:   err.Error(),
		})
		hasErrors = true
	} else {
		fmt.Println("✅ Success")
		results = append(results, ValidationResult{
			Component: "Cross-references",
			Status:    "✅ Success",
			Issues:    "0",
			Details:   "References valid",
		})
	}

	fmt.Println()

	// Display results in table format
	outputFormat := output.DetectFormat(globalFlags.Output)
	if outputFormat == output.FormatTable {
		displayValidationTable(results, verbose)
	} else {
		formatter := output.NewFormatter(outputFormat)
		return formatter.Format(os.Stdout, results)
	}

	// Create notifier and show contextual hints
	notifier, err := notify.NewFromCommand(cmd)
	if err != nil {
		return err
	}

	// Create context for hints
	succeeded := !hasErrors
	var errorType string
	if hasErrors {
		errorType = "validation_failed"
	}
	ctx := notify.Contexts.Validation("catalog", succeeded, errorType)

	if hasErrors {
		if err := notifier.Error("Catalog validation failed", ctx); err != nil {
			return err
		}
		return fmt.Errorf("catalog validation failed")
	}

	// Success is obvious from the validation table showing all green checkmarks
	return notifier.Hints(ctx)
}

func validateCrossReferences(verbose bool) error {
	// Load catalog to check cross-references
	cat, err := catalogs.New(catalogs.WithEmbedded())
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	var errors []string

	// Check that all model authors exist
	models := cat.GetAllModels()
	authors := cat.Authors().List()
	authorMap := make(map[string]bool)
	for _, author := range authors {
		authorMap[string(author.ID)] = true
	}

	for _, model := range models {
		// Check if model has authors
		for _, author := range model.Authors {
			if !authorMap[string(author.ID)] {
				errors = append(errors, fmt.Sprintf("model %s references unknown author: %s", model.ID, author.ID))
			}
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			if verbose {
				fmt.Printf("    ❌ %s\n", err)
			}
		}
		return fmt.Errorf("found %d cross-reference errors", len(errors))
	}

	return nil
}

// displayValidationTable shows validation results in a table format.
func displayValidationTable(results []ValidationResult, verbose bool) {
	if len(results) == 0 {
		return
	}

	formatter := output.NewFormatter(output.FormatTable)

	headers := []string{"Component", "Status", "Issues"}
	if verbose {
		headers = append(headers, "Details")
	}

	rows := make([][]string, 0, len(results))
	for _, result := range results {
		row := []string{
			result.Component,
			result.Status,
			result.Issues,
		}
		if verbose {
			details := result.Details
			if len(details) > 80 {
				details = details[:77] + "..."
			}
			row = append(row, details)
		}
		rows = append(rows, row)
	}

	tableData := output.TableData{
		Headers: headers,
		Rows:    rows,
	}

	fmt.Println("Catalog Validation Results:")
	_ = formatter.Format(os.Stdout, tableData)
	fmt.Println()
}
