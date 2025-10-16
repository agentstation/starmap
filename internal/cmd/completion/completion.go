// Package completion provides shared utilities for completion management.
package completion

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	pkgconstants "github.com/agentstation/starmap/pkg/constants"
)

// Install installs completion files to appropriate system locations.
func Install(cmd *cobra.Command, shell string) error {
	fmt.Printf("Installing %s completions for starmap...\n", shell)

	var targetPath string
	var err error

	switch shell {
	case constants.ShellBash:
		targetPath, err = GetBashPath()
		if err != nil {
			return fmt.Errorf("failed to determine bash completion path: %w", err)
		}

		// Create parent directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(targetPath), pkgconstants.DirPermissions); err != nil {
			return fmt.Errorf("failed to create completion directory: %w", err)
		}

		file, err := os.Create(targetPath) // #nosec G304 - Path comes from GetBashPath() which generates controlled paths
		if err != nil {
			return fmt.Errorf("failed to create completion file: %w", err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Printf("Warning: failed to close file: %v\n", closeErr)
			}
		}()

		if err = cmd.GenBashCompletion(file); err != nil {
			return fmt.Errorf("failed to generate bash completion: %w", err)
		}

	case constants.ShellZsh:
		targetPath, err = GetZshPath()
		if err != nil {
			return fmt.Errorf("failed to determine zsh completion path: %w", err)
		}

		// Create parent directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(targetPath), pkgconstants.DirPermissions); err != nil {
			return fmt.Errorf("failed to create completion directory: %w", err)
		}

		file, err := os.Create(targetPath) // #nosec G304 - Path comes from GetZshPath() which generates controlled paths
		if err != nil {
			return fmt.Errorf("failed to create completion file: %w", err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Printf("Warning: failed to close file: %v\n", closeErr)
			}
		}()

		if err = cmd.GenZshCompletion(file); err != nil {
			return fmt.Errorf("failed to generate zsh completion: %w", err)
		}

	case constants.ShellFish:
		targetPath, err = GetFishPath()
		if err != nil {
			return fmt.Errorf("failed to determine fish completion path: %w", err)
		}

		// Create parent directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(targetPath), pkgconstants.DirPermissions); err != nil {
			return fmt.Errorf("failed to create completion directory: %w", err)
		}

		file, err := os.Create(targetPath) // #nosec G304 - Path comes from GetFishPath() which generates controlled paths
		if err != nil {
			return fmt.Errorf("failed to create completion file: %w", err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Printf("Warning: failed to close file: %v\n", closeErr)
			}
		}()

		if err = cmd.GenFishCompletion(file, true); err != nil {
			return fmt.Errorf("failed to generate fish completion: %w", err)
		}

	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	fmt.Printf(emoji.Success+" %s completions installed to: %s\n", shell, targetPath)
	fmt.Printf("💡 Start a new shell session or reload your shell config to enable completions.\n")

	return nil
}

// Uninstall removes completion files from the same locations where Install puts them.
func Uninstall(shell string) error {
	fmt.Printf("Uninstalling %s completions for starmap...\n", shell)

	var targetPath string
	var err error

	// Use the same path logic as installation
	switch shell {
	case constants.ShellBash:
		targetPath, err = GetBashPath()
	case constants.ShellZsh:
		targetPath, err = GetZshPath()
	case constants.ShellFish:
		targetPath, err = GetFishPath()
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	if err != nil {
		return fmt.Errorf("failed to determine completion path: %w", err)
	}

	// Check if file exists and remove it
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		if err := os.Remove(targetPath); err != nil {
			// If we can't remove it (permission issue), provide manual instructions
			fmt.Printf(emoji.Error+" Could not remove: %s\n", targetPath)
			fmt.Printf("💡 Try manually: sudo rm -f %s\n", targetPath)
			return nil
		}
		fmt.Printf(emoji.Success+" Removed %s completions from: %s\n", shell, targetPath)
	} else {
		fmt.Printf("ℹ️  No %s completions found at: %s\n", shell, targetPath)

		// Also check other common locations as fallback
		fmt.Printf("🔍 Checking other common locations...\n")
		removed := checkAndRemoveFromCommonPaths(shell)
		if !removed {
			fmt.Printf("ℹ️  No completion files found in common locations.\n")
		}
	}

	fmt.Printf("💡 Start a new shell session to ensure completions are fully removed.\n")
	return nil
}

// UninstallAll removes completion files for all supported shells.
func UninstallAll() error {
	fmt.Printf("Uninstalling completions for all shells...\n\n")

	shells := []string{"bash", "zsh", "fish"}
	var errors []string

	for _, shell := range shells {
		fmt.Printf("🗑️  Removing %s completions...\n", shell)
		if err := Uninstall(shell); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", shell, err))
		}
		fmt.Println() // Add spacing between shells
	}

	if len(errors) > 0 {
		fmt.Printf(emoji.Error + " Some errors occurred:\n")
		for _, err := range errors {
			fmt.Printf("  - %s\n", err)
		}
		return fmt.Errorf("failed to uninstall some completions")
	}

	fmt.Printf("🎉 All completions successfully removed!\n")
	fmt.Printf("💡 Start a new shell session to ensure completions are fully removed.\n")
	return nil
}

// GetBashPath returns the appropriate bash completion path.
func GetBashPath() (string, error) {
	// Try Homebrew first (most common on macOS)
	if brewPrefix := os.Getenv("HOMEBREW_PREFIX"); brewPrefix != "" {
		return filepath.Join(brewPrefix, "etc", "bash_completion.d", "starmap"), nil
	}

	// Try to detect Homebrew
	for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
		if _, err := os.Stat(filepath.Join(prefix, "bin", "brew")); err == nil {
			return filepath.Join(prefix, "etc", "bash_completion.d", "starmap"), nil
		}
	}

	// Fall back to user directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".bash_completion.d", "starmap"), nil
}

// GetZshPath returns the appropriate zsh completion path.
func GetZshPath() (string, error) {
	// Try Homebrew first
	if brewPrefix := os.Getenv("HOMEBREW_PREFIX"); brewPrefix != "" {
		return filepath.Join(brewPrefix, "share", "zsh", "site-functions", "_starmap"), nil
	}

	// Try to detect Homebrew
	for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
		if _, err := os.Stat(filepath.Join(prefix, "bin", "brew")); err == nil {
			return filepath.Join(prefix, "share", "zsh", "site-functions", "_starmap"), nil
		}
	}

	// Fall back to user directory (less reliable for zsh)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".zsh", "completions", "_starmap"), nil
}

// GetFishPath returns the appropriate fish completion path.
func GetFishPath() (string, error) {
	// Try Homebrew first
	if brewPrefix := os.Getenv("HOMEBREW_PREFIX"); brewPrefix != "" {
		return filepath.Join(brewPrefix, "share", "fish", "vendor_completions.d", "starmap.fish"), nil
	}

	// Try to detect Homebrew
	for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
		if _, err := os.Stat(filepath.Join(prefix, "bin", "brew")); err == nil {
			return filepath.Join(prefix, "share", "fish", "vendor_completions.d", "starmap.fish"), nil
		}
	}

	// Fall back to user directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "fish", "completions", "starmap.fish"), nil
}

// checkAndRemoveFromCommonPaths checks and removes completion files from common fallback locations.
func checkAndRemoveFromCommonPaths(shell string) bool {
	var commonPaths []string

	switch shell {
	case constants.ShellBash:
		homeDir, _ := os.UserHomeDir()
		commonPaths = []string{
			"/etc/bash_completion.d/starmap",
			"/usr/local/etc/bash_completion.d/starmap",
			"/opt/homebrew/etc/bash_completion.d/starmap",
			"/usr/share/bash-completion/completions/starmap",
			filepath.Join(homeDir, ".bash_completion.d", "starmap"),
		}
	case constants.ShellZsh:
		homeDir, _ := os.UserHomeDir()
		commonPaths = []string{
			"/usr/local/share/zsh/site-functions/_starmap",
			"/opt/homebrew/share/zsh/site-functions/_starmap",
			filepath.Join(homeDir, ".zsh", "completions", "_starmap"),
		}
	case constants.ShellFish:
		homeDir, _ := os.UserHomeDir()
		commonPaths = []string{
			filepath.Join(homeDir, ".config", "fish", "completions", "starmap.fish"),
			"/usr/share/fish/completions/starmap.fish",
			"/usr/local/share/fish/completions/starmap.fish",
			"/opt/homebrew/share/fish/completions/starmap.fish",
			"/opt/homebrew/share/fish/vendor_completions.d/starmap.fish",
		}
	}

	removed := false
	for _, path := range commonPaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if err := os.Remove(path); err == nil {
				fmt.Printf(emoji.Success+" Removed: %s\n", path)
				removed = true
			} else {
				fmt.Printf(emoji.Error+" Could not remove: %s (try: sudo rm %s)\n", path, path)
			}
		}
	}

	return removed
}
