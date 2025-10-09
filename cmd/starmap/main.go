// Package main provides the entry point for the starmap CLI tool.
package main

import (
	"context"
	"os"

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
		// Perform graceful shutdown
		_ = application.Shutdown(ctx)
		app.ExitOnError(err)
	}
}
