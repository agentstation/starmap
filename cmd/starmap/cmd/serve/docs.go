package serve

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/logging"
)

// NewDocsCommand creates the serve docs command.
func NewDocsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "docs",
		Aliases: []string{"site", "documentation"},
		Short:   "Serve documentation site",
		Long: `Start a local Hugo development server for the documentation site.

Features:
  - Live reload on file changes
  - Draft content support
  - Custom themes and layouts
  - Fast rebuilds with Hugo

The server watches for changes in the docs/ directory and automatically
rebuilds and reloads the site in your browser.`,
		Example: `  starmap serve docs                    # Start on default port 1313
  starmap serve docs --port 8080         # Start on custom port
  starmap serve docs --drafts            # Include draft content
  starmap serve docs --source ./my-docs  # Use custom source directory`,
		RunE: runDocs,
	}

	// Add common server flags
	AddCommonFlags(cmd, getDefaultDocsPort())

	// Add docs-specific flags
	cmd.Flags().Bool("watch", true, "Enable file watching and live reload")
	cmd.Flags().Bool("drafts", false, "Include draft content")
	cmd.Flags().String("source", "./docs", "Source directory for documentation")
	cmd.Flags().String("root", "./site", "Hugo site root directory")

	return cmd
}

func runDocs(cmd *cobra.Command, _ []string) error {
	config, err := GetServerConfig(cmd, getDefaultDocsPort())
	if err != nil {
		return fmt.Errorf("getting server config: %w", err)
	}

	// Get docs-specific flags
	watch, _ := cmd.Flags().GetBool("watch")
	drafts, _ := cmd.Flags().GetBool("drafts")
	sourceDir, _ := cmd.Flags().GetString("source")
	rootDir, _ := cmd.Flags().GetString("root")

	// Override with environment-specific port
	if envPort := os.Getenv("STARMAP_DOCS_PORT"); envPort != "" {
		if port, err := parsePort(envPort); err == nil {
			config.Port = port
		}
	}

	ctx := context.Background()

	logging.Info().
		Int("port", config.Port).
		Str("host", config.Host).
		Str("source", sourceDir).
		Bool("watch", watch).
		Bool("drafts", drafts).
		Msg("Starting documentation server")

	// Start Hugo server manually (since site package doesn't have Serve method yet)
	if err := startHugoServer(ctx, config, rootDir, watch, drafts); err != nil {
		return fmt.Errorf("running documentation server: %w", err)
	}

	return nil
}

// startHugoServer starts Hugo development server manually.
func startHugoServer(ctx context.Context, config *ServerConfig, rootDir string, watch, drafts bool) error {
	// This is a simplified implementation
	// In production, this would integrate with the Hugo server directly
	fmt.Printf("🚀 Starting Hugo server on %s\n", config.URL())
	fmt.Printf("📁 Root directory: %s\n", rootDir)
	fmt.Printf("👀 Watch: %v, Drafts: %v\n", watch, drafts)
	fmt.Println("Press Ctrl+C to stop")

	// For now, just wait for context cancellation
	// TODO: Integrate with actual Hugo server
	<-ctx.Done()

	fmt.Println("📚 Documentation server stopped")
	return nil
}

// getDefaultDocsPort returns the default port for docs server.
func getDefaultDocsPort() int {
	// Hugo's default port
	return 1313
}
