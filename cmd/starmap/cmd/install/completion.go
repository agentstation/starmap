// Package install provides commands for installing starmap components.
package install

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/application"
	"github.com/agentstation/starmap/internal/cmd/completion"
)

// NewCompletionCommand creates the install completion subcommand using app context.
func NewCompletionCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
	Use:   "completion",
	Short: "Install shell completions",
	Long: `Install shell completions for starmap.

By default, installs completions for all supported shells (bash, zsh, fish).
Use flags to install for specific shells only.

Examples:
  starmap install completion           # Install for all shells
  starmap install completion --bash    # Install for bash only  
  starmap install completion --zsh     # Install for zsh only
  starmap install completion --fish    # Install for fish only`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		bash, _ := cmd.Flags().GetBool("bash")
		zsh, _ := cmd.Flags().GetBool("zsh")
		fish, _ := cmd.Flags().GetBool("fish")

		// If no specific shell flags are set, install for all shells
		if !bash && !zsh && !fish {
			bash, zsh, fish = true, true, true
		}

		fmt.Printf("Installing shell completions...\n\n")

		var errors []string
		installed := 0

		if bash {
			fmt.Printf("🐚 Installing bash completions...\n")
			if err := completion.Install(cmd.Root(), "bash"); err != nil {
				errors = append(errors, fmt.Sprintf("bash: %v", err))
			} else {
				installed++
			}
			fmt.Println()
		}

		if zsh {
			fmt.Printf("🐚 Installing zsh completions...\n")
			if err := completion.Install(cmd.Root(), "zsh"); err != nil {
				errors = append(errors, fmt.Sprintf("zsh: %v", err))
			} else {
				installed++
			}
			fmt.Println()
		}

		if fish {
			fmt.Printf("🐚 Installing fish completions...\n")
			if err := completion.Install(cmd.Root(), "fish"); err != nil {
				errors = append(errors, fmt.Sprintf("fish: %v", err))
			} else {
				installed++
			}
			fmt.Println()
		}

		if len(errors) > 0 {
			fmt.Printf("❌ Some installations failed:\n")
			for _, err := range errors {
				fmt.Printf("  - %s\n", err)
			}
			if installed > 0 {
				fmt.Printf("\n✅ Successfully installed completions for %d shell(s)\n", installed)
			}
			return fmt.Errorf("failed to install some completions")
		}

		fmt.Printf("🎉 Successfully installed completions for %d shell(s)!\n", installed)
		fmt.Printf("💡 Start a new shell session or reload your shell config to enable completions.\n")
		return nil
	},
	}

	// Shell-specific flags
	cmd.Flags().Bool("bash", false, "Install bash completions only")
	cmd.Flags().Bool("zsh", false, "Install zsh completions only")
	cmd.Flags().Bool("fish", false, "Install fish completions only")

	return cmd
}
