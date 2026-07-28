// Package migrate provides explicit local storage migration commands.
package migrate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/catalog/workspace"
)

type catalogMigrator interface {
	MigrateCatalogWorkspace(context.Context) (workspace.LegacyLayoutMigrationResult, error)
}

// NewCommand creates the explicit migration command.
func NewCommand(app catalogMigrator) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "migrate",
		GroupID: "setup",
		Short:   "Migrate local Starmap storage layouts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newCatalogCommand(app))
	return cmd
}

func newCatalogCommand(app catalogMigrator) *cobra.Command {
	return &cobra.Command{
		Use:   "catalog",
		Short: "Move the former generation store and restore editable provider YAML",
		Args:  cobra.NoArgs,
		Long: `Migrate the former machine-owned generation store at the configured
catalog_path into ~/.starmap/state/catalog, then project its current generation
back to catalog_path as the one human-editable provider-YAML workspace.

The command validates every retained generation and the running binary's schema
compatibility before moving anything. A validation or publication failure
restores the original generation-store layout.

Stop every older Starmap process that uses catalog_path before running this
command and do not restart it afterward. Older binaries do not understand the
new workspace meaning and can recreate machine state at the vacated path.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := app.MigrateCatalogWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"Migrated catalog generation %s (%d retained) to %s; editable YAML is at %s\n",
				result.GenerationID,
				result.RetainedCount,
				result.StatePath,
				result.WorkspacePath,
			)
			return err
		},
	}
}
