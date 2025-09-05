package serve

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// ServerConfig holds common server configuration.
type ServerConfig struct {
	Port        int
	Host        string
	Environment string
	ConfigFile  string
}

// GetServerConfig extracts common server configuration from command flags and environment.
func GetServerConfig(cmd *cobra.Command, defaultPort int) (*ServerConfig, error) {
	port, _ := cmd.Flags().GetInt("port")
	host, _ := cmd.Flags().GetString("host")
	env, _ := cmd.Flags().GetString("env")
	config, _ := cmd.Flags().GetString("config")

	// Use default port if not specified
	if port == 0 {
		port = defaultPort
	}

	// Override with PORT environment variable if set
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	// Override with HOST environment variable if set
	if envHost := os.Getenv("HOST"); envHost != "" {
		host = envHost
	}

	return &ServerConfig{
		Port:        port,
		Host:        host,
		Environment: env,
		ConfigFile:  config,
	}, nil
}

// Address returns the full address string for binding.
func (c *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// URL returns the full URL for the server.
func (c *ServerConfig) URL() string {
	hostname := c.Host
	if hostname == "" || hostname == "0.0.0.0" {
		hostname = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", hostname, c.Port)
}

// AddCommonFlags adds common server flags to a command.
func AddCommonFlags(cmd *cobra.Command, defaultPort int) {
	cmd.Flags().IntP("port", "p", defaultPort, "Port to bind server to")
	cmd.Flags().String("host", "localhost", "Host address to bind to")
	cmd.Flags().String("env", "development", "Environment mode (development, production)")
	cmd.Flags().String("config", "", "Configuration file path")
}

// StartServerWithGracefulShutdown starts an HTTP server with graceful shutdown.
func StartServerWithGracefulShutdown(server *http.Server, serviceName string) error {
	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		fmt.Printf("🚀 Starting %s server on %s\n", serviceName, server.Addr)
		fmt.Println("Press Ctrl+C to stop")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case <-quit:
		fmt.Printf("\n🛑 Shutting down %s server...\n", serviceName)

		// Give outstanding requests a deadline to complete
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown server gracefully
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("server forced to shutdown: %w", err)
		}

		fmt.Printf("✅ %s server stopped gracefully\n", serviceName)
		return nil
	}
}

// parsePort safely parses a port string to integer.
func parsePort(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %s", portStr)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port out of range: %d", port)
	}
	return port, nil
}
