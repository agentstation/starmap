package logging_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/logging"
)

func TestDefaultLogger(t *testing.T) {
	// Create a buffer to capture output
	buf := &bytes.Buffer{}
	logger := zerolog.New(buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	logging.SetDefault(logger)

	// Test logging at different levels
	logging.Debug().Msg("debug message")
	logging.Info().Msg("info message")
	logging.Warn().Msg("warning message")
	logging.Error().Msg("error message")

	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Errorf("Expected info message in output, got: %s", output)
	}
}

func TestContextLogger(t *testing.T) {
	// Create test logger
	testLogger := logging.NewTestLogger(t)

	// Create context with logger
	ctx := logging.WithLogger(context.Background(), testLogger.Logger)

	// Add fields to context
	ctx = logging.WithProvider(ctx, "test-provider")
	ctx = logging.WithModel(ctx, "test-model")

	// Get logger from context and log
	logger := logging.FromContext(ctx)
	logger.Info().Msg("test message")

	// Verify output contains expected fields
	testLogger.AssertContains(t, "test-provider")
	testLogger.AssertContains(t, "test-model")
	testLogger.AssertContains(t, "test message")
}

func TestConfiguration(t *testing.T) {
	// Test different configurations
	configs := []struct {
		name   string
		config *logging.Config
		check  func(t *testing.T, output string)
	}{
		{
			name: "debug level",
			config: &logging.Config{
				Level:  "debug",
				Format: "json",
			},
			check: func(t *testing.T, output string) {
				if !strings.Contains(output, `"level":"debug"`) {
					t.Errorf("Expected debug level in output")
				}
			},
		},
		{
			name: "error level only",
			config: &logging.Config{
				Level:  "error",
				Format: "json",
			},
			check: func(t *testing.T, output string) {
				if strings.Contains(output, `"level":"info"`) {
					t.Errorf("Should not contain info level when set to error")
				}
			},
		},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := logging.NewLoggerFromConfig(tc.config)
			logger = logger.Output(buf)

			logger.Debug().Msg("debug")
			logger.Info().Msg("info")
			logger.Error().Msg("error")

			tc.check(t, buf.String())
		})
	}
}

func TestTestLogger(t *testing.T) {
	// Test the test logger utility
	tl := logging.NewTestLogger(t)

	tl.Logger.Info().Msg("message 1")
	tl.Logger.Error().Err(nil).Msg("message 2")

	// Test various assertions
	tl.AssertContains(t, "message 1")
	tl.AssertContains(t, "message 2")
	tl.AssertCount(t, 2)

	if !tl.ContainsAll("message 1", "message 2") {
		t.Error("Should contain both messages")
	}

	// Clear and verify
	tl.Clear()
	if tl.Count() != 0 {
		t.Error("Should have 0 entries after clear")
	}
}
