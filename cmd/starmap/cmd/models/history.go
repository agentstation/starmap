package models

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// NewHistoryCommand creates the history subcommand for viewing model data sources.
func NewHistoryCommand(app application.Application) *cobra.Command {
	var fieldPatterns []string

	cmd := &cobra.Command{
		Use:   "history <model-id>",
		Short: "Show field history for a model",
		Long: `Show field-level data source tracking and change history for a model.

Displays which sources provided each field value, with full history showing:
- Current value and source
- Authority score (why this source was chosen)
- Timestamp of last update
- Complete history of value changes

Supports filtering to specific fields using the --fields flag with wildcards.
Field matching is case-insensitive for convenience.`,
		Args: cobra.ExactArgs(1),
		Example: `  starmap models history gpt-4o                        # Show all history
  starmap models history gpt-4o --fields=Name          # Show Name field only
  starmap models history gpt-4o --fields=Name,ID       # Multiple fields (comma-separated)
  starmap models history gpt-4o --fields='pricing.*'   # Show all Pricing fields (case-insensitive)
  starmap models history gpt-4o -o json                # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showModelHistory(cmd, app, args[0], fieldPatterns)
		},
	}

	// Add fields filter flag
	cmd.Flags().StringSliceVar(&fieldPatterns, "fields", []string{},
		"Filter to specific fields (comma-separated, case-insensitive, supports wildcards like 'pricing.*')")

	return cmd
}

// showModelHistory displays history data for a specific model.
func showModelHistory(cmd *cobra.Command, app application.Application, modelID string, fieldPatterns []string) error {
	// Get catalog
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Find model across all providers
	var found bool
	providers := cat.Providers().List()
	for _, provider := range providers {
		if _, exists := provider.Models[modelID]; exists {
			found = true
			break
		}
	}

	if !found {
		cmd.SilenceUsage = true
		return &errors.NotFoundError{
			Resource: "model",
			ID:       modelID,
		}
	}

	// Query provenance container directly for this model
	fieldProvenance := cat.Provenance().FindByResource(sources.ResourceTypeModel, modelID)

	if len(fieldProvenance) == 0 {
		return fmt.Errorf("no history data found for model %q\n\nRun 'starmap update' to generate history tracking data", modelID)
	}

	// Apply field filtering if requested
	if len(fieldPatterns) > 0 {
		filtered := make(map[string][]provenance.Provenance)
		for field, provList := range fieldProvenance {
			if table.MatchField(field, fieldPatterns) {
				filtered[field] = provList
			}
		}
		fieldProvenance = filtered

		if len(fieldProvenance) == 0 {
			return fmt.Errorf("no history data found for model %q matching fields: %s", modelID, strings.Join(fieldPatterns, ", "))
		}
	}

	// Get global flags for output format
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}

	// Format output
	formatter := format.NewFormatter(format.Format(globalFlags.Output))

	// For structured output (JSON/YAML), return raw data
	if globalFlags.Output != constants.FormatTable && globalFlags.Output != "" {
		return formatter.Format(os.Stdout, fieldProvenance)
	}

	// For table output, print detailed view
	printModelHistory(fieldProvenance, formatter)
	return nil
}

// printModelHistory prints detailed history information for a model.
func printModelHistory(fieldProvenance map[string][]provenance.Provenance, formatter format.Formatter) {
	// Use table format
	tableData := table.ProvenanceToTableData(fieldProvenance)

	// Convert table.Data to format.Data
	formatData := format.Data{
		Headers:         tableData.Headers,
		Rows:            tableData.Rows,
		ColumnAlignment: tableData.ColumnAlignment,
	}

	_ = formatter.Format(os.Stdout, formatData)
}
