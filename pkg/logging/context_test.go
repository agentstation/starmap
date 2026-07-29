package logging_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/logging"
)

func TestContextLoggerAndCorrelation(t *testing.T) {
	logger := zerolog.Nop()
	ctx := logging.WithLogger(context.Background(), &logger)
	ctx = logging.WithProvider(ctx, "openai")
	ctx = logging.WithSource(ctx, "provider-api")
	ctx = logging.WithRunID(ctx, "run-123")

	if logging.FromContext(ctx) == nil {
		t.Fatal("FromContext returned nil")
	}
	if logging.Ctx(ctx) == nil {
		t.Fatal("Ctx returned nil")
	}
	if got := logging.RunID(ctx); got != "run-123" {
		t.Fatalf("RunID = %q, want run-123", got)
	}
}
