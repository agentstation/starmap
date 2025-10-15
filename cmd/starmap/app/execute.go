package app

import (
	"context"
	"os"

	"github.com/spf13/cobra"
)

// Execute runs the starmap CLI application with the given arguments.
// This is the main entry point called from main.go.
func (a *App) Execute(ctx context.Context, args []string) error {
	// Create root command with app context
	rootCmd := a.createRootCommand()

	// Set arguments
	rootCmd.SetArgs(args)

	// Execute with context
	return rootCmd.ExecuteContext(ctx)
}

// createRootCommand creates the root cobra command with all subcommands.
func (a *App) createRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "starmap",
		Short:   "AI Model Catalog CLI",
		Version: a.version,
		Long: `Starmap is a comprehensive AI model catalog system that provides
information about AI models, their capabilities, and providers.

It includes an embedded catalog of models that can be accessed offline,
as well as the ability to fetch live model information from provider APIs
when API keys are configured.`,
		PersistentPreRunE: a.setupCommand,
		SilenceUsage:      true,
		SilenceErrors:     true,
	}

	// Add command groups
	rootCmd.AddGroup(&cobra.Group{
		ID:    "core",
		Title: "Core Commands:",
	})

	rootCmd.AddGroup(&cobra.Group{
		ID:    "management",
		Title: "Management Commands:",
	})

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&a.config.ConfigFile, "config", "", "config file (default is $HOME/.starmap.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&a.config.Verbose, "verbose", "v", false, "verbose output (shortcut for --log-level=debug)")
	rootCmd.PersistentFlags().BoolVarP(&a.config.Quiet, "quiet", "q", false, "minimal output (shortcut for --log-level=warn)")
	rootCmd.PersistentFlags().BoolVar(&a.config.NoColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().StringVarP(&a.config.Output, "output", "o", "", "output format: table, json, yaml, wide")
	rootCmd.PersistentFlags().StringVar(&a.config.LogLevel, "log-level", "", "log level: trace, debug, info, warn, error (overrides -v/-q)")

	// Customize version output to match version subcommand
	rootCmd.SetVersionTemplate("starmap {{.Version}}\n")

	// Register all commands
	a.registerCommands(rootCmd)

	return rootCmd
}

// setupCommand is called before any command runs.
func (a *App) setupCommand(cmd *cobra.Command, args []string) error {
	// Update config from parsed flags
	verbose, _ := cmd.Flags().GetBool("verbose")
	quiet, _ := cmd.Flags().GetBool("quiet")
	noColor, _ := cmd.Flags().GetBool("no-color")
	output, _ := cmd.Flags().GetString("output")
	logLevel, _ := cmd.Flags().GetString("log-level")

	a.config.UpdateFromFlags(verbose, quiet, noColor, output, logLevel)

	// Reinitialize logger with updated config
	logger := NewLogger(a.config)
	a.logger = &logger

	return nil
}

// registerCommands registers all subcommands with the root command.
// This is where we wire up all the command handlers.
func (a *App) registerCommands(rootCmd *cobra.Command) {
	// Core commands
	rootCmd.AddCommand(a.CreateListCommand())
	rootCmd.AddCommand(a.CreateUpdateCommand())
	rootCmd.AddCommand(a.CreateServeCommand())
	rootCmd.AddCommand(a.CreateFetchCommand())

	// Management commands
	rootCmd.AddCommand(a.CreateValidateCommand())
	rootCmd.AddCommand(a.CreateInspectCommand())
	rootCmd.AddCommand(a.CreateAuthCommand())
	rootCmd.AddCommand(a.CreateGenerateCommand())

	// Utility commands
	rootCmd.AddCommand(a.CreateVersionCommand())
	rootCmd.AddCommand(a.CreateManCommand())
	rootCmd.AddCommand(a.CreateInstallCommand())
	rootCmd.AddCommand(a.CreateUninstallCommand())
}

// ExitOnError is a helper that prints an error and exits with status 1.
// This is meant to be used in main.go for top-level error handling.
func ExitOnError(err error) {
	if err != nil {
		// Print to stderr (ignore write error since we're exiting anyway)
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
