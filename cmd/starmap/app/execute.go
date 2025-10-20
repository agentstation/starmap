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
	rootCmd.PersistentFlags().StringVarP(&a.config.Format, "format", "o", "", "output format: table, json, yaml, wide")
	rootCmd.PersistentFlags().StringVar(&a.config.LogLevel, "log-level", "", "log level: trace, debug, info, warn, error (overrides -v/-q)")

	// Add --output as deprecated alias for --format (backwards compatibility)
	rootCmd.PersistentFlags().StringVar(&a.config.Format, "output", "", "")
	rootCmd.PersistentFlags().MarkDeprecated("output", "use --format instead")

	// Customize version output to match version subcommand
	rootCmd.SetVersionTemplate("starmap {{.Version}}\n")

	// Register all commands
	a.registerCommands(rootCmd)

	return rootCmd
}

// setupCommand is called before any command runs.
func (a *App) setupCommand(cmd *cobra.Command, _ []string) error {
	// Update config from parsed flags
	// These flags are defined as persistent flags in createRootCommand, so errors indicate programming errors
	verbose := mustGetBool(cmd, "verbose")
	quiet := mustGetBool(cmd, "quiet")
	noColor := mustGetBool(cmd, "no-color")
	format := mustGetString(cmd, "format")
	logLevel := mustGetString(cmd, "log-level")

	a.config.UpdateFromFlags(verbose, quiet, noColor, format, logLevel)

	// Reinitialize logger with updated config
	logger := NewLogger(a.config)
	a.logger = &logger

	return nil
}

// registerCommands registers all subcommands with the root command.
// This is where we wire up all the command handlers.
func (a *App) registerCommands(rootCmd *cobra.Command) {
	// Core commands
	rootCmd.AddCommand(a.NewListCommand())
	rootCmd.AddCommand(a.NewUpdateCommand())
	rootCmd.AddCommand(a.NewServeCommand())
	rootCmd.AddCommand(a.NewFetchCommand())

	// Management commands
	rootCmd.AddCommand(a.NewValidateCommand())
	rootCmd.AddCommand(a.NewEmbedCommand())
	rootCmd.AddCommand(a.NewAuthCommand())
	rootCmd.AddCommand(a.NewDepsCommand())
	rootCmd.AddCommand(a.NewGenerateCommand())

	// Utility commands
	rootCmd.AddCommand(a.NewVersionCommand())
	rootCmd.AddCommand(a.NewManCommand())
	rootCmd.AddCommand(a.NewInstallCommand())
	rootCmd.AddCommand(a.NewUninstallCommand())
}

// ExitOnError is a helper that prints an error and exits with status 1.
// This is meant to be used in main.go for top-level error handling.
func ExitOnError(err error) {
	if err != nil {
		//nolint:errcheck // Ignoring write error since we're exiting anyway
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

// mustGetBool retrieves a boolean flag value or panics if the flag doesn't exist.
// This should only be used for flags defined in this package.
func mustGetBool(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic("programming error: failed to get flag " + name + ": " + err.Error())
	}
	return val
}

// mustGetString retrieves a string flag value or panics if the flag doesn't exist.
// This should only be used for flags defined in this package.
func mustGetString(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic("programming error: failed to get flag " + name + ": " + err.Error())
	}
	return val
}
