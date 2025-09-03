// Package registry provides provider client registry functions.
// This package is separate from the providers source to avoid circular dependencies.
package registry

import (
	"context"
	"fmt"
	"io"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"

	// Import provider implementations for registry
	"github.com/agentstation/starmap/internal/sources/providers/anthropic"
	"github.com/agentstation/starmap/internal/sources/providers/cerebras"
	"github.com/agentstation/starmap/internal/sources/providers/deepseek"
	googleaistudio "github.com/agentstation/starmap/internal/sources/providers/google-ai-studio"
	googlevertex "github.com/agentstation/starmap/internal/sources/providers/google-vertex"
	"github.com/agentstation/starmap/internal/sources/providers/groq"
	"github.com/agentstation/starmap/internal/sources/providers/openai"
)

// registry maps provider IDs to their client creation functions
var registry = map[catalogs.ProviderID]func(*catalogs.Provider) catalogs.Client{
	catalogs.ProviderIDOpenAI:         func(p *catalogs.Provider) catalogs.Client { return openai.NewClient(p) },
	catalogs.ProviderIDAnthropic:      func(p *catalogs.Provider) catalogs.Client { return anthropic.NewClient(p) },
	catalogs.ProviderIDGroq:           func(p *catalogs.Provider) catalogs.Client { return groq.NewClient(p) },
	catalogs.ProviderIDCerebras:       func(p *catalogs.Provider) catalogs.Client { return cerebras.NewClient(p) },
	catalogs.ProviderIDDeepSeek:       func(p *catalogs.Provider) catalogs.Client { return deepseek.NewClient(p) },
	catalogs.ProviderIDGoogleAIStudio: func(p *catalogs.Provider) catalogs.Client { return googleaistudio.NewClient(p) },
	catalogs.ProviderIDGoogleVertex:   func(p *catalogs.Provider) catalogs.Client { return googlevertex.NewClient(p) },
}

// Get creates a NEW client instance for the given provider.
// Each call returns a fresh client with its own HTTP client to avoid race conditions.
func Get(provider *catalogs.Provider) (catalogs.Client, error) {
	newClient, ok := registry[provider.ID]
	if !ok {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Value:   provider.ID,
			Message: fmt.Sprintf("unsupported provider: %s", provider.ID),
		}
	}
	return newClient(provider), nil
}

// Has checks if a provider ID has a client implementation.
func Has(id catalogs.ProviderID) bool {
	_, ok := registry[id]
	return ok
}

// List returns all provider IDs that have client implementations.
func List() []catalogs.ProviderID {
	ids := make([]catalogs.ProviderID, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	return ids
}

// FetchRaw fetches raw response data from a provider's API endpoint.
// This function is used for fetching raw API responses for testdata generation.
func FetchRaw(ctx context.Context, provider *catalogs.Provider, endpoint string) ([]byte, error) {
	// Create transport client configured for this provider
	transportClient := transport.NewForProvider(provider)

	// Make the raw request
	resp, err := transportClient.Get(ctx, endpoint, provider)
	if err != nil {
		return nil, &errors.APIError{
			Provider: string(provider.ID),
			Endpoint: endpoint,
			Message:  "API request failed",
			Err:      err,
		}
	}
	defer func() {
		// Drain any remaining body to allow connection reuse
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	// Read raw response body
	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WrapIO("read", "response body", err)
	}

	return rawData, nil
}
