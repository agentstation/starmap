package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/cli/emoji"
	"github.com/agentstation/starmap/internal/cli/format"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sync"
)

// Flags holds flags for update command.
type Flags struct {
	Provider           string
	Source             string
	DryRun             bool
	Force              bool
	AutoApprove        bool
	CatalogPath        string
	Cleanup            bool
	Reformat           bool
	SourcesDir         string
	ModelsDevGitCommit string
	AutoInstallDeps    bool
	SkipDepPrompts     bool
	RequireAllSources  bool
}

type syncClient interface {
	Sync(context.Context, ...sync.Option) (*sync.Result, error)
}

type catalogPathProvider interface {
	CatalogPath() (string, error)
}

// addUpdateFlags adds update-specific flags to the update command.
func addUpdateFlags(cmd *cobra.Command) *Flags {
	flags := &Flags{}

	cmd.Flags().StringVar(&flags.Source, "source", "",
		"Update from a specific source: all, local, provider-api, models.dev, models.dev-git")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false,
		"Preview changes without applying them")
	cmd.Flags().BoolVarP(&flags.Force, "force", "f", false,
		"Force fresh update (delete and recreate)")
	cmd.Flags().BoolVarP(&flags.AutoApprove, "yes", "y", false,
		"Auto-approve changes without confirmation")
	cmd.Flags().StringVar(&flags.CatalogPath, "catalog-path", "",
		"Human provider-YAML workspace (default: ~/.starmap/catalog)")
	cmd.Flags().BoolVar(&flags.Cleanup, "cleanup", false,
		"Remove temporary models.dev repository after update")
	cmd.Flags().BoolVar(&flags.Reformat, "reformat", false,
		"Reformat catalog files even without changes")
	cmd.Flags().StringVar(&flags.SourcesDir, "sources-dir", "",
		"Directory for external source data (default: ~/.starmap/sources)")
	cmd.Flags().StringVar(&flags.ModelsDevGitCommit, "models-dev-git-commit", "",
		"Exact commit required with --source models.dev-git")
	cmd.Flags().BoolVar(&flags.AutoInstallDeps, "auto-install-deps", false,
		"Automatically install missing dependencies without prompting")
	cmd.Flags().BoolVar(&flags.SkipDepPrompts, "skip-dep-prompts", false,
		"Skip dependency prompts and continue without optional dependencies")
	cmd.Flags().BoolVar(&flags.RequireAllSources, "require-all-sources", false,
		"Require every source to be available, complete, successful, and nonempty")

	return flags
}

// ExecuteUpdate orchestrates the complete update process using app context.
func ExecuteUpdate(ctx context.Context, app application, flags *Flags, logger *zerolog.Logger) error {
	// Determine quiet mode from logger level
	quiet := logger.GetLevel() > zerolog.InfoLevel

	// Validate force update if needed
	if flags.Force {
		proceed, err := ValidateForceUpdate(quiet, flags.AutoApprove)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	// Load the appropriate catalog using app context
	sm, err := LoadCatalog(app, flags.CatalogPath, quiet)
	if err != nil {
		return err
	}
	syncer, err := acquisition.New(sm)
	if err != nil {
		return errors.WrapResource("create", "catalog acquisition", "", err)
	}
	catalogPath, err := resolveCatalogPath(app, flags.CatalogPath)
	if err != nil {
		return err
	}
	resolvedFlags := *flags
	resolvedFlags.CatalogPath = catalogPath

	// Execute the update operation
	return updateCatalog(ctx, syncer, &resolvedFlags, logger, quiet)
}

func resolveCatalogPath(app any, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if provider, ok := app.(catalogPathProvider); ok {
		return provider.CatalogPath()
	}
	return expandPath(constants.DefaultCatalogPath), nil
}

// updateCatalog executes the update operation using app context.
func updateCatalog(ctx context.Context, sm syncClient, flags *Flags, logger *zerolog.Logger, quiet bool) error {
	return updateCatalogWithConfirmation(ctx, sm, flags, logger, quiet, ConfirmChanges)
}

func updateCatalogWithConfirmation(ctx context.Context, sm syncClient, flags *Flags, logger *zerolog.Logger, quiet bool, confirm func() (bool, error)) error {
	catalogPath := flags.CatalogPath
	// Support environment variable fallback for sources directory
	sourcesDir := flags.SourcesDir
	if sourcesDir == "" {
		sourcesDir = os.Getenv("STARMAP_SOURCES_DIR")
	}

	preview := flags.DryRun || !flags.AutoApprove
	opts, err := BuildUpdateOptions(flags.Provider, flags.Source, catalogPath, preview, flags.Force, flags.Cleanup, flags.Reformat, sourcesDir, flags.ModelsDevGitCommit, flags.AutoInstallDeps, flags.SkipDepPrompts, flags.RequireAllSources)
	if err != nil {
		return err
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "\n🔄 Starting update...\n\n")
	}

	// Perform the update
	result, err := sm.Sync(ctx, opts...)
	if err != nil {
		return &errors.ProcessError{
			Operation: "update catalog",
			Command:   "update",
			Err:       err,
		}
	}

	// Display results based on output format (checking if JSON logging is enabled)
	if logger.GetLevel() == zerolog.TraceLevel {
		// Assume structured output for trace level
		formatter := format.New(format.FormatJSON)
		return formatter.Format(os.Stdout, result)
	}

	// Handle results
	return handleResultsWithConfirmation(ctx, sm, result, flags, catalogPath, sourcesDir, quiet, confirm)
}

func handleResultsWithConfirmation(ctx context.Context, sm syncClient, result *sync.Result, flags *Flags, catalogPath string, sourcesDir string, quiet bool, confirm func() (bool, error)) error {
	if !result.HasChanges() {
		if !quiet {
			fmt.Fprintf(os.Stderr, emoji.Success+" All providers are up to date - no changes needed\n")
		}
		return nil
	}

	// Show results summary
	if !quiet {
		displayResultsSummary(result)
	}

	// Handle dry run
	if flags.DryRun {
		if !quiet {
			fmt.Fprintf(os.Stderr, "🔍 Dry run mode - no changes will be made\n")
		}
		return nil
	}

	// Handle auto-approve vs manual confirmation
	if flags.AutoApprove {
		return finalizeChanges(quiet, result)
	}

	// Ask for confirmation
	confirmed, err := confirm()
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	// Re-run update without dry-run
	if !quiet {
		fmt.Fprintf(os.Stderr, "\n🚀 Applying changes...\n")
	}

	// Rebuild options without dry-run
	opts, err := BuildUpdateOptions(flags.Provider, flags.Source, catalogPath, false, flags.Force, flags.Cleanup, flags.Reformat, sourcesDir, flags.ModelsDevGitCommit, flags.AutoInstallDeps, flags.SkipDepPrompts, flags.RequireAllSources)
	if err != nil {
		return err
	}

	// Apply changes
	finalResult, err := sm.Sync(ctx, opts...)
	if err != nil {
		return &errors.ProcessError{
			Operation: "apply changes",
			Command:   "update",
			Err:       err,
		}
	}

	return finalizeChanges(quiet, finalResult)
}

// finalizeChanges displays the completion message.
func finalizeChanges(isQuiet bool, result *sync.Result) error {
	if !isQuiet {
		fmt.Fprintf(os.Stderr, "\n🎉 Update completed successfully!\n")
		fmt.Fprintf(os.Stderr, "📊 Total: %s\n", result.Summary())
	}
	return nil
}

func expandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}
