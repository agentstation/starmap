package app

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/authors"
	"github.com/agentstation/starmap/cmd/starmap/cmd/completion"
	"github.com/agentstation/starmap/cmd/starmap/cmd/deps"
	"github.com/agentstation/starmap/cmd/starmap/cmd/embed"
	"github.com/agentstation/starmap/cmd/starmap/cmd/models"
	"github.com/agentstation/starmap/cmd/starmap/cmd/providers"
	"github.com/agentstation/starmap/cmd/starmap/cmd/serve"
	"github.com/agentstation/starmap/cmd/starmap/cmd/update"
	"github.com/agentstation/starmap/cmd/starmap/cmd/validate"
)

// NewProvidersCommand returns a new providers command with app dependencies.
func (a *App) NewProvidersCommand() *cobra.Command {
	return providers.NewCommand(a)
}

// NewModelsCommand returns a new models command with app dependencies.
func (a *App) NewModelsCommand() *cobra.Command {
	return models.NewCommand(a)
}

// NewAuthorsCommand returns a new authors command with app dependencies.
func (a *App) NewAuthorsCommand() *cobra.Command {
	return authors.NewCommand(a)
}

// NewUpdateCommand returns a new update command with app dependencies.
func (a *App) NewUpdateCommand() *cobra.Command {
	return update.NewCommand(a)
}

// NewServeCommand returns a new serve command with app dependencies.
func (a *App) NewServeCommand() *cobra.Command {
	return serve.NewCommand(a)
}

// NewValidateCommand returns a new validate command with app dependencies.
func (a *App) NewValidateCommand() *cobra.Command {
	return validate.NewCommand(a)
}

// NewEmbedCommand returns a new embed command with app dependencies.
func (a *App) NewEmbedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "embed",
		GroupID: "development",
		Aliases: []string{"inspect"},
		Short:   "Explore embedded filesystem",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Define custom help flag for ALL embed subcommands to free up -h and -f
	// This allows embed subcommands to use -h (ls: human-readable) and -f (cat: filename)
	cmd.PersistentFlags().BoolP("help", "?", false, "help for embed commands")

	// Add existing subcommands
	cmd.AddCommand(embed.LsCmd)
	cmd.AddCommand(embed.CatCmd)
	cmd.AddCommand(embed.TreeCmd)
	cmd.AddCommand(embed.StatCmd)
	return cmd
}

// NewDepsCommand returns a new deps command with app dependencies.
func (a *App) NewDepsCommand() *cobra.Command {
	return deps.NewCommand()
}

// NewCompletionCommand returns a new completion command.
// This overrides Cobra's auto-generated completion command to add install/uninstall subcommands.
func (a *App) NewCompletionCommand() *cobra.Command {
	return completion.NewCommand()
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
