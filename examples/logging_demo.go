// Example demonstrating the zerolog integration in starmap
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/logging"
)

func main() {
	// Configure logging - respects environment variables
	format := os.Getenv("LOG_FORMAT")
	if format == "" {
		format = "console" // Default to pretty console output
	}
	
	logging.Configure(&logging.Config{
		Level:      "debug",
		Format:     format,
		Output:     "stderr",
		TimeFormat: "kitchen",
		AddCaller:  true,
	})

	// Get the default logger
	logger := logging.Default()
	logger.Info().
		Str("app", "starmap").
		Str("version", "1.0.0").
		Msg("Starting starmap example")

	// Create a context with logging
	ctx := context.Background()
	ctx = logging.WithRequestID(ctx, "demo-001")
	ctx = logging.WithOperation(ctx, "catalog_sync")
	
	// Create starmap instance
	sm, err := starmap.New(
		starmap.WithAutoUpdates(false), // Disable auto-updates for demo
	)
	if err != nil {
		logging.Fatal().Err(err).Msg("Failed to create starmap instance")
	}

	// Log with context
	ctxLogger := logging.FromContext(ctx)
	ctxLogger.Debug().Msg("Fetching catalog")

	// Get catalog
	catalog, err := sm.Catalog()
	if err != nil {
		logging.Error().Err(err).Msg("Failed to get catalog")
		os.Exit(1)
	}

	// Log catalog statistics with structured fields
	providers := catalog.Providers()
	models := catalog.Models()
	
	logging.Info().
		Int("provider_count", len(providers.List())).
		Int("model_count", len(models.List())).
		Dur("duration", time.Millisecond*250).
		Msg("Catalog loaded successfully")

	// Demonstrate provider-specific logging
	for _, provider := range providers.List() {
		providerLogger := logging.WithProvider(ctx, string(provider.ID))
		
		// Check provider configuration
		if provider.IsAPIKeyRequired() && !provider.HasAPIKey() {
			logging.Ctx(providerLogger).Warn().
				Str("provider_id", string(provider.ID)).
				Str("required_key", provider.APIKey.Name).
				Msg("Provider requires API key but none configured")
		} else {
			logging.Ctx(providerLogger).Debug().
				Str("provider_id", string(provider.ID)).
				Bool("has_api_key", provider.HasAPIKey()).
				Msg("Provider configuration checked")
		}
	}

	// Demonstrate error logging with context
	testError := fmt.Errorf("model with ID %s not found", "test-model-123")
	logging.Error().
		Err(testError).
		Str("operation", "model_lookup").
		Msg("Model not found")

	// Log completion
	logging.Info().
		Str("request_id", "demo-001").
		Msg("Example completed successfully")
}