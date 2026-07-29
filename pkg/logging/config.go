package logging

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config holds logger configuration options.
type Config struct {
	// Level is the minimum log level to output
	Level string

	// Format is the output format (json, console, pretty)
	Format string

	// Output is where to write logs (stderr, stdout, discard, or none).
	Output string

	// TimeFormat for timestamps (kitchen, rfc3339, unix, etc.)
	TimeFormat string

	// NoColor disables color output in console mode
	NoColor bool

	// AddCaller includes file:line in log output
	AddCaller bool

	// Fields are default fields to include in all logs
	Fields map[string]any
}

func defaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "auto", // auto-detect based on terminal
		Output:     "stderr",
		TimeFormat: "kitchen",
		NoColor:    os.Getenv("NO_COLOR") != "",
		AddCaller:  false,
		Fields:     make(map[string]any),
	}
}

// New creates a logger from cfg without changing process-global zerolog state.
func New(cfg *Config) zerolog.Logger {
	if cfg == nil {
		cfg = defaultConfig()
	}

	// Parse log level
	level := parseLevel(cfg.Level)
	// Determine output writer
	writer := getWriter(cfg)

	// Create base logger
	logger := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Logger()

	// Add caller if requested or in debug mode
	if cfg.AddCaller || level <= zerolog.DebugLevel {
		logger = logger.With().Caller().Logger()
	}

	// Add default fields
	if len(cfg.Fields) > 0 {
		ctx := logger.With()
		for k, v := range cfg.Fields {
			ctx = addField(ctx, k, v)
		}
		logger = ctx.Logger()
	}

	return logger
}

// getWriter creates the appropriate writer based on configuration.
func getWriter(cfg *Config) io.Writer {
	// Determine output destination
	var output io.Writer
	switch strings.ToLower(cfg.Output) {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	case "discard", "none":
		output = io.Discard
	default:
		// A logger constructor that returns no closer cannot safely own a file.
		// Unknown output names therefore fall back to stderr.
		output = os.Stderr
	}

	// Determine format
	format := strings.ToLower(cfg.Format)
	if format == "auto" {
		// Auto-detect only real terminal files. Writers such as io.Discard are
		// intentionally not *os.File and must remain valid in auto mode.
		if file, ok := output.(*os.File); ok && isTerminalFile(file) {
			format = "console"
		} else {
			format = "json"
		}
	}

	// Create appropriate writer
	switch format {
	case "console", "pretty":
		return zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: parseTimeFormat(cfg.TimeFormat),
			NoColor:    cfg.NoColor,
		}
	default:
		// JSON format
		return output
	}
}

func isTerminalFile(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// parseLevel parses a log level string.
func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "disabled", "none", "off":
		return zerolog.Disabled
	default:
		// Try parsing with zerolog's built-in parser
		if l, err := zerolog.ParseLevel(level); err == nil {
			return l
		}
		return zerolog.InfoLevel
	}
}

// parseTimeFormat parses time format configuration.
func parseTimeFormat(format string) string {
	switch strings.ToLower(format) {
	case "kitchen":
		return time.Kitchen
	case "rfc3339":
		return time.RFC3339
	case "rfc3339nano":
		return time.RFC3339Nano
	case "unix", "epoch":
		return "" // Empty string means Unix timestamp
	case "stamp":
		return time.Stamp
	case "stampMilli":
		return time.StampMilli
	case "stampMicro":
		return time.StampMicro
	case "stampNano":
		return time.StampNano
	default:
		// Use as-is if it looks like a custom format
		if strings.Contains(format, "2006") || strings.Contains(format, "15:04") {
			return format
		}
		return time.Kitchen
	}
}

// addField adds a field to the context based on its type.
func addField(ctx zerolog.Context, key string, value any) zerolog.Context {
	switch v := value.(type) {
	case string:
		return ctx.Str(key, v)
	case int:
		return ctx.Int(key, v)
	case int64:
		return ctx.Int64(key, v)
	case float64:
		return ctx.Float64(key, v)
	case bool:
		return ctx.Bool(key, v)
	case time.Time:
		return ctx.Time(key, v)
	case error:
		return ctx.Err(v)
	default:
		return ctx.Interface(key, v)
	}
}
