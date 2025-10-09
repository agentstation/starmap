// Package sources provides public APIs for working with AI model data sources.
package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstation/starmap/internal/sources/clients"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ProviderFetcher provides operations for fetching models from provider APIs.
// This is the public API for external packages to interact with provider data.
type ProviderFetcher struct {
	providers *catalogs.Providers
	options   *providerOptions
}

// providerOptions holds configuration for ProviderFetcher operations.
type providerOptions struct {
	loadCredentials bool          // Auto-load credentials from environment
	allowMissingKey bool          // Allow operations without API key
	timeout         time.Duration // Context timeout for operations
}

func (po *providerOptions) apply(opts ...ProviderOption) *providerOptions {
	for _, opt := range opts {
		opt(po)
	}
	return po
}

// ProviderOption configures ProviderFetcher behavior.
type ProviderOption func(*providerOptions)

// providerDefaults returns options with sensible defaults.
func providerDefaults() *providerOptions {
	return &providerOptions{
		loadCredentials: true,  // Default: auto-load credentials
		allowMissingKey: false, // Default: require API key
		timeout:         0,     // Default: no timeout
	}
}

// NewProviderFetcher creates a new provider fetcher for interacting with provider APIs.
// It provides a clean public interface for external packages.
// The providers parameter should contain the catalog providers to create clients for.
func NewProviderFetcher(providers *catalogs.Providers, opts ...ProviderOption) *ProviderFetcher {
	options := providerDefaults().apply(opts...)

	return &ProviderFetcher{
		providers: providers,
		options:   options,
	}
}

// Providers returns the providers that can be used by the provider fetcher.
func (pf *ProviderFetcher) Providers() *catalogs.Providers {
	result := catalogs.NewProviders()
	for _, provider := range pf.providers.List() {
		if provider.IsAPIKeyRequired() && !provider.HasAPIKey() {
			continue
		}
		if !provider.HasRequiredEnvVars() {
			continue
		}
		_ = result.Add(&provider) // Ignore error - provider is valid
	}
	return result
}

// List returns all provider IDs that have client implementations.
func (pf *ProviderFetcher) List() []catalogs.ProviderID {
	var providerIDs []catalogs.ProviderID
	for _, provider := range pf.providers.List() {
		if pf.HasClient(provider.ID) {
			providerIDs = append(providerIDs, provider.ID)
		}
	}
	return providerIDs
}

// HasClient checks if a provider ID has a client implementation.
func (pf *ProviderFetcher) HasClient(id catalogs.ProviderID) bool {
	// Check if we have a provider configuration
	provider, found := pf.providers.Get(id)
	if !found {
		return false
	}

	// Try to create a client for this provider
	_, err := clients.NewProvider(provider)
	return err == nil
}

// WithoutCredentialLoading disables automatic credential loading from environment.
// Use this when credentials are already loaded or when testing.
func WithoutCredentialLoading() ProviderOption {
	return func(o *providerOptions) {
		o.loadCredentials = false
	}
}

// WithAllowMissingAPIKey allows operations even when API key is not configured.
// Useful for checking provider support without credentials.
func WithAllowMissingAPIKey() ProviderOption {
	return func(o *providerOptions) {
		o.allowMissingKey = true
	}
}

// WithTimeout sets a timeout for provider operations.
// The timeout applies to the context passed to FetchModels.
func WithTimeout(d time.Duration) ProviderOption {
	return func(o *providerOptions) {
		o.timeout = d
	}
}

// FetchModels fetches available models from a single provider's API.
// It handles credential loading, client creation, and API communication.
//
// Example:
//
//	fetcher := NewProviderFetcher()
//	models, err := fetcher.FetchModels(ctx, provider)
//
// With options:
//
//	fetcher := NewProviderFetcher(WithTimeout(30 * time.Second))
//	models, err := fetcher.FetchModels(ctx, provider, WithAllowMissingAPIKey())
func (pf *ProviderFetcher) FetchModels(ctx context.Context, provider *catalogs.Provider, opts ...ProviderOption) ([]catalogs.Model, error) {
	if provider == nil {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Message: "cannot be nil",
		}
	}

	// Apply options
	options := providerDefaults()
	for _, opt := range opts {
		opt(options)
	}

	// Apply timeout if specified
	if options.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
		defer cancel()
	}

	// Load credentials if requested
	if options.loadCredentials {
		provider.LoadAPIKey()
		provider.LoadEnvVars()
	}

	// Check credentials unless explicitly allowed to be missing
	if !options.allowMissingKey {
		if provider.IsAPIKeyRequired() && !provider.HasAPIKey() {
			return nil, &errors.AuthenticationError{
				Provider: string(provider.ID),
				Method:   "api_key",
				Message:  fmt.Sprintf("provider %s requires API key %s but it is not configured", provider.ID, provider.APIKey.Name),
			}
		}

		missingEnvVars := provider.MissingRequiredEnvVars()
		if len(missingEnvVars) > 0 {
			return nil, &errors.ConfigError{
				Component: string(provider.ID),
				Message:   fmt.Sprintf("missing required environment variables: %v", missingEnvVars),
			}
		}
	}

	// Get client from providers
	client, err := clients.NewProvider(provider)
	if err != nil {
		return nil, errors.WrapResource("get", "client", string(provider.ID), err)
	}

	// Fetch models from API
	models, err := client.ListModels(ctx)
	if err != nil {
		return nil, &errors.SyncError{
			Provider: string(provider.ID),
			Err:      err,
		}
	}

	return models, nil
}

// FetchRawResponse fetches the raw API response from a provider's endpoint.
// This is useful for testing, debugging, or saving raw responses as testdata.
//
// The endpoint parameter should be the full URL to the API endpoint.
// The response is returned as raw bytes (JSON) without any parsing.
func (pf *ProviderFetcher) FetchRawResponse(ctx context.Context, provider *catalogs.Provider, endpoint string, opts ...ProviderOption) ([]byte, error) {
	if provider == nil {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Message: "cannot be nil",
		}
	}

	// Apply options
	options := providerDefaults()
	for _, opt := range opts {
		opt(options)
	}

	// Apply timeout if specified
	if options.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
		defer cancel()
	}

	// Load credentials if requested
	if options.loadCredentials {
		provider.LoadAPIKey()
		provider.LoadEnvVars()
	}

	// Check credentials unless explicitly allowed to be missing
	if !options.allowMissingKey {
		if provider.IsAPIKeyRequired() && !provider.HasAPIKey() {
			return nil, &errors.AuthenticationError{
				Provider: string(provider.ID),
				Method:   "api_key",
				Message:  fmt.Sprintf("provider %s requires API key %s but it is not configured", provider.ID, provider.APIKey.Name),
			}
		}

		missingEnvVars := provider.MissingRequiredEnvVars()
		if len(missingEnvVars) > 0 {
			return nil, &errors.ConfigError{
				Component: string(provider.ID),
				Message:   fmt.Sprintf("missing required environment variables: %v", missingEnvVars),
			}
		}
	}

	// Call providers' FetchRaw function
	return clients.FetchRaw(ctx, provider, endpoint)
}
