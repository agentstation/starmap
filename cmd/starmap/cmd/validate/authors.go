package validate

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/emoji"
)

// NewAuthorsCommand creates the validate authors subcommand using app context.
func NewAuthorsCommand(app application.Application) *cobra.Command {
	return &cobra.Command{
		Use:   "authors",
		Short: "Validate authors.yaml structure",
		Long: `Validate that authors.yaml has all required fields and follows the schema.

This checks:
  - Required fields (id, name)
  - URL formats for social links
  - Duplicate IDs
  - Data consistency`,
		RunE: func(_ *cobra.Command, args []string) error {
			// This command doesn't take positional arguments yet
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument: %s", args[0])
			}

			logger := app.Logger()
			verbose := logger.GetLevel() <= zerolog.InfoLevel
			return validateAuthorsStructure(app, verbose)
		},
	}
}

func validateAuthorsStructure(app application.Application, verbose bool) error {
	// Load catalog from app context
	cat, err := app.Catalog()
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	authors := cat.Authors().List()
	if len(authors) == 0 {
		return fmt.Errorf("no authors found in catalog")
	}

	var validationErrors []string
	seenIDs := make(map[string]bool)

	for _, author := range authors {
		// Check required fields
		if author.ID == "" {
			validationErrors = append(validationErrors,
				"author missing required field 'id'")
			continue
		}

		// Check for duplicate IDs
		if seenIDs[string(author.ID)] {
			validationErrors = append(validationErrors,
				fmt.Sprintf("duplicate author ID: %s", author.ID))
		}
		seenIDs[string(author.ID)] = true

		if author.Name == "" {
			validationErrors = append(validationErrors,
				fmt.Sprintf("author %s missing required field 'name'", author.ID))
		}

		// Validate URLs
		if author.Website != nil && *author.Website != "" && !isValidURL(*author.Website) {
			validationErrors = append(validationErrors,
				fmt.Sprintf("author %s has invalid website URL: %s", author.ID, *author.Website))
		}

		if author.GitHub != nil && *author.GitHub != "" && !isValidURL(*author.GitHub) {
			validationErrors = append(validationErrors,
				fmt.Sprintf("author %s has invalid GitHub URL: %s", author.ID, *author.GitHub))
		}

		if author.HuggingFace != nil && *author.HuggingFace != "" && !isValidURL(*author.HuggingFace) {
			validationErrors = append(validationErrors,
				fmt.Sprintf("author %s has invalid HuggingFace URL: %s", author.ID, *author.HuggingFace))
		}

		if author.Twitter != nil && *author.Twitter != "" && !isValidURL(*author.Twitter) {
			validationErrors = append(validationErrors,
				fmt.Sprintf("author %s has invalid Twitter URL: %s", author.ID, *author.Twitter))
		}

		if verbose {
			fmt.Printf("  %s Validated author: %s\n", emoji.Success, author.Name)
		}
	}

	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			fmt.Printf("  %s %s\n", emoji.Error, err)
		}
		return fmt.Errorf("found %d validation errors", len(validationErrors))
	}

	fmt.Printf("%s Validated %d authors successfully\n", emoji.Success, len(authors))
	return nil
}
