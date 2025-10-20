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

	// Add command groups (workflow-based organization)
	rootCmd.AddGroup(&cobra.Group{
		ID:    "setup",
		Title: "Setup Commands:",
	})

	rootCmd.AddGroup(&cobra.Group{
		ID:    "catalog",
		Title: "Catalog Commands:",
	})

	rootCmd.AddGroup(&cobra.Group{
		ID:    "server",
		Title: "Server Commands:",
	})

	rootCmd.AddGroup(&cobra.Group{
		ID:    "development",
		Title: "Development Commands:",
	})

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&a.config.ConfigFile, "config", "", "config file (default is $HOME/.starmap.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&a.config.Verbose, "verbose", "v", false, "verbose output (shortcut for --log-level=debug)")
	rootCmd.PersistentFlags().BoolVarP(&a.config.Quiet, "quiet", "q", false, "minimal output (shortcut for --log-level=warn)")
	rootCmd.PersistentFlags().BoolVar(&a.config.NoColor, "no-color", false, "disable colored output")
	// Use -o for output (not -f) to avoid conflict with embed cat --filename
	rootCmd.PersistentFlags().StringVarP(&a.config.Output, "output", "o", "", "output format: table, json, yaml, wide")
	rootCmd.PersistentFlags().StringVar(&a.config.LogLevel, "log-level", "", "log level: trace, debug, info, warn, error (overrides -v/-q)")

	// Add --format and --fmt as aliases for --output (backwards compatibility)
	rootCmd.PersistentFlags().StringVar(&a.config.Output, "format", "", "")
	rootCmd.PersistentFlags().StringVar(&a.config.Output, "fmt", "", "")
	_ = rootCmd.PersistentFlags().MarkHidden("format") // Hidden but functional
	_ = rootCmd.PersistentFlags().MarkHidden("fmt")    // Hidden but functional

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
	output := mustGetString(cmd, "output")
	logLevel := mustGetString(cmd, "log-level")

	a.config.UpdateFromFlags(verbose, quiet, noColor, output, logLevel)

	// Reinitialize logger with updated config
	logger := NewLogger(a.config)
	a.logger = &logger

	return nil
}

// registerCommands registers all subcommands with the root command.
// This is where we wire up all the command handlers.
func (a *App) registerCommands(rootCmd *cobra.Command) {
	// Setup commands (getting started)
	rootCmd.AddCommand(a.NewAuthCommand())
	rootCmd.AddCommand(a.NewDepsCommand())

	// Catalog commands (working with models/providers)
	rootCmd.AddCommand(a.NewListCommand())
	rootCmd.AddCommand(a.NewFetchCommand())
	rootCmd.AddCommand(a.NewUpdateCommand())

	// Server commands (running the API)
	rootCmd.AddCommand(a.NewServeCommand())

	// Development commands (debugging and exploration)
	rootCmd.AddCommand(a.NewValidateCommand())
	rootCmd.AddCommand(a.NewEmbedCommand())

	// Additional commands (no group)
	rootCmd.AddCommand(a.NewCompletionCommand(rootCmd)) // Override Cobra's auto-generated completion
	rootCmd.AddCommand(a.NewVersionCommand())
	rootCmd.AddCommand(a.NewManCommand())
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
