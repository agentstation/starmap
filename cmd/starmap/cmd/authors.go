package cmd

import (
	"fmt"
	"sort"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/spf13/cobra"
)

// authorsCmd represents the authors command
var authorsCmd = &cobra.Command{
	Use:   "authors",
	Short: "List all model authors",
	Long: `List displays all AI model authors and organizations in the catalog.
	
For each author, it shows:
  - Author ID and display name
  - Description of their work
  - Number of models in the catalog
  - Website and social links`,
	RunE: runAuthors,
}

func init() {
	rootCmd.AddCommand(authorsCmd)
}

func runAuthors(cmd *cobra.Command, args []string) error {
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Get all authors from the catalog
	authors := catalog.Authors().List()
	if len(authors) == 0 {
		fmt.Println("No authors found in catalog")
		return nil
	}

	// Sort authors by ID
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].ID < authors[j].ID
	})

	fmt.Printf("Found %d authors in catalog:\n\n", len(authors))
	for _, author := range authors {
		fmt.Printf("• %s - %s\n", author.ID, author.Name)

		if author.Description != nil && *author.Description != "" {
			fmt.Printf("  %s\n", *author.Description)
		}

		if len(author.Models) > 0 {
			fmt.Printf("  Models: %d\n", len(author.Models))
		}

		if author.Website != nil && *author.Website != "" {
			fmt.Printf("  Website: %s\n", *author.Website)
		}

		if author.GitHub != nil && *author.GitHub != "" {
			fmt.Printf("  GitHub: %s\n", *author.GitHub)
		}

		if author.HuggingFace != nil && *author.HuggingFace != "" {
			fmt.Printf("  HuggingFace: %s\n", *author.HuggingFace)
		}

		if author.Twitter != nil && *author.Twitter != "" {
			fmt.Printf("  Twitter: %s\n", *author.Twitter)
		}

		fmt.Println()
	}

	return nil
}
