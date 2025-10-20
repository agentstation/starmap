package validate

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/notify"
)

// ValidationResult represents the result of validating a catalog component.
type ValidationResult struct {
	Component string
	Status    string
	Issues    string
	Details   string
}

// NewCatalogCommand creates the validate catalog subcommand using app context.
func NewCatalogCommand(app application.Application) *cobra.Command {
	return &cobra.Command{
		Use:   "catalog",
		Short: "Validate entire embedded catalog",
		Long: `Validate the structure and completeness of the entire embedded catalog.

This validates:
  - providers.yaml structure and required fields
  - authors.yaml structure and required fields
  - model definitions and consistency
  - cross-references between resources`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCatalog(cmd, args, app)
		},
	}
}

// runCatalog validates the entire embedded catalog.
func runCatalog(cmd *cobra.Command, args []string, app application.Application) error {
	// This command doesn't take positional arguments
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument: %s", args[0])
	}

	logger := app.Logger()
	verbose := logger.GetLevel() <= zerolog.InfoLevel

	var results []ValidationResult
	var hasErrors bool

	fmt.Println("Validating catalog components...")
	fmt.Println()

	// Validate providers.yaml
	fmt.Print("Validating providers.yaml... ")
	if err := validateProvidersStructure(app, verbose); err != nil {
		fmt.Printf("%s Failed\n", emoji.Error)
		results = append(results, ValidationResult{
			Component: "Providers",
			Status:    emoji.Error + " Failed",
			Issues:    "1",
			Details:   err.Error(),
		})
		hasErrors = true
	} else {
		fmt.Printf("%s Success\n", emoji.Success)
		results = append(results, ValidationResult{
			Component: "Providers",
			Status:    emoji.Success + " Success",
			Issues:    "0",
			Details:   "Structure valid",
		})
	}

	// Validate authors.yaml
	fmt.Print("Validating authors.yaml... ")
	if err := validateAuthorsStructure(app, verbose); err != nil {
		fmt.Printf("%s Failed\n", emoji.Error)
		results = append(results, ValidationResult{
			Component: "Authors",
			Status:    emoji.Error + " Failed",
			Issues:    "1",
			Details:   err.Error(),
		})
		hasErrors = true
	} else {
		fmt.Printf("%s Success\n", emoji.Success)
		results = append(results, ValidationResult{
			Component: "Authors",
			Status:    emoji.Success + " Success",
			Issues:    "0",
			Details:   "Structure valid",
		})
	}

	// Validate model consistency
	fmt.Print("Validating model definitions... ")
	if err := validateModelConsistency(app, verbose); err != nil {
		fmt.Printf("%s Failed\n", emoji.Error)
		results = append(results, ValidationResult{
			Component: "Models",
			Status:    emoji.Error + " Failed",
			Issues:    "1+",
			Details:   err.Error(),
		})
		hasErrors = true
	} else {
		fmt.Printf("%s Success\n", emoji.Success)
		results = append(results, ValidationResult{
			Component: "Models",
			Status:    emoji.Success + " Success",
			Issues:    "0",
			Details:   "Definitions valid",
		})
	}

	// Check cross-references
	fmt.Print("Validating cross-references... ")
	if err := validateCrossReferences(app, verbose); err != nil {
		fmt.Printf("%s Failed\n", emoji.Error)
		results = append(results, ValidationResult{
			Component: "Cross-references",
			Status:    emoji.Error + " Failed",
			Issues:    "1+",
			Details:   err.Error(),
		})
		hasErrors = true
	} else {
		fmt.Printf("%s Success\n", emoji.Success)
		results = append(results, ValidationResult{
			Component: "Cross-references",
			Status:    emoji.Success + " Success",
			Issues:    "0",
			Details:   "References valid",
		})
	}

	fmt.Println()

	// Display results in configured output format
	outputFormat := format.DetectFormat(app.OutputFormat())
	if outputFormat == format.FormatTable {
		displayValidationTable(results, verbose)
	} else {
		formatter := format.NewFormatter(outputFormat)
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

func validateCrossReferences(app application.Application, verbose bool) error {
	// Load catalog from app context to check cross-references
	cat, err := app.Catalog()
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	var errors []string

	// Check that all model authors exist
	models := cat.Models().List()
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
				fmt.Printf("    %s %s\n", emoji.Error, err)
			}
		}
		return fmt.Errorf("found %d cross-reference errors", len(errors))
	}

	return nil
}
