package logging

import (
	"context"

	"github.com/rs/zerolog"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey int

const (
	// loggerKey is the context key for the logger.
	loggerKey contextKey = iota
	// runIDKey is the context key for one operation or synchronization run.
	runIDKey
)

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, logger *zerolog.Logger) context.Context {
	if logger == nil {
		defaultLogger := Default()
		logger = &defaultLogger
	}
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext extracts the logger from context, or returns the default logger.
func FromContext(ctx context.Context) *zerolog.Logger {
	if ctx == nil {
		logger := Default()
		return &logger
	}

	if logger, ok := ctx.Value(loggerKey).(*zerolog.Logger); ok && logger != nil {
		return logger
	}

	logger := Default()
	return &logger
}

// Ctx returns a logger from the context or the default logger
// This is a shorter alias for FromContext.
func Ctx(ctx context.Context) *zerolog.Logger {
	return FromContext(ctx)
}

// WithRunID adds a run correlation ID to the context logger.
func WithRunID(ctx context.Context, runID string) context.Context {
	ctx = context.WithValue(ctx, runIDKey, runID)
	return withField(ctx, "run_id", runID)
}

// RunID extracts the run correlation ID from context.
func RunID(ctx context.Context) string {
	if ctx != nil {
		if id, ok := ctx.Value(runIDKey).(string); ok {
			return id
		}
	}
	return ""
}

func withField(ctx context.Context, key string, value any) context.Context {
	logger := FromContext(ctx)
	logCtx := logger.With()
	logCtx = addFieldToContext(logCtx, key, value)
	newLogger := logCtx.Logger()
	return WithLogger(ctx, &newLogger)
}

// addFieldToContext adds a field to the logger context based on its type.
func addFieldToContext(ctx zerolog.Context, key string, value any) zerolog.Context {
	switch v := value.(type) {
	case string:
		return ctx.Str(key, v)
	case int:
		return ctx.Int(key, v)
	case int64:
		return ctx.Int64(key, v)
	case uint:
		return ctx.Uint(key, v)
	case uint64:
		return ctx.Uint64(key, v)
	case float32:
		return ctx.Float32(key, v)
	case float64:
		return ctx.Float64(key, v)
	case bool:
		return ctx.Bool(key, v)
	case error:
		if key == "error" || key == "err" {
			return ctx.Err(v)
		}
		return ctx.Str(key, v.Error())
	default:
		return ctx.Interface(key, v)
	}
}

// WithProvider adds provider context to the logger.
func WithProvider(ctx context.Context, providerID string) context.Context {
	return withField(ctx, "provider_id", providerID)
}

// WithSource adds source context to the logger.
func WithSource(ctx context.Context, source string) context.Context {
	return withField(ctx, "source", source)
}
