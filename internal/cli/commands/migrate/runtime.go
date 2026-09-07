package migrate

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cli/format"
	"github.com/agentstation/starmap/runtime"
)

type runtimeMigrator interface {
	OutputFormat() string
	PrepareRuntimeMigration(context.Context, runtime.DirectoryMigrationRequest) (runtime.DirectoryMigrationPreparation, error)
	StageRuntimeMigration(context.Context, runtime.DirectoryMigrationRequest) (runtime.DirectoryMigrationStage, error)
	PublishRuntimeMigration(context.Context, runtime.DirectoryMigrationRequest) (runtime.DirectoryMigrationPublication, error)
	CompleteRuntimeMigration(context.Context, runtime.DirectoryMigrationRequest) (runtime.DirectoryMigrationPublication, error)
}

// NewRuntimeCommand creates explicit operations for one stopped runtime directory.
func NewRuntimeCommand(app runtimeMigrator) *cobra.Command {
	command := &cobra.Command{
		Use: "runtime", Short: "Migrate a stopped runtime and retain its identity",
		Long: `Stop the source process before migration. Keep older binaries stopped afterward.
Use the same operation ID, paths, identity, deployment, and instance on every retry.
Preparation records the source inventory. Staging copies and verifies its files.
Publication creates the final target and retires the source without deleting it.

Save the published target as state_dir and its identity as scheduler_identity
in the selected configuration file. Preserve the selected deployment and instance.
Completion opens that saved selection, records completion, and closes the runtime.
It does not change configuration files or service definitions.`,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	for _, step := range []struct {
		name, description string
		run               func(context.Context, runtime.DirectoryMigrationRequest) (any, error)
	}{
		{"prepare", "Record the source inventory without creating the target", func(ctx context.Context, request runtime.DirectoryMigrationRequest) (any, error) {
			return app.PrepareRuntimeMigration(ctx, request)
		}},
		{"stage", "Copy and verify files without selecting the target", func(ctx context.Context, request runtime.DirectoryMigrationRequest) (any, error) {
			return app.StageRuntimeMigration(ctx, request)
		}},
		{"publish", "Publish the verified target and retire the source", func(ctx context.Context, request runtime.DirectoryMigrationRequest) (any, error) {
			return app.PublishRuntimeMigration(ctx, request)
		}},
		{"complete", "Verify saved configuration and complete the root switch", func(ctx context.Context, request runtime.DirectoryMigrationRequest) (any, error) {
			return app.CompleteRuntimeMigration(ctx, request)
		}},
	} {
		var request runtime.DirectoryMigrationRequest
		child := &cobra.Command{
			Use: step.name, Short: step.description, Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if _, err := format.ParseFormat(app.OutputFormat()); err != nil {
					return err
				}
				result, err := step.run(cmd.Context(), request)
				if err != nil {
					return err
				}
				return format.New(format.DetectFormat(app.OutputFormat())).Format(cmd.OutOrStdout(), result)
			},
		}
		child.Flags().StringVar(&request.SourceDirectory, "from", "", "absolute source runtime directory")
		child.Flags().StringVar(&request.TargetDirectory, "to", "", "absolute replacement runtime directory")
		child.Flags().StringVar(&request.JournalRoot, "journal-root", "", "absolute journal root (default: <state>/migrations)")
		child.Flags().StringVar(&request.OperationID, "operation-id", "", "stable operation ID reused on every retry")
		child.Flags().StringVar(&request.SourceIdentity, "source-identity", "", "identity observed in the stopped source deployment")
		for _, name := range []string{"from", "to", "operation-id", "source-identity"} {
			_ = child.MarkFlagRequired(name)
		}
		command.AddCommand(child)
	}
	return command
}
