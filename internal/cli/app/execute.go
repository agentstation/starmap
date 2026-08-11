package app

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

type commandFlags struct {
	configFile string
	verbose    bool
	quiet      bool
	noColor    bool
	output     string
	logLevel   string
}

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
	rootCmd.PersistentFlags().StringVar(&a.commandFlags.configFile, "config", "", "config file (default is $HOME/.starmap/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&a.commandFlags.verbose, "verbose", "v", a.config.Verbose, "verbose output (shortcut for --log-level=debug)")
	rootCmd.PersistentFlags().BoolVarP(&a.commandFlags.quiet, "quiet", "q", a.config.Quiet, "minimal output (shortcut for --log-level=warn)")
	rootCmd.PersistentFlags().BoolVar(&a.commandFlags.noColor, "no-color", a.config.NoColor, "disable colored output")
	// Use -o for output (not -f) to avoid conflict with embed cat --filename
	rootCmd.PersistentFlags().StringVarP(&a.commandFlags.output, "output", "o", a.config.Output, "output format: table, json, yaml, wide")
	rootCmd.PersistentFlags().StringVar(&a.commandFlags.logLevel, "log-level", a.config.LogLevel, "log level: trace, debug, info, warn, error (overrides -v/-q)")

	// Customize version output to match version subcommand
	rootCmd.SetVersionTemplate("starmap {{.Version}}\n")

	// Register all commands
	a.registerCommands(rootCmd)

	return rootCmd
}

// setupCommand is called before any command runs.
func (a *App) setupCommand(cmd *cobra.Command, _ []string) error {
	if a.commandFlags.configFile != "" {
		config, err := loadConfig(a.commandFlags.configFile)
		if err != nil {
			return errors.WrapResource("load", "config", a.commandFlags.configFile, err)
		}
		a.config = config
		a.credentialMu.Lock()
		a.credentialResolver = nil
		a.credentialMu.Unlock()
	}

	// Apply only explicitly provided flags. Values loaded from a config file or
	// environment must survive command construction and parsing.
	if cmd.Flags().Changed("verbose") {
		a.config.Verbose = mustGetBool(cmd, "verbose")
	}
	if cmd.Flags().Changed("quiet") {
		a.config.Quiet = mustGetBool(cmd, "quiet")
	}
	if cmd.Flags().Changed("no-color") {
		a.config.NoColor = mustGetBool(cmd, "no-color")
	}
	if cmd.Flags().Changed("output") {
		a.config.Output = mustGetString(cmd, "output")
	}
	if cmd.Flags().Changed("log-level") {
		a.config.LogLevel = mustGetString(cmd, "log-level")
	}

	// Reinitialize logger with updated config
	logger := NewLogger(a.config)
	a.logger = &logger
	// The executable is the process-level composition root. Install its logger
	// explicitly so package-level diagnostics honor CLI flags without mutating
	// zerolog's separate global logger or level.
	logging.SetDefault(logger)

	return nil
}

// registerCommands registers all subcommands with the root command.
// This is where we wire up all the command handlers.
func (a *App) registerCommands(rootCmd *cobra.Command) {
	// Setup commands (getting started)
	rootCmd.AddCommand(a.NewDepsCommand())
	rootCmd.AddCommand(a.NewAuthCommand())
	rootCmd.AddCommand(a.NewMigrateCommand())

	// Catalog commands (working with models/providers)
	rootCmd.AddCommand(a.NewProvidersCommand())
	rootCmd.AddCommand(a.NewModelsCommand())
	rootCmd.AddCommand(a.NewAuthorsCommand())
	rootCmd.AddCommand(a.NewUpdateCommand())

	// Server commands (running the API)
	rootCmd.AddCommand(a.NewServeCommand())

	// Development commands (debugging and exploration)
	rootCmd.AddCommand(a.NewValidateCommand())
	rootCmd.AddCommand(a.NewEmbedCommand())

	// Additional commands (no group)
	rootCmd.AddCommand(a.NewCompletionCommand()) // Custom completion with install/uninstall
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
