package models

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cli/constants"
	"github.com/agentstation/starmap/internal/cli/format"
	"github.com/agentstation/starmap/internal/cli/globals"
	"github.com/agentstation/starmap/internal/cli/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// NewHistoryCommand creates the history subcommand for viewing model data sources.
func NewHistoryCommand(app application) *cobra.Command {
	var fieldPatterns []string
	var providerID string

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
  starmap models history shared --provider=openrouter    # Select one provider offering
  starmap models history gpt-4o --fields=Name          # Show Name field only
  starmap models history gpt-4o --fields=Name,ID       # Multiple fields (comma-separated)
  starmap models history gpt-4o --fields='pricing.*'   # Show all Pricing fields (case-insensitive)
  starmap models history gpt-4o -o json                # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showModelHistory(cmd, app, catalogs.ProviderID(providerID), args[0], fieldPatterns)
		},
	}

	cmd.Flags().StringVar(&providerID, "provider", "",
		"Provider that owns the model (required when the model ID exists at multiple providers)")
	// Add fields filter flag
	cmd.Flags().StringSliceVar(&fieldPatterns, "fields", []string{},
		"Filter to specific fields (comma-separated, case-insensitive, supports wildcards like 'pricing.*')")

	return cmd
}

// showModelHistory displays history data for a specific model.
func showModelHistory(
	cmd *cobra.Command,
	app application,
	requestedProvider catalogs.ProviderID,
	modelID string,
	fieldPatterns []string,
) error {
	// Get catalog
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	providerID, err := resolveHistoryProvider(cat, requestedProvider, modelID)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	fieldProvenance := cat.Provenance().FindModel(providerID, modelID)

	if len(fieldProvenance) == 0 {
		return fmt.Errorf(
			"no history data found for provider model %q/%q\n\nRun 'starmap update' to generate history tracking data",
			providerID,
			modelID,
		)
	}

	// Apply field filtering if requested
	if len(fieldPatterns) > 0 {
		filtered := make(map[string][]provenance.Entry)
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
	formatter := format.New(format.Kind(globalFlags.Output))

	// For structured output (JSON/YAML), return raw data
	if globalFlags.Output != constants.FormatTable && globalFlags.Output != "" {
		return formatter.Format(os.Stdout, fieldProvenance)
	}

	// For table output, print detailed view
	printModelHistory(fieldProvenance, formatter)
	return nil
}

func resolveHistoryProvider(
	catalog *catalogs.Catalog,
	requested catalogs.ProviderID,
	modelID string,
) (catalogs.ProviderID, error) {
	if requested != "" {
		offering, err := catalog.Offering(requested, catalogs.ProviderModelID(modelID))
		if err != nil {
			return "", err
		}
		return offering.ProviderID, nil
	}

	var matches []catalogs.ProviderID
	for _, provider := range catalog.Providers().List() {
		if _, exists := provider.Models[modelID]; exists {
			matches = append(matches, provider.ID)
		}
	}
	slices.Sort(matches)
	switch len(matches) {
	case 0:
		return "", &errors.NotFoundError{Resource: "model", ID: modelID}
	case 1:
		return matches[0], nil
	default:
		return "", &errors.ValidationError{
			Field:   "provider",
			Value:   matches,
			Message: fmt.Sprintf("is required because model %q exists at providers %s", modelID, strings.Join(providerIDStrings(matches), ", ")),
		}
	}
}

func providerIDStrings(providerIDs []catalogs.ProviderID) []string {
	values := make([]string, len(providerIDs))
	for index, providerID := range providerIDs {
		values[index] = string(providerID)
	}
	return values
}

// printModelHistory prints detailed history information for a model.
func printModelHistory(fieldProvenance map[string][]provenance.Entry, formatter format.Renderer) {
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
