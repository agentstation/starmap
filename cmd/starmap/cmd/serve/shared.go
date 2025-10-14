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
)

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
