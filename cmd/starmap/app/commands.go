package app

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/auth"
	"github.com/agentstation/starmap/cmd/starmap/cmd/fetch"
	"github.com/agentstation/starmap/cmd/starmap/cmd/generate"
	"github.com/agentstation/starmap/cmd/starmap/cmd/inspect"
	"github.com/agentstation/starmap/cmd/starmap/cmd/install"
	"github.com/agentstation/starmap/cmd/starmap/cmd/list"
	"github.com/agentstation/starmap/cmd/starmap/cmd/serve"
	"github.com/agentstation/starmap/cmd/starmap/cmd/uninstall"
	"github.com/agentstation/starmap/cmd/starmap/cmd/update"
	"github.com/agentstation/starmap/cmd/starmap/cmd/validate"
	"github.com/agentstation/starmap/internal/cmd/globals"
)

// CreateListCommand creates the list command with app dependencies.
func (a *App) CreateListCommand() *cobra.Command {
	return list.NewCommand(a)
}

// CreateUpdateCommand creates the update command with app dependencies.
func (a *App) CreateUpdateCommand() *cobra.Command {
	return update.NewCommand(a)
}

// CreateServeCommand creates the serve command with app dependencies.
// TODO: Migrate serve command to use app.Context pattern
func (a *App) CreateServeCommand() *cobra.Command {
	return serve.NewCommand()
}

// CreateFetchCommand creates the fetch command with app dependencies.
// TODO: Migrate fetch command to use app.Context pattern
func (a *App) CreateFetchCommand() *cobra.Command {
	globalFlags := a.createGlobalFlags()
	cmd := &cobra.Command{
		Use:     "fetch [resource]",
		GroupID: "core",
		Short:   "Retrieve resources from provider APIs",
		Long: `Fetch retrieves live data from provider APIs.

This requires the appropriate API key to be configured either through
environment variables or the configuration file.

Supported providers include: openai, anthropic, google-ai-studio, google-vertex, groq`,
	}
	// Add subcommands
	cmd.AddCommand(fetch.NewModelsCmd(globalFlags))
	return cmd
}

// createGlobalFlags creates a globals.Flags from app config for backward compatibility
func (a *App) createGlobalFlags() *globals.Flags {
	return &globals.Flags{
		Verbose: a.config.Verbose,
		Quiet:   a.config.Quiet,
		NoColor: a.config.NoColor,
		Output:  a.config.Output,
	}
}

// CreateValidateCommand creates the validate command with app dependencies.
// TODO: Migrate validate command to use app.Context pattern
func (a *App) CreateValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate",
		GroupID: "management",
		Short:   "Validate catalog configuration and structure",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add existing subcommands
	cmd.AddCommand(validate.ModelsCmd)
	cmd.AddCommand(validate.ProvidersCmd)
	cmd.AddCommand(validate.AuthorsCmd)
	cmd.AddCommand(validate.CatalogCmd)
	return cmd
}

// CreateInspectCommand creates the inspect command with app dependencies.
// Note: No stuttering - inspect.LsCmd, not inspect.InspectCmd
func (a *App) CreateInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect embedded filesystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add existing subcommands
	cmd.AddCommand(inspect.LsCmd)
	cmd.AddCommand(inspect.CatCmd)
	cmd.AddCommand(inspect.TreeCmd)
	cmd.AddCommand(inspect.StatCmd)
	return cmd
}

// CreateAuthCommand creates the auth command with app dependencies.
// TODO: Migrate auth command to use app.Context pattern
func (a *App) CreateAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication for AI providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add existing subcommands
	cmd.AddCommand(auth.StatusCmd)
	cmd.AddCommand(auth.VerifyCmd)
	cmd.AddCommand(auth.GCloudCmd)
	return cmd
}

// CreateGenerateCommand creates the generate command with app dependencies.
// TODO: Migrate generate command to use app.Context pattern
func (a *App) CreateGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate various artifacts (completion)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add existing subcommands
	cmd.AddCommand(generate.CompletionCmd)
	return cmd
}

// CreateInstallCommand creates the install command with app dependencies.
// TODO: Migrate install command to use app.Context pattern
func (a *App) CreateInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install starmap components",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add existing subcommands
	cmd.AddCommand(install.CompletionCmd)
	return cmd
}

// CreateUninstallCommand creates the uninstall command with app dependencies.
// TODO: Migrate uninstall command to use app.Context pattern
func (a *App) CreateUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall starmap components",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add existing subcommands
	cmd.AddCommand(uninstall.CompletionCmd)
	return cmd
}

// CreateVersionCommand creates the version command.
func (a *App) CreateVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("starmap %s\n", a.version)
			if a.config.Verbose {
				cmd.Printf("  commit:   %s\n", a.commit)
				cmd.Printf("  built:    %s\n", a.date)
				cmd.Printf("  built by: %s\n", a.builtBy)
			}
		},
	}
}

// CreateManCommand creates the man command.
func (a *App) CreateManCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement man page generation
			return nil
		},
	}
}
