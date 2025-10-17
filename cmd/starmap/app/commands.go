package app

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/auth"
	"github.com/agentstation/starmap/cmd/starmap/cmd/deps"
	"github.com/agentstation/starmap/cmd/starmap/cmd/embed"
	"github.com/agentstation/starmap/cmd/starmap/cmd/fetch"
	"github.com/agentstation/starmap/cmd/starmap/cmd/generate"
	"github.com/agentstation/starmap/cmd/starmap/cmd/install"
	"github.com/agentstation/starmap/cmd/starmap/cmd/list"
	"github.com/agentstation/starmap/cmd/starmap/cmd/serve"
	"github.com/agentstation/starmap/cmd/starmap/cmd/uninstall"
	"github.com/agentstation/starmap/cmd/starmap/cmd/update"
	"github.com/agentstation/starmap/cmd/starmap/cmd/validate"
)

// NewListCommand returns a new list command with app dependencies.
func (a *App) NewListCommand() *cobra.Command {
	return list.NewCommand(a)
}

// NewUpdateCommand returns a new update command with app dependencies.
func (a *App) NewUpdateCommand() *cobra.Command {
	return update.NewCommand(a)
}

// NewServeCommand returns a new serve command with app dependencies.
func (a *App) NewServeCommand() *cobra.Command {
	return serve.NewCommand(a)
}

// NewFetchCommand returns a new fetch command with app dependencies.
func (a *App) NewFetchCommand() *cobra.Command {
	return fetch.NewCommand(a)
}

// NewValidateCommand returns a new validate command with app dependencies.
func (a *App) NewValidateCommand() *cobra.Command {
	return validate.NewCommand(a)
}

// NewEmbedCommand returns a new embed command with app dependencies.
func (a *App) NewEmbedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "embed",
		Aliases: []string{"inspect"},
		Short:   "Explore embedded filesystem",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	// Add existing subcommands
	cmd.AddCommand(embed.LsCmd)
	cmd.AddCommand(embed.CatCmd)
	cmd.AddCommand(embed.TreeCmd)
	cmd.AddCommand(embed.StatCmd)
	return cmd
}

// NewAuthCommand returns a new auth command with app dependencies.
func (a *App) NewAuthCommand() *cobra.Command {
	return auth.NewCommand(a)
}

// NewDepsCommand returns a new deps command with app dependencies.
func (a *App) NewDepsCommand() *cobra.Command {
	return deps.NewCommand(a)
}

// NewGenerateCommand returns a new generate command with app dependencies.
func (a *App) NewGenerateCommand() *cobra.Command {
	return generate.NewCommand(a)
}

// NewInstallCommand returns a new install command with app dependencies.
func (a *App) NewInstallCommand() *cobra.Command {
	return install.NewCommand(a)
}

// NewUninstallCommand returns a new uninstall command with app dependencies.
func (a *App) NewUninstallCommand() *cobra.Command {
	return uninstall.NewCommand(a)
}

// NewVersionCommand returns a new version command.
func (a *App) NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Printf("starmap %s\n", a.version)
			if a.config.Verbose {
				cmd.Printf("  commit:   %s\n", a.commit)
				cmd.Printf("  built:    %s\n", a.date)
				cmd.Printf("  built by: %s\n", a.builtBy)
			}
		},
	}
}

// NewManCommand returns a new man command.
func (a *App) NewManCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			// TODO: Implement man page generation
			return nil
		},
	}
}
