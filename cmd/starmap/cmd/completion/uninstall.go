package completion

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/completion"
	"github.com/agentstation/starmap/internal/cmd/emoji"
)

// NewUninstallCommand creates the completion uninstall subcommand.
func NewUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove shell completions",
		Long: `Remove shell completions for starmap.

By default, removes completions for all supported shells (bash, zsh, fish).
Use flags to remove from specific shells only.

Examples:
  starmap completion uninstall           # Remove from all shells
  starmap completion uninstall --bash    # Remove from bash only
  starmap completion uninstall --zsh     # Remove from zsh only
  starmap completion uninstall --fish    # Remove from fish only`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			bash := mustGetBool(cmd, "bash")
			zsh := mustGetBool(cmd, "zsh")
			fish := mustGetBool(cmd, "fish")

			// If no specific shell flags are set, uninstall from all shells
			if !bash && !zsh && !fish {
				bash, zsh, fish = true, true, true
			}

			fmt.Printf("Uninstalling shell completions...\n\n")

			var errors []string
			removed := 0

			if bash {
				fmt.Printf("🗑️  Removing bash completions...\n")
				if err := completion.Uninstall("bash"); err != nil {
					errors = append(errors, fmt.Sprintf("bash: %v", err))
				} else {
					removed++
				}
				fmt.Println()
			}

			if zsh {
				fmt.Printf("🗑️  Removing zsh completions...\n")
				if err := completion.Uninstall("zsh"); err != nil {
					errors = append(errors, fmt.Sprintf("zsh: %v", err))
				} else {
					removed++
				}
				fmt.Println()
			}

			if fish {
				fmt.Printf("🗑️  Removing fish completions...\n")
				if err := completion.Uninstall("fish"); err != nil {
					errors = append(errors, fmt.Sprintf("fish: %v", err))
				} else {
					removed++
				}
				fmt.Println()
			}

			if len(errors) > 0 {
				fmt.Printf("%s Some removals failed:\n", emoji.Error)
				for _, err := range errors {
					fmt.Printf("  - %s\n", err)
				}
				if removed > 0 {
					fmt.Printf("\n%s Successfully removed completions from %d shell(s)\n", emoji.Success, removed)
				}
				return fmt.Errorf("failed to remove some completions")
			}

			fmt.Printf("🎉 Successfully removed completions from %d shell(s)!\n", removed)
			fmt.Printf("💡 Start a new shell session to ensure completions are fully removed.\n")
			return nil
		},
	}

	// Shell-specific flags
	cmd.Flags().Bool("bash", false, "Remove bash completions only")
	cmd.Flags().Bool("zsh", false, "Remove zsh completions only")
	cmd.Flags().Bool("fish", false, "Remove fish completions only")

	return cmd
}
