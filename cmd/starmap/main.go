// Package main provides the entry point for the starmap CLI tool.
package main

import (
	"context"
	"os"
	"time"

	"github.com/agentstation/starmap/internal/cli/app"
)

// Version information populated by goreleaser.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	// Create app instance
	application, err := app.New(version, commit, date, builtBy)
	if err != nil {
		app.ExitOnError(err)
	}

	// Handle signals for graceful shutdown.
	ctx, cancel := app.ContextWithSignals(context.Background())
	defer cancel()

	if err := application.Execute(ctx, os.Args[1:]); err != nil {
		// The signal context is already canceled, so use a fresh shutdown context.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if shutdownErr := application.Shutdown(shutdownCtx); shutdownErr != nil {
			// Log shutdown error but do not return it - original error takes precedence
			application.Logger().Error().Err(shutdownErr).Msg("Shutdown error during error handling")
		}

		// Exit with original error, not shutdown error
		app.ExitOnError(err)
	}
}
