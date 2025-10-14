package list

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/application"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// NewAuthorsCommand creates the list authors subcommand using app context.
func NewAuthorsCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "authors [author-id]",
		Short:   "List authors from catalog",
		Aliases: []string{"author"},
		Args:    cobra.MaximumNArgs(1),
		Example: `  starmap list authors                  # List all authors
  starmap list authors openai           # Show specific author details
  starmap list authors --search meta    # Search authors by name`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := app.Logger()

			// Single author detail view
			if len(args) == 1 {
				return showAuthorDetails(cmd, app, logger, args[0])
			}

			// List view
			resourceFlags := globals.ParseResources(cmd)
			return listAuthors(cmd, app, logger, resourceFlags)
		},
	}

	// Add resource-specific flags
	globals.AddResourceFlags(cmd)

	return cmd
}

// listAuthors lists all authors using app context.
func listAuthors(cmd *cobra.Command, app application.Application, logger *zerolog.Logger, flags *globals.ResourceFlags) error {
	// Get catalog from app
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Get all authors
	allAuthors := cat.Authors().List()

	// Apply search filter if specified
	var filtered []catalogs.Author
	if flags.Search != "" {
		searchLower := strings.ToLower(flags.Search)
		for _, a := range allAuthors {
			if strings.Contains(strings.ToLower(string(a.ID)), searchLower) ||
				strings.Contains(strings.ToLower(a.Name), searchLower) {
				filtered = append(filtered, a)
			}
		}
	} else {
		filtered = allAuthors
	}

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
		authorPointers := make([]*catalogs.Author, len(filtered))
		for i := range filtered {
			authorPointers[i] = &filtered[i]
		}
		tableData := table.AuthorsToTableData(authorPointers)
		outputData = output.Data{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		logger.Info().Msgf("Found %d authors", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showAuthorDetails shows detailed information about a specific author.
func showAuthorDetails(cmd *cobra.Command, app application.Application, logger *zerolog.Logger, authorID string) error {
	// Get catalog from app
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Find specific author
	author, exists := cat.Authors().Get(catalogs.AuthorID(authorID))
	if !exists {
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

// printAuthorDetails prints detailed author information.
func printAuthorDetails(author *catalogs.Author) {
	formatter := output.NewFormatter(output.FormatTable)

	fmt.Printf("Author: %s\n\n", author.ID)

	// Basic info
	basicRows := [][]string{
		{"Author ID", string(author.ID)},
		{"Name", author.Name},
	}

	if author.Description != nil && *author.Description != "" {
		basicRows = append(basicRows, []string{"Description", *author.Description})
	}
	if author.Website != nil && *author.Website != "" {
		basicRows = append(basicRows, []string{"Website", *author.Website})
	}

	basicTable := output.Data{
		Headers: []string{"Property", "Value"},
		Rows:    basicRows,
	}

	fmt.Println("Basic Information:")
	_ = formatter.Format(os.Stdout, basicTable)
	fmt.Println()
}
