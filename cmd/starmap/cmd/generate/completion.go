package generate

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/context"
	"github.com/agentstation/starmap/internal/cmd/completion"
)

// NewCompletionCommand creates the generate completion subcommand using app context.
func NewCompletionCommand(appCtx context.Context) *cobra.Command {
	cmd := &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for starmap.

💡 TIP: For easier installation, use the install/uninstall commands:
  starmap install completion    # Install for all shells
  starmap uninstall completion  # Remove from all shells

To load completions:

Bash:

  $ source <(starmap generate completion bash)

  # To install completions permanently:
  $ starmap generate completion bash --install

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To install completions permanently:
  $ starmap generate completion zsh --install

Fish:

  $ starmap generate completion fish | source

  # To install completions permanently:
  $ starmap generate completion fish --install

To uninstall completions:

  # Remove completions for a specific shell:
  $ starmap generate completion bash --uninstall
  $ starmap generate completion zsh --uninstall  
  $ starmap generate completion fish --uninstall

  # Remove completions for all shells:
  $ starmap generate completion --uninstall

Advanced usage:

  # Generate to stdout (for manual installation):
  $ starmap generate completion bash > /path/to/completions
  $ starmap generate completion zsh > "${fpath[1]}/_starmap"
  $ starmap generate completion fish > ~/.config/fish/completions/starmap.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.MatchAll(cobra.RangeArgs(0, 1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		uninstall, _ := cmd.Flags().GetBool("uninstall")
		install, _ := cmd.Flags().GetBool("install")

		if uninstall {
			if len(args) == 0 {
				// Uninstall completions for all shells
				return completion.UninstallAll()
			}
			return completion.Uninstall(args[0])
		}

		if install {
			if len(args) == 0 {
				return fmt.Errorf("shell argument required for --install (bash|zsh|fish)")
			}
			return completion.Install(cmd, args[0])
		}

		// Shell argument is required for default behavior (generate to stdout)
		if len(args) == 0 {
			return fmt.Errorf("shell argument required (bash|zsh|fish)")
		}

		// Default: generate to stdout
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		}
		return nil
	},
	}

	cmd.Flags().Bool("install", false, "Install completions to system location")
	cmd.Flags().Bool("uninstall", false, "Remove completions from system location")

	return cmd
}
