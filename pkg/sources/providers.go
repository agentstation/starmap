// Package sources provides public APIs for working with AI model data sources.
package sources

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ProviderClient fetches model information from a provider API.
type ProviderClient interface {
	ListModels(ctx context.Context, material ProviderCredentialMaterial) ([]catalogs.Model, error)
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
type ProviderRawFetcher func(
	context.Context,
	*catalogs.Provider,
	ProviderCredentialMaterial,
	string,
) (*RawFetchResult, error)

// ProviderFetcher provides operations for fetching models from provider APIs.
// Concrete provider clients are an explicit injected composition. Use package
// acquisition for Starmap's built-in provider implementations.
type ProviderFetcher struct {
	providers catalogs.ProvidersReader
	options   *providerOptions
}

// providerOptions holds configuration for ProviderFetcher operations.
type providerOptions struct {
	timeout            time.Duration // Context timeout for operations
	clientFactory      ProviderClientFactory
	rawFetcher         ProviderRawFetcher
	credentialResolver ProviderCredentialResolver
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
		timeout:            0, // Default: no timeout
		clientFactory:      nil,
		rawFetcher:         nil,
		credentialResolver: nil,
	}
}

func (po *providerOptions) clone() *providerOptions {
	if po == nil {
		return providerDefaults()
	}
	clone := *po
	return &clone
}

// FetchStats contains metadata about a fetch operation.
// This provides transparency into API requests for debugging and monitoring.
type FetchStats struct {
	URL          string
	StatusCode   int           // HTTP response status code
	Latency      time.Duration // Request duration
	PayloadSize  int64         // Response body size in bytes
	ContentType  string        // Content-Type from response header
	AuthMethod   string
	AuthLocation string
	AuthScheme   string // Authentication scheme for header auth (Bearer, Basic, Direct)
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
func getAuthInfo(material ProviderCredentialMaterial) (method, location, scheme string) {
	profile := material.Profile()
	if profile.ID == "" || len(profile.Placements) == 0 {
		return "None", "", ""
	}
	placement := profile.Placements[0]
	method = "Header"
	if placement.Kind == catalogs.ProviderCredentialPlacementQuery {
		method = "Query"
	}
	switch placement.Scheme {
	case catalogs.ProviderCredentialSchemeBearer:
		scheme = "Bearer"
	case catalogs.ProviderCredentialSchemeBasic:
		scheme = "Basic"
	case catalogs.ProviderCredentialSchemeDirect:
		scheme = "Direct"
	default:
		scheme = "Direct"
	}
	return method, placement.Name, scheme
}

// NewProviderFetcher creates a provider fetcher over the supplied catalog
// providers. Callers must inject the provider-client and raw-fetch roles they
// use. The root library never selects concrete provider implementations.
func NewProviderFetcher(providers catalogs.ProvidersReader, opts ...ProviderOption) *ProviderFetcher {
	options := providerDefaults().apply(opts...)

	return &ProviderFetcher{
		providers: providers,
		options:   options,
	}
}

// Providers returns the providers available to the provider fetcher.
func (pf *ProviderFetcher) Providers() *catalogs.Providers {
	result := catalogs.NewProviders()
	for _, provider := range pf.providers.List() {
		if provider.Catalog == nil {
			continue
		}
		if pf.options.credentialResolver == nil {
			continue
		}
		if _, err := pf.options.credentialResolver.ResolveCatalog(context.Background(), &provider); err != nil {
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
	if !found || provider.Catalog == nil {
		return false
	}

	// Try to create a client for this provider
	_, err := pf.options.clientFactory(provider)
	return err == nil
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

// WithProviderCredentialResolver configures catalog credential resolution.
func WithProviderCredentialResolver(resolver ProviderCredentialResolver) ProviderOption {
	return func(o *providerOptions) {
		o.credentialResolver = resolver
	}
}

// FetchModels fetches available models from a single provider's API.
// It handles credential resolution, client creation, and API communication.
// When a provider quarantines malformed records, FetchModels returns the valid
// siblings together with a non-nil *sourcepayload.QuarantineError wrapped in a
// SyncError. Callers may consume the partial result only as degraded evidence.
//
// Example:
//
//	fetcher := NewProviderFetcher(providers, WithProviderClientFactory(factory))
//	models, err := fetcher.FetchModels(ctx, provider)
//
// With options:
//
//	fetcher := NewProviderFetcher(providers,
//	    WithProviderClientFactory(factory),
//	    WithTimeout(30*time.Second),
//	)
//	models, err := fetcher.FetchModels(ctx, provider)
func (pf *ProviderFetcher) FetchModels(ctx context.Context, provider *catalogs.Provider, opts ...ProviderOption) ([]catalogs.Model, error) {
	options := pf.options.clone().apply(opts...)
	ctx, cancel, err := prepareProviderContext(ctx, provider, options)
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
	material, err := resolveCatalogCredentials(ctx, provider, options)
	if err != nil {
		return nil, err
	}

	// Fetch models from API
	models, err := client.ListModels(ctx, material)
	if err != nil {
		return models, &errors.SyncError{
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
// FetchRawResponse returns unparsed JSON bytes and fetch statistics.
func (pf *ProviderFetcher) FetchRawResponse(ctx context.Context, provider *catalogs.Provider, endpoint string, opts ...ProviderOption) ([]byte, *FetchStats, error) {
	options := pf.options.clone().apply(opts...)
	ctx, cancel, err := prepareProviderContext(ctx, provider, options)
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
	material, err := resolveCatalogCredentials(ctx, provider, options)
	if err != nil {
		return nil, nil, err
	}

	result, err := options.rawFetcher(ctx, provider, material, endpoint)
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
	authMethod, authLocation, authScheme := getAuthInfo(material)

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

func prepareProviderContext(
	ctx context.Context,
	provider *catalogs.Provider,
	options *providerOptions,
) (context.Context, context.CancelFunc, error) {
	if provider == nil {
		return ctx, func() {}, &errors.ValidationError{
			Field: "provider", Message: "cannot be nil",
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cancel := func() {}
	if options.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
	}
	return ctx, cancel, nil
}

func resolveCatalogCredentials(
	ctx context.Context,
	provider *catalogs.Provider,
	options *providerOptions,
) (ProviderCredentialMaterial, error) {
	if options.credentialResolver == nil {
		return ProviderCredentialMaterial{}, &errors.ConfigError{
			Component: string(provider.ID), Message: "provider credential resolver is not configured",
		}
	}
	material, err := options.credentialResolver.ResolveCatalog(ctx, provider)
	if err != nil {
		return ProviderCredentialMaterial{}, err
	}
	return material, nil
}
