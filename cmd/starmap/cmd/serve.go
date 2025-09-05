package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/tools/site"
	"github.com/agentstation/starmap/pkg/logging"
)

var servePort int

// serveCmd represents the serve command.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve documentation or other resources",
	Long:  `Serve starts a local development server for various resources.`,
}

// serveSiteCmd represents the serve site command.
var serveSiteCmd = &cobra.Command{
	Use:   "site",
	Short: "Run local Hugo development server",
	Long:  `Start a local Hugo development server with live reload for documentation preview.`,
	Example: `  starmap serve site
  starmap serve site --port 8080`,
	RunE: runServeSite,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.AddCommand(serveSiteCmd)

	serveSiteCmd.Flags().IntVar(&servePort, "port", 1313, "Port for development server")
}

func runServeSite(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	logging.Info().
		Int("port", servePort).
		Msg("Starting development server")

	// Create site instance
	config := &site.Config{
		RootDir:    "./site",
		ContentDir: "./docs",
		BaseURL:    fmt.Sprintf("http://localhost:%d/", servePort),
	}

	siteInstance, err := site.New(config)
	if err != nil {
		return fmt.Errorf("creating site: %w", err)
	}

	fmt.Printf("🚀 Starting Hugo server on http://localhost:%d/\n", servePort)
	fmt.Println("Press Ctrl+C to stop")

	// Run the server
	if err := siteInstance.Serve(ctx, servePort); err != nil {
		return fmt.Errorf("running server: %w", err)
	}

	return nil
}
