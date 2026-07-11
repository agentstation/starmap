// Package sources provides public APIs for working with AI model data sources.
package sources

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/providers/clients"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ProviderClient fetches model information from a provider API.
type ProviderClient interface {
	ListModels(ctx context.Context) ([]catalogs.Model, error)
	IsAPIKeyRequired() bool
	HasAPIKey() bool
}

// ProviderClientFactory creates provider API clients.
type ProviderClientFactory func(*catalogs.Provider) (ProviderClient, error)

// RawFetchResult contains the result of a raw provider fetch operation.
type RawFetchResult struct {
	Data       []byte
	Response   *http.Response
	Latency    time.Duration
	RequestURL string
}

// ProviderRawFetcher fetches a raw provider API response.
type ProviderRawFetcher func(context.Context, *catalogs.Provider, string) (*RawFetchResult, error)

var providerRegistry struct {
	mu            sync.RWMutex
	clientFactory ProviderClientFactory
	rawFetcher    ProviderRawFetcher
}

// ProviderFetcher provides operations for fetching models from provider APIs.
// This is the public API for external packages to interact with provider data.
type ProviderFetcher struct {
	providers catalogs.ProvidersReader
	options   *providerOptions
}

// providerOptions holds configuration for ProviderFetcher operations.
type providerOptions struct {
	loadCredentials bool          // Auto-load credentials from environment
	allowMissingKey bool          // Allow operations without API key
	timeout         time.Duration // Context timeout for operations
	clientFactory   ProviderClientFactory
	rawFetcher      ProviderRawFetcher
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
	clientFactory, rawFetcher := registeredProviderHooks()
	if clientFactory == nil {
		clientFactory = defaultProviderClientFactory
	}
	if rawFetcher == nil {
		rawFetcher = defaultProviderRawFetcher
	}
	return &providerOptions{
		loadCredentials: true,  // Default: auto-load credentials
		allowMissingKey: false, // Default: require API key
		timeout:         0,     // Default: no timeout
		clientFactory:   clientFactory,
		rawFetcher:      rawFetcher,
	}
}

func defaultProviderClientFactory(provider *catalogs.Provider) (ProviderClient, error) {
	return clients.NewProvider(provider)
}

func defaultProviderRawFetcher(ctx context.Context, provider *catalogs.Provider, endpoint string) (*RawFetchResult, error) {
	result, err := clients.FetchRaw(ctx, provider, endpoint)
	if err != nil {
		return nil, err
	}
	return &RawFetchResult{
		Data:       result.Data,
		Response:   result.Response,
		Latency:    result.Latency,
		RequestURL: result.RequestURL,
	}, nil
}

func (po *providerOptions) clone() *providerOptions {
	if po == nil {
		return providerDefaults()
	}
	clone := *po
	return &clone
}

// RegisterProviderClientFactory registers the default provider client factory.
// It returns a restore function intended for tests and temporary integrations.
func RegisterProviderClientFactory(factory ProviderClientFactory) func() {
	providerRegistry.mu.Lock()
	previous := providerRegistry.clientFactory
	providerRegistry.clientFactory = factory
	providerRegistry.mu.Unlock()

	return func() {
		providerRegistry.mu.Lock()
		providerRegistry.clientFactory = previous
		providerRegistry.mu.Unlock()
	}
}

// RegisterProviderRawFetcher registers the default raw provider fetcher.
// It returns a restore function intended for tests and temporary integrations.
func RegisterProviderRawFetcher(fetcher ProviderRawFetcher) func() {
	providerRegistry.mu.Lock()
	previous := providerRegistry.rawFetcher
	providerRegistry.rawFetcher = fetcher
	providerRegistry.mu.Unlock()

	return func() {
		providerRegistry.mu.Lock()
		providerRegistry.rawFetcher = previous
		providerRegistry.mu.Unlock()
	}
}

func registeredProviderHooks() (ProviderClientFactory, ProviderRawFetcher) {
	providerRegistry.mu.RLock()
	defer providerRegistry.mu.RUnlock()
	return providerRegistry.clientFactory, providerRegistry.rawFetcher
}

// FetchStats contains metadata about a fetch operation.
// This provides transparency into API requests for debugging and monitoring.
type FetchStats struct {
	URL          string        // Endpoint that was called
	StatusCode   int           // HTTP response status code
	Latency      time.Duration // Request duration
	PayloadSize  int64         // Response body size in bytes
	ContentType  string        // Content-Type from response header
	AuthMethod   string        // How authentication was applied (Header, Query, None)
	AuthLocation string        // Where auth was placed (header name or query param name)
	AuthScheme   string        // Authentication scheme for header auth (Bearer, Basic, Direct)
}

// HumanSize returns the payload size in human-readable format.
func (s *FetchStats) HumanSize() string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	size := float64(s.PayloadSize)
	switch {
	case s.PayloadSize >= gb:
		return fmt.Sprintf("%.2f GB", size/gb)
	case s.PayloadSize >= mb:
		return fmt.Sprintf("%.2f MB", size/mb)
	case s.PayloadSize >= kb:
		return fmt.Sprintf("%.2f KB", size/kb)
	default:
		return fmt.Sprintf("%d B", s.PayloadSize)
	}
}

// getAuthInfo extracts authentication configuration from a provider.
// Returns method (Header/Query/None), location (header or query param name), and scheme (Bearer/Basic/Direct).
func getAuthInfo(provider *catalogs.Provider) (method, location, scheme string) {
	if provider == nil || provider.APIKey == nil {
		return "None", "", ""
	}
	if !provider.HasAPIKey() {
		return "None", "", ""
	}

	// Check for query parameter authentication
	if provider.APIKey.QueryParam != "" {
		return "Query", provider.APIKey.QueryParam, ""
	}

	// Header-based authentication
	header := provider.APIKey.Header
	if header == "" {
		header = "Authorization"
	}

	// Determine auth scheme
	var authScheme string
	switch provider.APIKey.Scheme {
	case catalogs.ProviderAPIKeySchemeBearer:
		authScheme = "Bearer"
	case catalogs.ProviderAPIKeySchemeBasic:
		authScheme = "Basic"
	case catalogs.ProviderAPIKeySchemeDirect:
		authScheme = "Direct"
	default:
		authScheme = "Direct"
	}

	return "Header", header, authScheme
}

// NewProviderFetcher creates a new provider fetcher for interacting with provider APIs.
// It provides a clean public interface for external packages.
// The providers parameter should contain the catalog providers to create clients for.
func NewProviderFetcher(providers catalogs.ProvidersReader, opts ...ProviderOption) *ProviderFetcher {
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
	if pf.options.clientFactory == nil {
		return false
	}

	// Check if we have a provider configuration
	provider, found := pf.providers.Get(id)
	if !found {
		return false
	}

	// Try to create a client for this provider
	_, err := pf.options.clientFactory(provider)
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

// WithProviderClientFactory configures the factory used to create provider API clients.
func WithProviderClientFactory(factory ProviderClientFactory) ProviderOption {
	return func(o *providerOptions) {
		o.clientFactory = factory
	}
}

// WithProviderRawFetcher configures the raw provider response fetcher.
func WithProviderRawFetcher(fetcher ProviderRawFetcher) ProviderOption {
	return func(o *providerOptions) {
		o.rawFetcher = fetcher
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
	options := pf.options.clone().apply(opts...)
	ctx, cancel, err := prepareProviderOperation(ctx, provider, options)
	if err != nil {
		cancel()
		return nil, err
	}
	defer cancel()

	// Get client from providers
	if options.clientFactory == nil {
		return nil, &errors.ConfigError{
			Component: string(provider.ID),
			Message:   "provider client factory is not configured",
		}
	}

	client, err := options.clientFactory(provider)
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
// The response is returned as raw bytes (JSON) without any parsing, along with fetch statistics.
func (pf *ProviderFetcher) FetchRawResponse(ctx context.Context, provider *catalogs.Provider, endpoint string, opts ...ProviderOption) ([]byte, *FetchStats, error) {
	options := pf.options.clone().apply(opts...)
	ctx, cancel, err := prepareProviderOperation(ctx, provider, options)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	defer cancel()

	if options.rawFetcher == nil {
		return nil, nil, &errors.ConfigError{
			Component: string(provider.ID),
			Message:   "provider raw fetcher is not configured",
		}
	}

	result, err := options.rawFetcher(ctx, provider, endpoint)
	if err != nil {
		return nil, nil, err
	}

	// Build stats from result
	contentType := result.Response.Header.Get("Content-Type")
	// Clean up content type (remove charset and other parameters)
	if idx := len(contentType); idx > 0 {
		for i, c := range contentType {
			if c == ';' {
				idx = i
				break
			}
		}
		contentType = contentType[:idx]
	}

	// Get authentication info from provider config
	authMethod, authLocation, authScheme := getAuthInfo(provider)

	stats := &FetchStats{
		URL:          result.RequestURL,
		StatusCode:   result.Response.StatusCode,
		Latency:      result.Latency,
		PayloadSize:  int64(len(result.Data)),
		ContentType:  contentType,
		AuthMethod:   authMethod,
		AuthLocation: authLocation,
		AuthScheme:   authScheme,
	}

	return result.Data, stats, nil
}

func prepareProviderOperation(
	ctx context.Context,
	provider *catalogs.Provider,
	options *providerOptions,
) (context.Context, context.CancelFunc, error) {
	if provider == nil {
		return ctx, func() {}, &errors.ValidationError{Field: "provider", Message: "cannot be nil"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cancel := func() {}
	if options.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
	}
	if options.loadCredentials {
		provider.LoadAPIKey()
		provider.LoadEnvVars()
	}
	if options.allowMissingKey {
		return ctx, cancel, nil
	}
	if provider.IsAPIKeyRequired() && !provider.HasAPIKey() {
		return ctx, cancel, &errors.AuthenticationError{
			Provider: string(provider.ID),
			Method:   "api_key",
			Message:  fmt.Sprintf("provider %s requires API key %s but it is not configured", provider.ID, provider.APIKey.Name),
		}
	}
	if missing := provider.MissingRequiredEnvVars(); len(missing) > 0 {
		return ctx, cancel, &errors.ConfigError{
			Component: string(provider.ID),
			Message:   fmt.Sprintf("missing required environment variables: %v", missing),
		}
	}
	return ctx, cancel, nil
}
