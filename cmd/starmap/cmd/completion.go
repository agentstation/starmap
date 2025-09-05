// Package cmd provides CLI commands for the starmap tool.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(starmap completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ starmap completion bash > /etc/bash_completion.d/starmap
  # macOS:
  $ starmap completion bash > $(brew --prefix)/etc/bash_completion.d/starmap

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ starmap completion zsh > "${fpath[1]}/_starmap"

  # You will need to start a new shell for this setup to take effect.

Fish:

  $ starmap completion fish | source

  # To load completions for each session, execute once:
  $ starmap completion fish > ~/.config/fish/completions/starmap.fish

To uninstall completions:

  $ starmap completion --uninstall [bash|zsh|fish]
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		uninstall, _ := cmd.Flags().GetBool("uninstall")

		if uninstall {
			return uninstallCompletion(args[0])
		}

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

// uninstallCompletion provides instructions and attempts to remove completion files.
func uninstallCompletion(shell string) error {
	fmt.Printf("Uninstalling %s completions for starmap...\n\n", shell)

	var completionPaths []string
	var instructions string

	switch shell {
	case "bash":
		homeDir, _ := os.UserHomeDir()
		completionPaths = []string{
			"/etc/bash_completion.d/starmap",
			"/usr/local/etc/bash_completion.d/starmap",
			"/opt/homebrew/etc/bash_completion.d/starmap", // Apple Silicon Homebrew
			"/usr/share/bash-completion/completions/starmap",
			filepath.Join(homeDir, ".bash_completion.d", "starmap"),
		}
		instructions = `Manual removal instructions for bash:

  # Check for completion files in common locations and remove them:
  sudo rm -f /etc/bash_completion.d/starmap
  sudo rm -f /usr/local/etc/bash_completion.d/starmap
  sudo rm -f /opt/homebrew/etc/bash_completion.d/starmap
  sudo rm -f /usr/share/bash-completion/completions/starmap
  rm -f ~/.bash_completion.d/starmap

  # Remove any sourcing from ~/.bashrc or ~/.bash_profile if added manually`

	case "zsh":
		instructions = `Manual removal instructions for zsh:

  # Remove completion files from zsh fpath directories:
  rm -f /usr/local/share/zsh/site-functions/_starmap
  rm -f /opt/homebrew/share/zsh/site-functions/_starmap
  rm -f ~/.zsh/completions/_starmap
  
  # Check your fpath and remove _starmap from any of those directories:
  echo $fpath`

	case "fish":
		homeDir, _ := os.UserHomeDir()
		completionPaths = []string{
			filepath.Join(homeDir, ".config", "fish", "completions", "starmap.fish"),
			"/usr/share/fish/completions/starmap.fish",
			"/usr/local/share/fish/completions/starmap.fish",
			"/opt/homebrew/share/fish/completions/starmap.fish",
		}
		instructions = `Manual removal instructions for fish:

  # Remove completion files:
  rm -f ~/.config/fish/completions/starmap.fish
  sudo rm -f /usr/share/fish/completions/starmap.fish
  sudo rm -f /usr/local/share/fish/completions/starmap.fish
  sudo rm -f /opt/homebrew/share/fish/completions/starmap.fish`
	}

	// Attempt automatic removal for user-writable paths
	removed := false
	for _, path := range completionPaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// Try to remove if it exists
			if err := os.Remove(path); err == nil {
				fmt.Printf("✅ Removed: %s\n", path)
				removed = true
			} else {
				fmt.Printf("❌ Could not remove: %s (try: sudo rm %s)\n", path, path)
			}
		}
	}

	if !removed {
		fmt.Println("No completion files found in common user-writable locations.")
	}

	fmt.Printf("\n%s\n", instructions)
	fmt.Println("\n💡 Tip: Start a new shell session to ensure completions are fully removed.")

	return nil
}

func init() {
	rootCmd.AddCommand(completionCmd)
	completionCmd.Flags().Bool("uninstall", false, "Show instructions to uninstall completions")
}
