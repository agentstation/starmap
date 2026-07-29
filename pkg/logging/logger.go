// Package logging provides structured logging for the starmap system using zerolog.
// It offers high-performance, zero-allocation logging with support for both
// human-readable console output during development and structured JSON output
// for production environments.
//
// Example usage:
//
//	// Get the default logger
//	log := logging.Default()
//	log.Info().Str("provider", "openai").Msg("Fetching models")
//
//	// Create a logger with context
//	ctx := logging.WithLogger(context.Background(), log)
//	ctxLog := logging.FromContext(ctx)
//	ctxLog.Debug().Msg("Using logger from context")
//
//	// Add structured fields
//	log.Error().
//	    Err(err).
//	    Str("provider_id", "anthropic").
//	    Int("retry_count", 3).
//	    Msg("Failed to fetch models")
package logging

import (
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// defaultLogger holds the process default as an immutable logger value.
var defaultLogger atomic.Pointer[zerolog.Logger]

func init() {
	SetDefault(createDefaultLogger())
}

// createDefaultLogger creates a logger with default settings.
func createDefaultLogger() zerolog.Logger {
	// Auto-detect if we're in a terminal for pretty output
	isTerminal := isatty()

	var writer io.Writer = os.Stderr

	if isTerminal && os.Getenv("LOG_FORMAT") != "json" {
		// Use console writer for human-readable output in terminals
		writer = zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.Kitchen,
			NoColor:    os.Getenv("NO_COLOR") != "",
		}
	}

	level := getLogLevel()

	// Create logger with context
	logger := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Logger()

	// Add caller information in debug mode
	if level <= zerolog.DebugLevel {
		logger = logger.With().Caller().Logger()
	}

	return logger
}

// Default returns a copy of the default logger.
func Default() zerolog.Logger {
	return *defaultLogger.Load()
}

// SetDefault atomically replaces the logger used by this package. It does not
// mutate zerolog's separate process-global logger or global level.
func SetDefault(logger zerolog.Logger) {
	defaultLogger.Store(&logger)
}

// Debug starts a new debug level log event.
func Debug() *zerolog.Event {
	logger := Default()
	return logger.Debug()
}

// Info starts a new info level log event.
func Info() *zerolog.Event {
	logger := Default()
	return logger.Info()
}

// Warn starts a new warning level log event.
func Warn() *zerolog.Event {
	logger := Default()
	return logger.Warn()
}

// Error starts a new error level log event.
func Error() *zerolog.Event {
	logger := Default()
	return logger.Error()
}

// isatty checks if stderr is a terminal.
func isatty() bool {
	// Check if stderr is a terminal
	if fileInfo, _ := os.Stderr.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		return true
	}
	return false
}

// getLogLevel returns the log level from environment or defaults.
func getLogLevel() zerolog.Level {
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		// Check for common verbose/debug flags
		if os.Getenv("DEBUG") != "" {
			return zerolog.DebugLevel
		}
		return zerolog.InfoLevel
	}

	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		return zerolog.InfoLevel
	}
	return level
}
