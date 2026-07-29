package app

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/logging"
)

// NewLogger creates a configured logger based on the application configuration.
// Log level precedence (highest to lowest):
//  1. --log-level flag (explicit always wins)
//  2. -v/--verbose flag (shortcut for debug)
//  3. -q/--quiet flag (shortcut for warn)
//  4. LOG_LEVEL environment variable
//  5. Default (info)
func NewLogger(config *Config) zerolog.Logger {
	// Determine log level using precedence rules
	level := determineLogLevel(config)

	// Build logging configuration
	logConfig := &logging.Config{
		Level:     level,
		Format:    config.LogFormat,
		Output:    config.LogOutput,
		AddCaller: level == "debug" || level == "trace",
	}

	// Create logger from config
	return logging.NewLoggerFromConfig(logConfig)
}

// determineLogLevel determines the log level using clear precedence rules.
func determineLogLevel(config *Config) string {
	// 1. Explicit --log-level always wins
	if config.LogLevel != "" {
		validated := validateLogLevel(config.LogLevel)
		if validated != config.LogLevel {
			// Validation changed the level (invalid input)
			fmt.Fprintf(os.Stderr, "Warning: invalid log level %q, using %q\n", config.LogLevel, validated)
		}
		return validated
	}

	// 2. Check for conflicting boolean flags
	if config.Verbose && config.Quiet {
		// Both specified - warn user and use quiet (more restrictive)
		fmt.Fprintf(os.Stderr, "Warning: both --verbose and --quiet specified, using --quiet\n")
		return "warn"
	}

	// 3. Boolean shortcuts
	if config.Verbose {
		return "debug"
	}
	if config.Quiet {
		return "warn"
	}

	// 4. Environment variable (already loaded in config)
	// This is handled by LoadConfig reading LOG_LEVEL env var

	// 5. Default
	return "info"
}

// validateLogLevel validates a log level string and returns a valid level.
// If the input is invalid, returns "info" as a safe default.
func validateLogLevel(level string) string {
	validLevels := map[string]bool{
		"trace": true,
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if validLevels[level] {
		return level
	}

	return "info"
}
