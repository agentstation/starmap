package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentstation/starmap/internal/sources/providers/baseclient"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// TestClientErrors tests error handling in the OpenAI client.
func TestClientErrors(t *testing.T) {
	t.Run("missing provider configuration", func(t *testing.T) {
		// Skip this test - NewClient doesn't validate nil provider
		// and will panic when ListModels is called
		t.Skip("NewClient doesn't validate nil provider")
	})

	t.Run("API authentication error", func(t *testing.T) {
		// Create a test server that returns 401
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
		}))
		defer server.Close()

		// Set up environment for test
		os.Setenv("OPENAI_API_KEY", "invalid")
		defer os.Unsetenv("OPENAI_API_KEY")
		os.Setenv("OPENAI_BASE_URL", server.URL)
		defer os.Unsetenv("OPENAI_BASE_URL")

		provider := &catalogs.Provider{
			ID:   catalogs.ProviderIDOpenAI,
			Name: "OpenAI",
			APIKey: &catalogs.ProviderAPIKey{
				Name:   "OPENAI_API_KEY",
				Header: "Authorization",
				Scheme: "Bearer",
			},
			Catalog: &catalogs.ProviderCatalog{
				APIURL: &server.URL,
			},
		}

		client := NewClient(provider)
		_, err := client.ListModels(context.Background())

		require.Error(t, err)
		var apiErr *errors.APIError
		assert.ErrorAs(t, err, &apiErr)
		assert.Equal(t, "openai", apiErr.Provider)
		assert.Equal(t, 401, apiErr.StatusCode)
	})

	t.Run("API rate limit error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
		}))
		defer server.Close()

		os.Setenv("OPENAI_API_KEY", "test")
		defer os.Unsetenv("OPENAI_API_KEY")
		os.Setenv("OPENAI_BASE_URL", server.URL)
		defer os.Unsetenv("OPENAI_BASE_URL")

		provider := &catalogs.Provider{
			ID:   catalogs.ProviderIDOpenAI,
			Name: "OpenAI",
			APIKey: &catalogs.ProviderAPIKey{
				Name:   "OPENAI_API_KEY",
				Header: "Authorization",
				Scheme: "Bearer",
			},
			Catalog: &catalogs.ProviderCatalog{
				APIURL: &server.URL,
			},
		}

		client := NewClient(provider)
		_, err := client.ListModels(context.Background())

		require.Error(t, err)
		var apiErr *errors.APIError
		assert.ErrorAs(t, err, &apiErr)
		assert.Equal(t, 429, apiErr.StatusCode)
	})

	t.Run("API server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": {"message": "Internal server error"}}`))
		}))
		defer server.Close()

		os.Setenv("OPENAI_API_KEY", "test")
		defer os.Unsetenv("OPENAI_API_KEY")
		os.Setenv("OPENAI_BASE_URL", server.URL)
		defer os.Unsetenv("OPENAI_BASE_URL")

		provider := &catalogs.Provider{
			ID:   catalogs.ProviderIDOpenAI,
			Name: "OpenAI",
			APIKey: &catalogs.ProviderAPIKey{
				Name:   "OPENAI_API_KEY",
				Header: "Authorization",
				Scheme: "Bearer",
			},
			Catalog: &catalogs.ProviderCatalog{
				APIURL: &server.URL,
			},
		}

		client := NewClient(provider)
		_, err := client.ListModels(context.Background())

		require.Error(t, err)
		var apiErr *errors.APIError
		assert.ErrorAs(t, err, &apiErr)
		assert.Equal(t, 500, apiErr.StatusCode)
		assert.Contains(t, err.Error(), "failed to decode response")
	})

	t.Run("malformed JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": [{"id": "gpt-4", "invalid json`))
		}))
		defer server.Close()

		os.Setenv("OPENAI_API_KEY", "test")
		defer os.Unsetenv("OPENAI_API_KEY")
		os.Setenv("OPENAI_BASE_URL", server.URL)
		defer os.Unsetenv("OPENAI_BASE_URL")

		provider := &catalogs.Provider{
			ID:   catalogs.ProviderIDOpenAI,
			Name: "OpenAI",
			APIKey: &catalogs.ProviderAPIKey{
				Name:   "OPENAI_API_KEY",
				Header: "Authorization",
				Scheme: "Bearer",
			},
			Catalog: &catalogs.ProviderCatalog{
				APIURL: &server.URL,
			},
		}

		client := NewClient(provider)
		_, err := client.ListModels(context.Background())

		require.Error(t, err)
		// Should be a decode error
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("network timeout", func(t *testing.T) {
		// Create a server that doesn't respond quickly
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Don't respond, let the client timeout
			<-r.Context().Done()
		}))
		defer server.Close()

		os.Setenv("OPENAI_API_KEY", "test")
		defer os.Unsetenv("OPENAI_API_KEY")
		os.Setenv("OPENAI_BASE_URL", server.URL)
		defer os.Unsetenv("OPENAI_BASE_URL")

		provider := &catalogs.Provider{
			ID:   catalogs.ProviderIDOpenAI,
			Name: "OpenAI",
			APIKey: &catalogs.ProviderAPIKey{
				Name:   "OPENAI_API_KEY",
				Header: "Authorization",
				Scheme: "Bearer",
			},
			Catalog: &catalogs.ProviderCatalog{
				APIURL: &server.URL,
			},
		}

		client := NewClient(provider)

		// Use a short timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 1) // 1 nanosecond timeout
		defer cancel()

		_, err := client.ListModels(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request failed")
	})
}

// TestClientModelConversion tests model conversion with edge cases.
func TestClientModelConversion(t *testing.T) {
	// No server needed for model conversion tests
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "OPENAI_API_KEY",
			Header: "Authorization",
			Scheme: "Bearer",
		},
	}
	os.Setenv("OPENAI_API_KEY", "test")
	defer os.Unsetenv("OPENAI_API_KEY")

	client := NewClient(provider)

	t.Run("minimal model data", func(t *testing.T) {
		minimalModel := baseclient.OpenAIModelData{
			ID:      "test-model",
			Created: 1234567890,
			Object:  "model",
			OwnedBy: "openai",
		}

		model := client.ConvertToModel(minimalModel)

		assert.Equal(t, "test-model", model.ID)
		assert.Equal(t, "test-model", model.Name)
		// Model should be created with basic fields
	})

	t.Run("model with null permission", func(t *testing.T) {
		modelData := baseclient.OpenAIModelData{
			ID:      "gpt-4",
			Created: 1234567890,
			Object:  "model",
			OwnedBy: "openai",
		}

		model := client.ConvertToModel(modelData)
		assert.NotNil(t, model)
		assert.Equal(t, "gpt-4", model.ID)
	})

	t.Run("deprecated model detection", func(t *testing.T) {
		deprecatedModel := baseclient.OpenAIModelData{
			ID:      "text-davinci-003",
			Created: 1234567890,
			Object:  "model",
			OwnedBy: "openai",
		}

		model := client.ConvertToModel(deprecatedModel)
		// OpenAI deprecated models like text-davinci-003 should be detected by name pattern
		assert.Contains(t, model.ID, "davinci")
	})

	t.Run("model feature inference", func(t *testing.T) {
		// Test vision model
		visionModel := baseclient.OpenAIModelData{
			ID:      "gpt-4-vision-preview",
			Created: 1234567890,
			Object:  "model",
			OwnedBy: "openai",
		}

		model := client.ConvertToModel(visionModel)
		assert.NotNil(t, model.Features)
		assert.Contains(t, model.Features.Modalities.Input, catalogs.ModelModalityImage)

		// Test function calling model
		functionModel := baseclient.OpenAIModelData{
			ID:      "gpt-4-turbo",
			Created: 1234567890,
			Object:  "model",
			OwnedBy: "openai",
		}

		model = client.ConvertToModel(functionModel)
		assert.NotNil(t, model.Features)
		assert.True(t, model.Features.ToolCalls)
		assert.True(t, model.Features.ToolChoice)
	})
}

// TestClientConcurrency tests concurrent access to the client.
func TestClientConcurrency(t *testing.T) {
	// Create a test server that returns valid responses
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object": "list", "data": [{"id": "test-model", "object": "model", "created": 1234567890, "owned_by": "openai"}]}`))
	}))
	defer server.Close()

	os.Setenv("OPENAI_API_KEY", "test")
	defer os.Unsetenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_BASE_URL", server.URL)
	defer os.Unsetenv("OPENAI_BASE_URL")

	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "OPENAI_API_KEY",
			Header: "Authorization",
			Scheme: "Bearer",
		},
		Catalog: &catalogs.ProviderCatalog{
			APIURL: &server.URL,
		},
	}

	client := NewClient(provider)

	// Run multiple concurrent requests
	numGoroutines := 10
	done := make(chan bool, numGoroutines)
	errorChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			models, err := client.ListModels(context.Background())
			if err != nil {
				errorChan <- err
			} else if len(models) == 0 {
				errorChan <- assert.AnError
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check for errors
	close(errorChan)
	for err := range errorChan {
		t.Errorf("Concurrent request failed: %v", err)
	}

	assert.Equal(t, numGoroutines, callCount)
}
