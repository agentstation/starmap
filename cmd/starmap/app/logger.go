package app

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/logging"
)

// NewLogger creates a configured logger based on the application configuration.
func NewLogger(config *Config) zerolog.Logger {
	// Determine log level from config or flags
	level := config.LogLevel
	if config.Verbose {
		level = "debug"
	}
	if config.Quiet {
		level = "warn"
	}

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
