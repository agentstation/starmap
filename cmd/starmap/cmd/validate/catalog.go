package validate

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/catalogs"
)

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

	var hasErrors bool

	// Validate providers.yaml
	fmt.Println("Validating providers.yaml...")
	if err := validateProvidersStructure(verbose); err != nil {
		fmt.Printf("  ❌ Providers validation failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Println("  ✅ Providers structure valid")
	}

	// Validate authors.yaml
	fmt.Println("\nValidating authors.yaml...")
	if err := validateAuthorsStructure(verbose); err != nil {
		fmt.Printf("  ❌ Authors validation failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Println("  ✅ Authors structure valid")
	}

	// Validate model consistency
	fmt.Println("\nValidating model definitions...")
	if err := validateModelConsistency(verbose); err != nil {
		fmt.Printf("  ❌ Model validation failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Println("  ✅ Model definitions valid")
	}

	// Check cross-references
	fmt.Println("\nValidating cross-references...")
	if err := validateCrossReferences(verbose); err != nil {
		fmt.Printf("  ❌ Cross-reference validation failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Println("  ✅ Cross-references valid")
	}

	if hasErrors {
		return fmt.Errorf("catalog validation failed")
	}

	fmt.Println("\n✨ Catalog validation successful!")
	return nil
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