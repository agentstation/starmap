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

	err = application.Execute(ctx, os.Args[1:])

	// Every exit path stops the connected runtime. The signal context is
	// already canceled, so the shutdown uses a fresh bounded context.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if shutdownErr := application.Shutdown(shutdownCtx); shutdownErr != nil {
		// The command error takes precedence, so the shutdown error only logs.
		application.Logger().Error().Err(shutdownErr).Msg("Shutdown error")
	}

	app.ExitOnError(err)
}
