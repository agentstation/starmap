// Package list provides commands for listing starmap resources.
package list

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/catalog"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/filter"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// AuthorsCmd represents the list authors subcommand.
var AuthorsCmd = &cobra.Command{
	Use:     "authors [author-id]",
	Short:   "List authors from catalog",
	Aliases: []string{"author"},
	Args:    cobra.MaximumNArgs(1),
	Example: `  starmap list authors                     # List all authors
  starmap list authors openai              # Show specific author details
  starmap list authors --search meta       # Search for authors by name`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Single author detail view
		if len(args) == 1 {
			return showAuthorDetails(cmd, args[0])
		}

		// List view with filters
		resourceFlags := globals.ParseResources(cmd)
		return listAuthors(cmd, resourceFlags)
	},
}

func init() {
	// Add resource-specific flags
	globals.AddResourceFlags(AuthorsCmd)
}

// listAuthors lists all authors with optional filters.
func listAuthors(cmd *cobra.Command, flags *globals.ResourceFlags) error {
	cat, err := catalog.Load()
	if err != nil {
		return err
	}

	// Get all authors
	authors := cat.Authors().List()

	// Apply filters
	authorFilter := &filter.AuthorFilter{
		Search: flags.Search,
	}
	// Convert to value slice for filter
	authorValues := make([]catalogs.Author, len(authors))
	for i, a := range authors {
		authorValues[i] = *a
	}
	filtered := authorFilter.Apply(authorValues)

	// Sort authors
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	// Apply limit
	if flags.Limit > 0 && len(filtered) > flags.Limit {
		filtered = filtered[:flags.Limit]
	}

	// Get global flags and format output
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		tableData := table.AuthorsToTableData(filtered)
		// Convert to output.TableData for formatter compatibility
		outputData = output.TableData{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "Found %d authors\n", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showAuthorDetails shows detailed information about a specific author.
func showAuthorDetails(cmd *cobra.Command, authorID string) error {
	cat, err := catalog.Load()
	if err != nil {
		return err
	}

	author, found := cat.Authors().Get(catalogs.AuthorID(authorID))
	if !found {
		// Suppress usage display for not found errors
		cmd.SilenceUsage = true
		return &errors.NotFoundError{
			Resource: "author",
			ID:       authorID,
		}
	}

	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// For table output, show detailed view
	if globalFlags.Output == constants.FormatTable || globalFlags.Output == "" {
		printAuthorDetails(author)
		return nil
	}

	// For structured output, return the author
	return formatter.Format(os.Stdout, author)
}

// Removed authorsToTableData - now using shared table.AuthorsToTableData

// printAuthorDetails prints detailed author information in a human-readable format.
func printAuthorDetails(author *catalogs.Author) {
	fmt.Printf("Author: %s\n", author.ID)
	fmt.Printf("Name: %s\n", author.Name)

	if author.Description != nil && *author.Description != "" {
		fmt.Printf("Description: %s\n", *author.Description)
	}

	fmt.Printf("Models: %d\n", len(author.Models))

	if author.Website != nil && *author.Website != "" {
		fmt.Printf("\nLinks:\n")
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

	if len(author.Models) > 0 && len(author.Models) <= 20 {
		fmt.Printf("\nModels:\n")
		// Sort model IDs for consistent output
		modelIDs := make([]string, 0, len(author.Models))
		for id := range author.Models {
			modelIDs = append(modelIDs, id)
		}
		sort.Strings(modelIDs)

		for _, id := range modelIDs {
			model := author.Models[id]
			fmt.Printf("  • %s - %s\n", model.ID, model.Name)
		}
	}
}
