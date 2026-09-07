// Package config provides operator configuration inspection commands.
package config

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cli/format"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
)

type application interface {
	OutputFormat() string
	FileManifest() (productpaths.FileManifest, error)
	InspectFiles(context.Context, int) (productpaths.FileManifest, error)
}

// NewCommand creates passive configuration inspection commands.
func NewCommand(app application) *cobra.Command {
	command := &cobra.Command{Use: "config", GroupID: "setup", Short: "Inspect application configuration", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	var inspect bool
	var limit int
	paths := &cobra.Command{Use: "paths", Short: "Show selected roots and the managed file inventory", Args: cobra.NoArgs,
		Long: `Report configured file locations without opening catalog state or creating files.
Patterns describe possible files, not a directory listing or an existence check.
Planned entries identify reserved paths without an implemented writer.
Use --output json or --output yaml for origins, anchors, and recovery rules.
Add --inspect for bounded file metadata. This scan does not establish readiness,
effective access, runtime ownership, or an atomic snapshot.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := format.ParseFormat(app.OutputFormat()); err != nil {
				return err
			}
			if !inspect && cmd.Flags().Changed("max-entries") || limit < 1 || limit > productpaths.MaximumInspectionEntries {
				return &errors.ValidationError{Field: "max-entries", Message: "requires --inspect and a limit between 1 and 100000"}
			}
			var report productpaths.FileManifest
			var err error
			if inspect {
				report, err = app.InspectFiles(cmd.Context(), limit)
			} else {
				report, err = app.FileManifest()
			}
			if err != nil {
				return err
			}
			kind := format.DetectFormat(app.OutputFormat())
			var output any = report
			if kind == format.FormatTable || kind == format.FormatWide {
				if report.Inspection != nil {
					return formatInspection(cmd.OutOrStdout(), kind, *report.Inspection)
				}
				data := format.Data{Headers: []string{"Path role", "Location", "Availability"}}
				if kind == format.FormatWide {
					data.Headers = append(data.Headers, "Origin", "Patterns", "Access policy", "Retention")
				}

				for _, role := range []productpaths.Root{productpaths.Config, productpaths.Data, productpaths.State, productpaths.Cache} {
					root := report.Roots[role]
					row := []string{"root/" + string(role), root.Path, "root"}
					if kind == format.FormatWide {
						row = append(row, root.Origin, "", "mixed", "mixed")
					}
					data.Rows = append(data.Rows, row)
				}
				for _, entry := range report.Files {
					row := []string{entry.ID, entry.Location.Path, entry.Availability}
					if entry.Availability == "disabled" {
						row[1] = "(disabled)"
					}
					if kind == format.FormatWide {
						row = append(row, entry.Location.Origin, strings.Join(entry.Patterns, ", "), entry.Policy.Access, entry.Policy.Retention)
					}
					data.Rows = append(data.Rows, row)
				}
				output = data
			}
			return format.New(kind).Format(cmd.OutOrStdout(), output)
		}}
	paths.Flags().BoolVar(&inspect, "inspect", false, "read bounded metadata without opening catalog state")
	paths.Flags().IntVar(&limit, "max-entries", productpaths.DefaultInspectionEntries, "maximum visited entries with --inspect")
	command.AddCommand(paths)
	return command
}
