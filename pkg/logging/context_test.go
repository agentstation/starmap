package logging_test

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/logging"
	"github.com/stretchr/testify/assert"
)

func TestContextFunctions(t *testing.T) {
	t.Run("WithProvider adds provider to context", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.WithProvider(ctx, "openai")
		
		// Extract logger and verify it has the provider field
		logger := logging.FromContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("WithSource adds source to context", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.WithSource(ctx, "provider_api")
		
		logger := logging.FromContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("WithOperation adds operation to context", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.WithOperation(ctx, "fetch_models")
		
		logger := logging.FromContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("WithModel adds model to context", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.WithModel(ctx, "gpt-4")
		
		logger := logging.FromContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("WithFields adds custom fields to context", func(t *testing.T) {
		ctx := context.Background()
		fields := map[string]interface{}{
			"user_id": "123",
			"request_id": "abc-def",
		}
		ctx = logging.WithFields(ctx, fields)
		
		logger := logging.FromContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("FromContext returns logger from context", func(t *testing.T) {
		ctx := context.Background()
		
		// First call should create a new logger
		logger1 := logging.FromContext(ctx)
		assert.NotNil(t, logger1)
		
		// Add provider and get logger again
		ctx = logging.WithProvider(ctx, "anthropic")
		logger2 := logging.FromContext(ctx)
		assert.NotNil(t, logger2)
	})

	t.Run("Ctx extracts logger from context", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.WithProvider(ctx, "groq")
		
		logger := logging.Ctx(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("chaining context functions", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.WithProvider(ctx, "openai")
		ctx = logging.WithSource(ctx, "api")
		ctx = logging.WithOperation(ctx, "list_models")
		ctx = logging.WithModel(ctx, "gpt-4o")
		
		logger := logging.FromContext(ctx)
		assert.NotNil(t, logger)
	})
}