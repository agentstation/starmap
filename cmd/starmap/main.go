// Package main provides the entry point for the starmap CLI tool.
package main

import (
	"context"
	"os"
	"time"

	"github.com/agentstation/starmap/cmd/starmap/app"
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

	// Create context with signal handling for graceful shutdown
	ctx, cancel := app.ContextWithSignals(context.Background())
	defer cancel()

	// Execute with context
	if err := application.Execute(ctx, os.Args[1:]); err != nil {
		// Perform graceful shutdown with fresh context (signal context may be cancelled)
		// Give shutdown operations 5 seconds to complete
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if shutdownErr := application.Shutdown(shutdownCtx); shutdownErr != nil {
			// Log shutdown error to stderr, but don't let it mask the original error
			application.Logger().Error().Err(shutdownErr).Msg("Shutdown error during error handling")
		}
		app.ExitOnError(err)
	}
}
