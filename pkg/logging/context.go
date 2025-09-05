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
	// requestIDKey is the context key for request ID.
	requestIDKey
)

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, logger *zerolog.Logger) context.Context {
	if logger == nil {
		logger = Default()
	}
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext extracts the logger from context, or returns the default logger.
func FromContext(ctx context.Context) *zerolog.Logger {
	if ctx == nil {
		return Default()
	}

	if logger, ok := ctx.Value(loggerKey).(*zerolog.Logger); ok && logger != nil {
		return logger
	}

	return Default()
}

// Ctx returns a logger from the context or the default logger
// This is a shorter alias for FromContext.
func Ctx(ctx context.Context) *zerolog.Logger {
	return FromContext(ctx)
}

// WithRequestID adds a request ID to the context for tracing.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	ctx = context.WithValue(ctx, requestIDKey, requestID)

	// Also update the logger with the request ID
	logger := FromContext(ctx)
	newLogger := logger.With().Str("request_id", requestID).Logger()
	return WithLogger(ctx, &newLogger)
}

// RequestID extracts the request ID from context.
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithFields adds structured fields to the logger in the context.
func WithFields(ctx context.Context, fields map[string]any) context.Context {
	logger := FromContext(ctx)
	logCtx := logger.With()

	for key, value := range fields {
		logCtx = addFieldToContext(logCtx, key, value)
	}

	newLogger := logCtx.Logger()
	return WithLogger(ctx, &newLogger)
}

// WithField adds a single field to the logger in the context.
func WithField(ctx context.Context, key string, value any) context.Context {
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
	return WithField(ctx, "provider_id", providerID)
}

// WithModel adds model context to the logger.
func WithModel(ctx context.Context, modelID string) context.Context {
	return WithField(ctx, "model_id", modelID)
}

// WithSource adds source context to the logger.
func WithSource(ctx context.Context, source string) context.Context {
	return WithField(ctx, "source", source)
}

// WithOperation adds operation context to the logger.
func WithOperation(ctx context.Context, operation string) context.Context {
	return WithField(ctx, "operation", operation)
}

// WithError adds an error to the context logger.
func WithError(ctx context.Context, err error) context.Context {
	if err == nil {
		return ctx
	}
	return WithField(ctx, "error", err)
}
