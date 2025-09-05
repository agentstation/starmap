package serve

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/logging"
)

// NewAllCommand creates the serve all command.
func NewAllCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Serve both docs and API servers",
		Long: `Start both documentation and API servers simultaneously.

This command starts both the Hugo documentation server and the REST API
server on different ports. Both servers run concurrently and can be
stopped together with Ctrl+C.

Default Ports:
  - Documentation: 1313
  - API: 8080

Environment Variables:
  STARMAP_DOCS_PORT    - Override docs server port
  STARMAP_API_PORT     - Override API server port`,
		Example: `  starmap serve all                        # Start both on default ports
  starmap serve all --docs-port 3000       # Custom docs port
  starmap serve all --api-port 4000        # Custom API port`,
		RunE: runAll,
	}

	// Add flags for both services
	cmd.Flags().Int("docs-port", getDefaultDocsPort(), "Port for documentation server")
	cmd.Flags().Int("api-port", getDefaultAPIPort(), "Port for API server")
	cmd.Flags().String("host", "localhost", "Host address to bind to")
	cmd.Flags().String("env", "development", "Environment mode")

	// Add service-specific flags
	cmd.Flags().Bool("docs-drafts", false, "Include draft content in docs")
	cmd.Flags().Bool("api-cors", false, "Enable CORS for API")
	cmd.Flags().Bool("api-auth", false, "Enable API key authentication")

	return cmd
}

func runAll(cmd *cobra.Command, _ []string) error {
	// Get port configurations
	docsPort, _ := cmd.Flags().GetInt("docs-port")
	apiPort, _ := cmd.Flags().GetInt("api-port")
	host, _ := cmd.Flags().GetString("host")
	env, _ := cmd.Flags().GetString("env")

	// Get service-specific flags
	docsDrafts, _ := cmd.Flags().GetBool("docs-drafts")
	apiCors, _ := cmd.Flags().GetBool("api-cors")
	apiAuth, _ := cmd.Flags().GetBool("api-auth")

	// Override with environment variables
	if envDocsPort := os.Getenv("STARMAP_DOCS_PORT"); envDocsPort != "" {
		if port, err := parsePort(envDocsPort); err == nil {
			docsPort = port
		}
	}
	if envAPIPort := os.Getenv("STARMAP_API_PORT"); envAPIPort != "" {
		if port, err := parsePort(envAPIPort); err == nil {
			apiPort = port
		}
	}

	// Check for port conflicts
	if docsPort == apiPort {
		return fmt.Errorf("docs and API ports cannot be the same (%d)", docsPort)
	}

	logging.Info().
		Int("docs_port", docsPort).
		Int("api_port", apiPort).
		Str("host", host).
		Str("env", env).
		Msg("Starting both documentation and API servers")

	fmt.Printf("🚀 Starting services:\n")
	fmt.Printf("  📚 Documentation: http://%s:%d\n", host, docsPort)
	fmt.Printf("  🔌 API Server:    http://%s:%d\n", host, apiPort)
	fmt.Println("Press Ctrl+C to stop both services")

	// Create contexts for both servers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use WaitGroup to coordinate both servers
	var wg sync.WaitGroup
	errorChan := make(chan error, 2)

	// Start documentation server
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel() // Cancel the other server if this one fails

		if err := startDocsServer(ctx, docsPort, host, docsDrafts); err != nil {
			errorChan <- fmt.Errorf("docs server failed: %w", err)
		}
	}()

	// Start API server
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel() // Cancel the other server if this one fails

		if err := startAPIServer(ctx, apiPort, host, apiCors, apiAuth); err != nil {
			errorChan <- fmt.Errorf("API server failed: %w", err)
		}
	}()

	// Wait for completion or error
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// Return the first error, if any
	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	fmt.Println("✅ Both servers stopped gracefully")
	return nil
}

// startDocsServer starts the documentation server.
func startDocsServer(ctx context.Context, port int, host string, _ bool) error {
	// This would integrate with the existing Hugo site serving logic
	// For now, this is a placeholder that simulates the docs server

	fmt.Printf("📚 Documentation server started on http://%s:%d\n", host, port)

	// Wait for context cancellation
	<-ctx.Done()

	fmt.Println("📚 Documentation server stopped")
	return nil
}

// startAPIServer starts the API server.
func startAPIServer(ctx context.Context, port int, host string, _, _ bool) error {
	// This would integrate with the API server logic
	// For now, this is a placeholder that simulates the API server

	fmt.Printf("🔌 API server started on http://%s:%d\n", host, port)

	// Wait for context cancellation
	<-ctx.Done()

	fmt.Println("🔌 API server stopped")
	return nil
}
