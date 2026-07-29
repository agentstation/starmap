package app

import (
	"context"
	"os/signal"
	"syscall"
)

// ContextWithSignals creates a context that is cancelled when the application
// receives an interrupt or termination signal. This enables graceful shutdown.
func ContextWithSignals(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}

// Context creates a new context with signal handling for the application.
// This is a convenience wrapper around ContextWithSignals that uses
// context.Background() as the parent.
func Context() (context.Context, context.CancelFunc) {
	return ContextWithSignals(context.Background())
}
