package deps

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// PromptResult represents the user's response to a dependency prompt.
type PromptResult int

const (
	// PromptInstall indicates user wants to install the dependency.
	PromptInstall PromptResult = iota
	// PromptSkip indicates user wants to skip this dependency.
	PromptSkip
	// PromptCancel indicates user wants to cancel the entire operation.
	PromptCancel
)

// PromptOptions configures how prompts are displayed and handled.
type PromptOptions struct {
	// AutoInstall automatically installs dependencies without prompting.
	AutoInstall bool
	// SkipPrompts skips all prompts and dependencies.
	SkipPrompts bool
	// RequireAll requires all sources to succeed (fail if any deps missing).
	RequireAll bool
}

// PromptForMissingDep asks the user what to do about a missing dependency.
// Returns PromptInstall, PromptSkip, or PromptCancel.
func PromptForMissingDep(dep sources.Dependency, sourceName string) PromptResult {
	fmt.Printf("\n⚠️  Missing Dependency: %s\n", dep.DisplayName)
	fmt.Printf("   Required by: %s\n", sourceName)
	fmt.Printf("   Description: %s\n", dep.Description)

	if dep.AlternativeSource != "" {
		fmt.Printf("   Alternative: %s\n", dep.AlternativeSource)
	}

	if dep.WhyNeeded != "" {
		fmt.Printf("   Why needed: %s\n", dep.WhyNeeded)
	}

	fmt.Println()

	// Show installation instructions
	if dep.AutoInstallCommand != "" {
		fmt.Printf("This dependency can be installed automatically.\n")
		fmt.Printf("Would you like to install %s now? [y/N/cancel] ", dep.DisplayName)
	} else {
		fmt.Printf("To install %s:\n", dep.DisplayName)
		fmt.Printf("  Visit: %s\n\n", dep.InstallURL)
		fmt.Printf("Would you like to continue without %s? [y/N/cancel] ", dep.DisplayName)
	}

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return PromptCancel
	}

	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "y", "yes":
		if dep.AutoInstallCommand != "" {
			return PromptInstall
		}
		return PromptSkip
	case "n", "no", "":
		return PromptSkip
	case "c", "cancel":
		return PromptCancel
	default:
		return PromptSkip
	}
}

// AutoInstall attempts to automatically install a dependency using its AutoInstallCommand.
func AutoInstall(ctx context.Context, dep sources.Dependency) error {
	if dep.AutoInstallCommand == "" {
		return fmt.Errorf("no auto-install command configured for %s", dep.DisplayName)
	}

	logger := logging.FromContext(ctx)
	logger.Info().
		Str("dependency", dep.Name).
		Str("command", dep.AutoInstallCommand).
		Msg("Auto-installing dependency")

	fmt.Printf("\n🔄 Installing %s...\n", dep.DisplayName)

	// Execute the install command via shell
	//nolint:gosec // AutoInstallCommand comes from Dependency struct (trusted source code)
	cmd := exec.CommandContext(ctx, "sh", "-c", dep.AutoInstallCommand)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Installation failed: %v\n", err)
		return fmt.Errorf("failed to install %s: %w", dep.DisplayName, err)
	}

	fmt.Printf("✅ %s installed successfully\n", dep.DisplayName)

	// Verify installation
	status := Check(ctx, dep)
	if !status.Available {
		fmt.Printf("⚠️  Warning: %s was installed but is not yet available in PATH\n", dep.DisplayName)
		fmt.Printf("   You may need to restart your shell or update your PATH\n")
		return fmt.Errorf("%s not available after installation", dep.DisplayName)
	}

	if status.Version != "" {
		fmt.Printf("   Version: %s\n", status.Version)
	}

	return nil
}

// ShowInstallInstructions displays installation instructions for a dependency.
func ShowInstallInstructions(dep sources.Dependency) {
	fmt.Printf("\n📦 %s is required\n", dep.DisplayName)
	fmt.Printf("   Description: %s\n", dep.Description)

	if dep.WhyNeeded != "" {
		fmt.Printf("   Why needed: %s\n", dep.WhyNeeded)
	}

	if dep.AlternativeSource != "" {
		fmt.Printf("   Alternative: %s\n", dep.AlternativeSource)
	}

	fmt.Println()
	fmt.Printf("To install %s:\n", dep.DisplayName)

	if dep.AutoInstallCommand != "" {
		fmt.Printf("  Automatic: %s\n", dep.AutoInstallCommand)
	}

	if dep.InstallURL != "" {
		fmt.Printf("  Manual: %s\n", dep.InstallURL)
	}

	fmt.Println()
}

// ShowMissingDepsSummary shows a summary of all missing dependencies.
func ShowMissingDepsSummary(deps []sources.Dependency, sourceName string) {
	if len(deps) == 0 {
		return
	}

	fmt.Printf("\n⚠️  %s requires the following dependencies:\n\n", sourceName)

	for _, dep := range deps {
		fmt.Printf("  • %s - %s\n", dep.DisplayName, dep.Description)
		if dep.InstallURL != "" {
			fmt.Printf("    Install: %s\n", dep.InstallURL)
		}
	}

	fmt.Println()
}

// ConfirmSkipSource asks the user if they want to skip a source with missing dependencies.
func ConfirmSkipSource(sourceName string, missingDeps []sources.Dependency) bool {
	fmt.Printf("\n⚠️  Cannot use %s due to missing dependencies:\n", sourceName)
	for _, dep := range missingDeps {
		fmt.Printf("  • %s\n", dep.DisplayName)
	}

	fmt.Printf("\nWould you like to skip %s? [Y/n] ", sourceName)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return true // Default to skip on error
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "" || response == "y" || response == "yes"
}
