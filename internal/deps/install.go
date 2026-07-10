package deps

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// AutoInstall attempts to install a dependency using its trusted source
// declaration, then verifies that the command is available.
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

	//nolint:gosec // AutoInstallCommand comes from a trusted source declaration.
	cmd := exec.CommandContext(ctx, "sh", "-c", dep.AutoInstallCommand)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Installation failed: %v\n", err)
		return fmt.Errorf("failed to install %s: %w", dep.DisplayName, err)
	}

	fmt.Printf("✅ %s installed successfully\n", dep.DisplayName)

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
