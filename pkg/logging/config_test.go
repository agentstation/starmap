package logging_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/agentstation/starmap/pkg/logging"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestConfigFunctions(t *testing.T) {
	// Save and restore the original logger and level
	originalLogger := *logging.Default()
	originalLevel := zerolog.GlobalLevel()
	defer func() {
		logging.SetDefault(originalLogger)
		zerolog.SetGlobalLevel(originalLevel)
	}()
	
	t.Run("DefaultConfig returns sensible defaults", func(t *testing.T) {
		cfg := logging.DefaultConfig()
		assert.NotNil(t, cfg)
		assert.Equal(t, "info", cfg.Level)
		assert.Equal(t, "auto", cfg.Format)
		assert.False(t, cfg.AddCaller)
		assert.Equal(t, "stderr", cfg.Output)
	})

	t.Run("NewLoggerFromConfig creates logger with config", func(t *testing.T) {
		// Create a temp file for output
		tmpfile, err := os.CreateTemp("", "test-log-*.txt")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())
		defer tmpfile.Close()
		
		cfg := &logging.Config{
			Level:     "debug",
			Format:    "json",
			Output:    tmpfile.Name(),
			AddCaller: true,
		}
		
		logger := logging.NewLoggerFromConfig(cfg)
		logger.Info().Msg("test message")
		
		// Read the output from the file
		content, err := os.ReadFile(tmpfile.Name())
		assert.NoError(t, err)
		output := string(content)
		assert.Contains(t, output, "test message")
		assert.Contains(t, output, "info")
	})

	t.Run("Configure sets global logger from config", func(t *testing.T) {
		// Create a temp file for output
		tmpfile, err := os.CreateTemp("", "test-log-*.txt")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())
		defer tmpfile.Close()
		
		cfg := &logging.Config{
			Level:     "warn",
			Format:    "json",
			Output:    tmpfile.Name(),
			AddCaller: false,
		}
		
		logging.Configure(cfg)
		
		// These should not appear (below warn level)
		logging.Debug().Msg("debug message")
		logging.Info().Msg("info message")
		
		// These should appear
		logging.Warn().Msg("warn message")
		logging.Error().Msg("error message")
		
		// Read the output from the file
		content, err := os.ReadFile(tmpfile.Name())
		assert.NoError(t, err)
		output := string(content)
		assert.NotContains(t, output, "debug message")
		assert.NotContains(t, output, "info message")
		assert.Contains(t, output, "warn message")
		assert.Contains(t, output, "error message")
	})

	t.Run("ConfigureFromEnv reads from environment", func(t *testing.T) {
		// Set environment variables
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("LOG_FORMAT", "console")
		defer os.Unsetenv("LOG_LEVEL")
		defer os.Unsetenv("LOG_FORMAT")
		
		// This should read from env and configure the logger
		logging.ConfigureFromEnv()
		
		// The global logger should now be at debug level
		// We can't easily test this without capturing output
		// but we can ensure it doesn't panic
	})

	t.Run("console format configuration", func(t *testing.T) {
		// Create a temp file for output
		tmpfile, err := os.CreateTemp("", "test-log-*.txt")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())
		defer tmpfile.Close()
		
		cfg := &logging.Config{
			Level:     "info",
			Format:    "console",
			Output:    tmpfile.Name(),
			AddCaller: false,
		}
		
		logger := logging.NewLoggerFromConfig(cfg)
		logger.Info().Str("key", "value").Msg("console test")
		
		// Read the output from the file
		content, err := os.ReadFile(tmpfile.Name())
		assert.NoError(t, err)
		output := string(content)
		assert.Contains(t, output, "console test")
		// Console format uses short level names
		assert.Contains(t, output, "INF")
	})

	t.Run("different log levels", func(t *testing.T) {
		testCases := []struct {
			level    string
			logFunc  func() *zerolog.Event
			shouldLog bool
		}{
			{"debug", logging.Debug, true},
			{"info", logging.Info, true},
			{"info", logging.Debug, false}, // debug below info
			{"warn", logging.Warn, true},
			{"warn", logging.Info, false}, // info below warn
			{"error", logging.Error, true},
			{"error", logging.Warn, false}, // warn below error
		}
		
		for _, tc := range testCases {
			t.Run(tc.level, func(t *testing.T) {
				// Create a temp file for output
				tmpfile, err := os.CreateTemp("", "test-log-*.txt")
				assert.NoError(t, err)
				defer os.Remove(tmpfile.Name())
				defer tmpfile.Close()
				
				cfg := &logging.Config{
					Level:     tc.level,
					Format:    "json",
					Output:    tmpfile.Name(),
				}
				
				logging.Configure(cfg)
				tc.logFunc().Msg("test")
				
				// Read the output from the file
				content, err := os.ReadFile(tmpfile.Name())
				assert.NoError(t, err)
				output := string(content)
				
				if tc.shouldLog {
					assert.Contains(t, output, "test")
				} else {
					assert.Empty(t, output)
				}
			})
		}
	})
}

func TestLoggerFunctions(t *testing.T) {
	// Save and restore the global level
	originalLevel := zerolog.GlobalLevel()
	defer zerolog.SetGlobalLevel(originalLevel)
	
	t.Run("Default returns global logger", func(t *testing.T) {
		logger := logging.Default()
		assert.NotNil(t, logger)
	})

	t.Run("SetDefault sets global logger", func(t *testing.T) {
		var buf bytes.Buffer
		newLogger := zerolog.New(&buf).Level(zerolog.InfoLevel)
		logging.SetDefault(newLogger)
		
		// Now the global functions should use this logger
		logging.Info().Msg("test with new default")
		assert.Contains(t, buf.String(), "test with new default")
	})

	t.Run("New creates JSON logger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.New(&buf)
		logger.Info().Msg("json test")
		
		output := buf.String()
		assert.Contains(t, output, "json test")
		assert.Contains(t, output, `"level":"info"`)
	})

	t.Run("NewConsole creates console logger", func(t *testing.T) {
		logger := logging.NewConsole()
		// Just ensure it doesn't panic
		logger.Info().Msg("console test")
	})

	t.Run("NewJSON creates JSON logger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewJSON(&buf)
		logger.Info().Msg("json test")
		
		output := buf.String()
		assert.Contains(t, output, "json test")
		assert.Contains(t, output, `"level":"info"`)
	})

	t.Run("Level creates logger with specific level", func(t *testing.T) {
		logger := logging.Level(zerolog.WarnLevel)
		
		// Can't easily test without capturing output
		// Just ensure it doesn't panic
		logger.Debug().Msg("should not appear")
		logger.Warn().Msg("should appear")
	})

	t.Run("logging event functions", func(t *testing.T) {
		var buf bytes.Buffer
		// Set both the logger and global level to ensure debug shows
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		logging.SetDefault(zerolog.New(&buf).Level(zerolog.DebugLevel))
		
		// Test each level
		logging.Debug().Msg("debug")
		logging.Info().Msg("info")
		logging.Warn().Msg("warn")
		logging.Error().Msg("error")
		
		output := buf.String()
		assert.Contains(t, output, "debug")
		assert.Contains(t, output, "info")
		assert.Contains(t, output, "warn")
		assert.Contains(t, output, "error")
	})

	t.Run("WithLevel creates event with dynamic level", func(t *testing.T) {
		var buf bytes.Buffer
		logging.SetDefault(zerolog.New(&buf).Level(zerolog.InfoLevel))
		
		logging.WithLevel(zerolog.InfoLevel).Msg("dynamic level")
		assert.Contains(t, buf.String(), "dynamic level")
	})

	t.Run("Err adds error to event", func(t *testing.T) {
		var buf bytes.Buffer
		logging.SetDefault(zerolog.New(&buf).Level(zerolog.ErrorLevel))
		
		err := assert.AnError
		logging.Err(err).Msg("error test")
		
		output := buf.String()
		assert.Contains(t, output, "error test")
		assert.Contains(t, output, err.Error())
	})

	t.Run("With creates context for fields", func(t *testing.T) {
		var buf bytes.Buffer
		baseLogger := zerolog.New(&buf).Level(zerolog.InfoLevel)
		logging.SetDefault(baseLogger)
		
		// Create a context with fields
		ctx := logging.With().
			Str("component", "test").
			Int("version", 1).
			Logger()
		
		ctx.Info().Msg("with context")
		
		output := buf.String()
		assert.Contains(t, output, "with context")
		assert.Contains(t, output, `"component":"test"`)
		assert.Contains(t, output, `"version":1`)
	})
}