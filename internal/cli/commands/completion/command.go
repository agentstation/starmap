// Package completion provides shell completion management commands.
package completion

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the completion command with install/uninstall subcommands.
// This overrides Cobra's auto-generated completion command to add our custom subcommands.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Manage shell completions",
		Long: `Manage shell completions for starmap.

You can generate completion scripts to stdout, or use the install/uninstall
subcommands to automatically set up completions for your shell.

Examples:
  # Generate bash completion to stdout
  starmap completion bash

  # Install completions for all shells
  starmap completion install

  # Install for specific shell
  starmap completion install --bash

  # Uninstall completions
  starmap completion uninstall`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add Cobra's built-in shell-specific generation commands
	// These generate completion scripts to stdout
	cmd.AddCommand(newBashCommand())
	cmd.AddCommand(newZshCommand())
	cmd.AddCommand(newFishCommand())
	cmd.AddCommand(newPowershellCommand())

	// Add our custom install/uninstall commands
	cmd.AddCommand(NewInstallCommand())
	cmd.AddCommand(NewUninstallCommand())

	return cmd
}

// newBashCommand creates the bash completion generation subcommand.
func newBashCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		Long: `Generate the autocompletion script for bash.

To load completions in your current shell session:

  source <(starmap completion bash)

To load completions for every new session, execute once:

  # Linux:
  starmap completion bash > /etc/bash_completion.d/starmap

  # macOS:
  starmap completion bash > $(brew --prefix)/etc/bash_completion.d/starmap

You can also use "starmap completion install" to automatically install.`,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		},
	}
}

// newZshCommand creates the zsh completion generation subcommand.
func newZshCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		Long: `Generate the autocompletion script for zsh.

To load completions in your current shell session:

  source <(starmap completion zsh)

To load completions for every new session, execute once:

  starmap completion zsh > "${fpath[1]}/_starmap"

You will need to start a new shell for this setup to take effect.

You can also use "starmap completion install" to automatically install.`,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		},
	}
}

// newFishCommand creates the fish completion generation subcommand.
func newFishCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		Long: `Generate the autocompletion script for fish.

To load completions in your current shell session:

  starmap completion fish | source

To load completions for every new session, execute once:

  starmap completion fish > ~/.config/fish/completions/starmap.fish

You will need to start a new shell for this setup to take effect.

You can also use "starmap completion install" to automatically install.`,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		},
	}
}

// newPowershellCommand creates the powershell completion generation subcommand.
func newPowershellCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "powershell",
		Short: "Generate powershell completion script",
		Long: `Generate the autocompletion script for powershell.

To load completions in your current shell session:

  starmap completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.`,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		},
	}
}
