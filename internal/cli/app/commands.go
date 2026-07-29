package app

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/agentstation/starmap/internal/cli/commands/auth"
	"github.com/agentstation/starmap/internal/cli/commands/authors"
	"github.com/agentstation/starmap/internal/cli/commands/completion"
	"github.com/agentstation/starmap/internal/cli/commands/deps"
	"github.com/agentstation/starmap/internal/cli/commands/embed"
	"github.com/agentstation/starmap/internal/cli/commands/migrate"
	"github.com/agentstation/starmap/internal/cli/commands/models"
	"github.com/agentstation/starmap/internal/cli/commands/providers"
	"github.com/agentstation/starmap/internal/cli/commands/serve"
	"github.com/agentstation/starmap/internal/cli/commands/update"
	"github.com/agentstation/starmap/internal/cli/commands/validate"
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

// NewAuthCommand returns a new auth command with app dependencies.
func (a *App) NewAuthCommand() *cobra.Command {
	return auth.NewCommand()
}

// NewMigrateCommand returns the explicit local-storage migration command.
func (a *App) NewMigrateCommand() *cobra.Command {
	return migrate.NewCommand(a)
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
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "starmap %s\n", a.version)
			if a.config.Verbose {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  commit:   %s\n", a.commit)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  built:    %s\n", a.date)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  built by: %s\n", a.builtBy)
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
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			header := &doc.GenManHeader{
				Title:   "STARMAP",
				Section: "1",
				Source:  "Starmap",
				Manual:  "Starmap Manual",
			}
			return doc.GenMan(cmd.Root(), header, cmd.OutOrStdout())
		},
	}
}
