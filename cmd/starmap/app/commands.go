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
func (a *App) CreateServeCommand() *cobra.Command {
	return serve.NewCommand(a)
}

// CreateFetchCommand creates the fetch command with app dependencies.
func (a *App) CreateFetchCommand() *cobra.Command {
	return fetch.NewCommand(a)
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
func (a *App) CreateValidateCommand() *cobra.Command {
	return validate.NewCommand(a)
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
func (a *App) CreateAuthCommand() *cobra.Command {
	return auth.NewCommand(a)
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
