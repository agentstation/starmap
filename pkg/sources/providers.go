// Package sources provides public APIs for working with AI model data sources.
package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Default implementation functions that will be wired by init().
var (
	defaultGetClient     func(*catalogs.Provider) (catalogs.Client, error)
	defaultHasClient     func(catalogs.ProviderID) bool
	defaultListProviders func() []catalogs.ProviderID
	defaultFetchRaw      func(context.Context, *catalogs.Provider, string) ([]byte, error)
)

func init() {
	// Wire up the internal implementation to the public API
	// This bridges the internal packages to the public interface
	defaultGetClient = registry.Get
	defaultHasClient = registry.Has
	defaultListProviders = registry.List
	defaultFetchRaw = registry.FetchRaw
}

// ProviderFetcher provides operations for fetching models from provider APIs.
// This is the public API for external packages to interact with provider data.
type ProviderFetcher struct {
	// Private implementation functions that will be wired to internal packages
	getClientFunc     func(*catalogs.Provider) (catalogs.Client, error)
	hasClientFunc     func(catalogs.ProviderID) bool
	listProvidersFunc func() []catalogs.ProviderID
	fetchRawFunc      func(context.Context, *catalogs.Provider, string) ([]byte, error)
}

// providerOptions holds configuration for ProviderFetcher operations.
type providerOptions struct {
	loadCredentials bool          // Auto-load credentials from environment
	allowMissingKey bool          // Allow operations without API key
	timeout         time.Duration // Context timeout for operations
}

// ProviderOption configures ProviderFetcher behavior.
type ProviderOption func(*providerOptions)

// defaultProviderOptions returns options with sensible defaults.
func defaultProviderOptions() *providerOptions {
	return &providerOptions{
		loadCredentials: true,  // Default: auto-load credentials
		allowMissingKey: false, // Default: require API key
		timeout:         0,     // Default: no timeout
	}
}

// NewProviderFetcher creates a new provider fetcher for interacting with provider APIs.
// It provides a clean public interface for external packages.
func NewProviderFetcher(opts ...ProviderOption) *ProviderFetcher {
	options := defaultProviderOptions()
	for _, opt := range opts {
		opt(options)
	}

	return &ProviderFetcher{
		// These will be wired up by the implementation bridge
		getClientFunc:     defaultGetClient,
		hasClientFunc:     defaultHasClient,
		listProvidersFunc: defaultListProviders,
		fetchRawFunc:      defaultFetchRaw,
	}
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
	options := defaultProviderOptions()
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

		missingEnvVars := provider.MissingEnvVars()
		if len(missingEnvVars) > 0 {
			return nil, &errors.ConfigError{
				Component: string(provider.ID),
				Message:   fmt.Sprintf("missing required environment variables: %v", missingEnvVars),
			}
		}
	}

	// Get client from implementation
	if pf.getClientFunc == nil {
		return nil, &errors.ConfigError{
			Component: "provider_fetcher",
			Message:   "provider implementation not initialized",
		}
	}

	client, err := pf.getClientFunc(provider)
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

// HasClient checks if a provider has a client implementation available.
// This can be used to determine which providers are supported.
func (pf *ProviderFetcher) HasClient(id catalogs.ProviderID) bool {
	if pf.hasClientFunc == nil {
		return false
	}
	return pf.hasClientFunc(id)
}

// List returns all provider IDs that have client implementations.
// This is useful for discovering which providers can be used with FetchModels.
func (pf *ProviderFetcher) List() []catalogs.ProviderID {
	if pf.listProvidersFunc == nil {
		return []catalogs.ProviderID{}
	}
	return pf.listProvidersFunc()
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
	options := defaultProviderOptions()
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

		missingEnvVars := provider.MissingEnvVars()
		if len(missingEnvVars) > 0 {
			return nil, &errors.ConfigError{
				Component: string(provider.ID),
				Message:   fmt.Sprintf("missing required environment variables: %v", missingEnvVars),
			}
		}
	}

	// Call internal implementation
	if pf.fetchRawFunc == nil {
		return nil, &errors.ConfigError{
			Component: "provider_fetcher",
			Message:   "raw fetch implementation not initialized",
		}
	}

	return pf.fetchRawFunc(ctx, provider, endpoint)
}
