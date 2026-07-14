// Package sources provides public APIs for working with AI model data sources.
package sources

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/auth/cloudchains"
	"github.com/agentstation/starmap/internal/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ProviderClient fetches model information for one resolved logical source.
type ProviderClient interface {
	ListModels(context.Context) ([]catalogs.Model, error)
}

// ProviderClientFactory creates a connector from a fully resolved source.
type ProviderClientFactory func(acquisition.Source) (ProviderClient, error)

// RawFetchResult contains the result of one governed raw source fetch.
type RawFetchResult struct {
	Data       []byte
	StatusCode int
	Header     http.Header
	Latency    time.Duration
}

// ProviderRawFetcher fetches the configured endpoint for one resolved source.
type ProviderRawFetcher func(context.Context, acquisition.Source) (*RawFetchResult, error)

// ProviderFetcher resolves and executes configured provider sources.
type ProviderFetcher struct {
	providers catalogs.ProvidersReader
	options   *providerOptions
}

type providerOptions struct {
	timeout       time.Duration
	retry         ProviderRetryPolicy
	resolver      *acquisition.Resolver
	resolverError error
	clientFactory ProviderClientFactory
	customFactory bool
	rawFetcher    ProviderRawFetcher
}

func (options *providerOptions) apply(values ...ProviderOption) *providerOptions {
	for _, value := range values {
		value(options)
	}
	return options
}

func (options *providerOptions) clone() *providerOptions {
	if options == nil {
		return providerDefaults()
	}
	clone := *options
	return &clone
}

// ProviderOption configures ProviderFetcher behavior.
type ProviderOption func(*providerOptions)

func providerDefaults() *providerOptions {
	cloudRegistry, err := cloudchains.NewRegistry()
	authResolver := auth.NewResolver()
	if err == nil {
		authResolver = auth.NewResolver(auth.WithCloudChainRegistry(cloudRegistry))
	}
	return &providerOptions{
		retry:         DefaultProviderRetryPolicy(),
		resolver:      acquisition.NewResolver(acquisition.WithAuthResolver(authResolver)),
		resolverError: err,
		clientFactory: func(source acquisition.Source) (ProviderClient, error) { return registry.New(source) },
		rawFetcher:    defaultProviderRawFetcher,
	}
}

func defaultProviderRawFetcher(ctx context.Context, source acquisition.Source) (*RawFetchResult, error) {
	result, err := registry.FetchRaw(ctx, source)
	if err != nil {
		return nil, err
	}
	return &RawFetchResult{Data: result.Data, StatusCode: result.StatusCode, Header: result.Header.Clone(), Latency: result.Latency}, nil
}

// WithTimeout sets a timeout for provider operations.
func WithTimeout(timeout time.Duration) ProviderOption {
	return func(options *providerOptions) { options.timeout = timeout }
}

// WithProviderRetryPolicy configures bounded retry for provider model calls.
func WithProviderRetryPolicy(policy ProviderRetryPolicy) ProviderOption {
	return func(options *providerOptions) { options.retry = policy }
}

// WithProviderSourceResolver supplies a request-scoped source resolver.
func WithProviderSourceResolver(resolver *acquisition.Resolver) ProviderOption {
	return func(options *providerOptions) {
		options.resolver = resolver
		options.resolverError = nil
	}
}

// WithProviderClientFactory supplies per-instance connector construction.
func WithProviderClientFactory(factory ProviderClientFactory) ProviderOption {
	return func(options *providerOptions) {
		options.clientFactory = factory
		options.customFactory = true
	}
}

// WithProviderRawFetcher supplies per-instance governed raw acquisition.
func WithProviderRawFetcher(fetcher ProviderRawFetcher) ProviderOption {
	return func(options *providerOptions) { options.rawFetcher = fetcher }
}

// NewProviderFetcher creates a source-aware provider fetcher.
func NewProviderFetcher(providers catalogs.ProvidersReader, values ...ProviderOption) *ProviderFetcher {
	return &ProviderFetcher{providers: providers, options: providerDefaults().apply(values...)}
}

// Providers returns providers with at least one executable source type. Runtime
// credential readiness is reported separately and never mutates this catalog.
func (fetcher *ProviderFetcher) Providers() *catalogs.Providers {
	result := catalogs.NewProviders()
	if fetcher == nil || fetcher.providers == nil {
		return result
	}
	for _, provider := range fetcher.providers.List() {
		if fetcher.HasClient(provider.ID) {
			copied := provider
			_ = result.Add(&copied)
		}
	}
	return result
}

// List returns provider IDs with at least one executable source type.
func (fetcher *ProviderFetcher) List() []catalogs.ProviderID {
	if fetcher == nil || fetcher.providers == nil {
		return nil
	}
	ids := make([]catalogs.ProviderID, 0)
	for _, provider := range fetcher.providers.List() {
		if fetcher.HasClient(provider.ID) {
			ids = append(ids, provider.ID)
		}
	}
	return ids
}

// HasClient reports whether any configured source has a registered connector.
func (fetcher *ProviderFetcher) HasClient(id catalogs.ProviderID) bool {
	if fetcher == nil || fetcher.providers == nil || fetcher.options.clientFactory == nil {
		return false
	}
	provider, found := fetcher.providers.Get(id)
	if !found || provider.Catalog == nil {
		return false
	}
	if fetcher.options.customFactory {
		for _, source := range provider.Catalog.Sources {
			// Application-only entries describe products whose inventory is
			// curated or embedded. They are not acquisition protocols, even when
			// tests inject a general-purpose client factory.
			if source.Endpoint.Type != catalogs.EndpointTypeApplication {
				return true
			}
		}
		return false
	}
	for _, source := range provider.Catalog.Sources {
		if registry.Supports(source.Endpoint.Type) {
			return true
		}
	}
	return false
}

// FetchModels executes every configured logical source once. Optional sources
// may be unavailable; required sources fail before transport.
func (fetcher *ProviderFetcher) FetchModels(ctx context.Context, provider *catalogs.Provider, values ...ProviderOption) ([]catalogs.Model, error) {
	results, err := fetcher.FetchModelSources(ctx, provider, values...)
	if err != nil {
		return nil, err
	}
	models := make([]catalogs.Model, 0)
	var sourceErrors []error
	for _, result := range results {
		if result.Err != nil {
			if !result.Skipped {
				sourceErrors = append(sourceErrors, result.Err)
			}
			continue
		}
		models = append(models, result.Models...)
	}
	if len(sourceErrors) > 0 {
		return models, stderrors.Join(sourceErrors...)
	}
	return models, nil
}

// ProviderSourceModels is one independently resolved logical source result.
type ProviderSourceModels struct {
	SourceID      string
	Models        []catalogs.Model
	Scope         catalogs.ProviderObservationScope
	AuthMethod    catalogs.ProviderCredentialID
	Topology      catalogs.ProviderSourceTopology
	Authenticated bool
	Skipped       bool
	Err           error
}

// FetchSource executes one exact configured logical source and returns its
// independently classified result. The returned models are caller-owned.
func (fetcher *ProviderFetcher) FetchSource(ctx context.Context, provider *catalogs.Provider, sourceID string, values ...ProviderOption) (ProviderSourceModels, error) {
	if provider == nil {
		return ProviderSourceModels{}, &errors.ValidationError{Field: string(SchemaRecordProvider), Message: validationCannotBeNil}
	}
	var config *catalogs.ProviderSource
	if provider.Catalog != nil {
		for index := range provider.Catalog.Sources {
			if provider.Catalog.Sources[index].ID == sourceID {
				selected := provider.Catalog.Sources[index]
				config = &selected
				break
			}
		}
	}
	if config == nil {
		return ProviderSourceModels{}, &errors.NotFoundError{Resource: "provider source", ID: string(provider.ID) + "/" + sourceID}
	}
	models, resolved, err := fetcher.fetchSourceModels(ctx, provider, sourceID, values...)
	result := ProviderSourceModels{SourceID: sourceID, Topology: normalizedTopology(config.Topology), Err: err}
	if err != nil {
		return result, err
	}
	authenticated := !resolved.Auth().Anonymous()
	result.Models = models
	result.Scope = config.ObservationScope.Scope(authenticated)
	result.AuthMethod = resolved.Auth().Method()
	result.Authenticated = authenticated
	return result, nil
}

// FetchModelSources executes each configured logical source once while
// retaining source identity and publication scope for observation policy.
func (fetcher *ProviderFetcher) FetchModelSources(ctx context.Context, provider *catalogs.Provider, values ...ProviderOption) ([]ProviderSourceModels, error) {
	if provider == nil {
		return nil, &errors.ValidationError{Field: string(SchemaRecordProvider), Message: validationCannotBeNil}
	}
	if provider.Catalog == nil || len(provider.Catalog.Sources) == 0 {
		return nil, &errors.ValidationError{Field: "provider.catalog.sources", Message: "must contain at least one source"}
	}
	results := make([]ProviderSourceModels, 0, len(provider.Catalog.Sources))
	for _, source := range provider.Catalog.Sources {
		result, err := fetcher.FetchSource(ctx, provider, source.ID, values...)
		if err != nil {
			if source.Optional && optionalSourceUnavailable(err) {
				result.Skipped = true
				results = append(results, result)
				continue
			}
			results = append(results, result)
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

func normalizedTopology(topology catalogs.ProviderSourceTopology) catalogs.ProviderSourceTopology {
	if topology == "" {
		return catalogs.ProviderSourceTopologySingleEndpoint
	}
	return topology
}

func optionalSourceUnavailable(err error) bool {
	return stderrors.Is(err, errors.ErrNotFound) || stderrors.Is(err, errors.ErrAPIKeyRequired)
}

// FetchSourceModels resolves and executes one exact logical source.
func (fetcher *ProviderFetcher) FetchSourceModels(ctx context.Context, provider *catalogs.Provider, sourceID string, values ...ProviderOption) ([]catalogs.Model, error) {
	models, _, err := fetcher.fetchSourceModels(ctx, provider, sourceID, values...)
	return models, err
}

func (fetcher *ProviderFetcher) fetchSourceModels(ctx context.Context, provider *catalogs.Provider, sourceID string, values ...ProviderOption) ([]catalogs.Model, acquisition.Source, error) {
	options := fetcher.options.clone().apply(values...)
	ctx, cancel, err := prepareProviderOperation(ctx, provider, sourceID, options)
	if err != nil {
		cancel()
		return nil, acquisition.Source{}, err
	}
	defer cancel()
	resolved, err := options.resolver.Resolve(ctx, provider, sourceID)
	if err != nil {
		return nil, acquisition.Source{}, err
	}
	if options.clientFactory == nil {
		return nil, acquisition.Source{}, &errors.ConfigError{Component: string(provider.ID) + "/" + sourceID, Message: "provider client factory is not configured"}
	}
	client, err := options.clientFactory(resolved)
	if err != nil {
		return nil, acquisition.Source{}, errors.WrapResource("get", "source client", string(provider.ID)+"/"+sourceID, err)
	}
	var models []catalogs.Model
	err = RetryProviderCall(ctx, options.retry, func(callCtx context.Context) (RetryHint, error) {
		fetched, fetchErr := client.ListModels(callCtx)
		if fetchErr == nil {
			models = applyProviderSourceDefaults(fetched, resolved.Config())
		}
		return RetryHint{}, fetchErr
	})
	if err != nil {
		return nil, acquisition.Source{}, &errors.SyncError{Provider: string(provider.ID), Err: err}
	}
	return models, resolved, nil
}

func applyProviderSourceDefaults(models []catalogs.Model, source catalogs.ProviderSource) []catalogs.Model {
	result := make([]catalogs.Model, len(models))
	for index := range models {
		model := catalogs.DeepCopyModel(models[index])
		if source.Offering != nil {
			defaults := source.Offering
			if model.OfferingAccess == nil && model.InvocationAPIs == nil {
				access := defaults.Access
				access.APIs = append([]catalogs.InvocationAPI(nil), defaults.Access.APIs...)
				model.OfferingAccess = &access
			}
			if model.OfferingEndpoint == (catalogs.ProviderOfferingEndpoint{}) {
				model.OfferingEndpoint = defaults.Endpoint
			}
			if model.OfferingDeployment.Type == "" {
				model.OfferingDeployment = defaults.Deployment
			}
			if model.OfferingRegions == nil {
				model.OfferingRegions = copyCloudRegions(defaults.Regions)
			}
		}
		result[index] = model
	}
	return result
}

func copyCloudRegions(regions []catalogs.CloudRegion) []catalogs.CloudRegion {
	result := append([]catalogs.CloudRegion(nil), regions...)
	for index := range result {
		if regions[index].Residency != nil {
			residency := *regions[index].Residency
			residency.Countries = append([]string(nil), regions[index].Residency.Countries...)
			result[index].Residency = &residency
		}
	}
	return result
}

// FetchRawResponse fetches exactly one configured source endpoint. Callers
// cannot substitute an arbitrary URL that would receive source credentials.
func (fetcher *ProviderFetcher) FetchRawResponse(ctx context.Context, provider *catalogs.Provider, sourceID string, values ...ProviderOption) ([]byte, *FetchStats, error) {
	options := fetcher.options.clone().apply(values...)
	ctx, cancel, err := prepareProviderOperation(ctx, provider, sourceID, options)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	defer cancel()
	resolved, err := options.resolver.Resolve(ctx, provider, sourceID)
	if err != nil {
		return nil, nil, err
	}
	if options.rawFetcher == nil {
		return nil, nil, &errors.ConfigError{Component: string(provider.ID) + "/" + sourceID, Message: "provider raw fetcher is not configured"}
	}
	result, err := options.rawFetcher(ctx, resolved)
	if err != nil {
		return nil, nil, err
	}
	if result == nil || result.Header == nil {
		return nil, nil, &errors.ValidationError{Field: "raw_fetch_result", Message: "response metadata is required"}
	}
	contentType := result.Header.Get("Content-Type")
	if index := strings.IndexByte(contentType, ';'); index >= 0 {
		contentType = contentType[:index]
	}
	method, location, scheme := authInfo(resolved)
	return result.Data, &FetchStats{
		StatusCode: result.StatusCode, Latency: result.Latency,
		PayloadSize: int64(len(result.Data)), ContentType: contentType,
		AuthMethod: method, AuthLocation: location, AuthScheme: scheme,
	}, nil
}

func prepareProviderOperation(ctx context.Context, provider *catalogs.Provider, sourceID string, options *providerOptions) (context.Context, context.CancelFunc, error) {
	if provider == nil {
		return ctx, func() {}, &errors.ValidationError{Field: string(SchemaRecordProvider), Message: validationCannotBeNil}
	}
	if strings.TrimSpace(sourceID) == "" {
		return ctx, func() {}, &errors.ValidationError{Field: "source_id", Message: validationIsRequired}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cancel := func() {}
	if options.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
	}
	if options.resolverError != nil {
		return ctx, cancel, &errors.ConfigError{Component: "cloud chain registry", Message: "cannot initialize source resolver", Err: options.resolverError}
	}
	if options.resolver == nil {
		return ctx, cancel, &errors.ConfigError{Component: string(provider.ID) + "/" + sourceID, Message: "source resolver is not configured"}
	}
	return ctx, cancel, nil
}

// FetchStats contains safe metadata about a source fetch operation.
type FetchStats struct {
	StatusCode   int
	Latency      time.Duration
	PayloadSize  int64
	ContentType  string
	AuthMethod   string
	AuthLocation string
	AuthScheme   string
}

// HumanSize returns the payload size in human-readable form.
func (stats *FetchStats) HumanSize() string {
	const kb, mb, gb = 1024, 1024 * 1024, 1024 * 1024 * 1024
	size := float64(stats.PayloadSize)
	switch {
	case stats.PayloadSize >= gb:
		return fmt.Sprintf("%.2f GB", size/gb)
	case stats.PayloadSize >= mb:
		return fmt.Sprintf("%.2f MB", size/mb)
	case stats.PayloadSize >= kb:
		return fmt.Sprintf("%.2f KB", size/kb)
	default:
		return fmt.Sprintf("%d B", stats.PayloadSize)
	}
}

func authInfo(source acquisition.Source) (method, location, scheme string) {
	if source.Auth().Anonymous() {
		return "None", "", ""
	}
	credential, found := source.Provider().Credentials[source.Auth().Method()]
	if !found {
		return "SDK", string(source.Auth().Method()), ""
	}
	normalized, err := credential.Normalized(source.Auth().Method())
	if err != nil {
		return "Resolved", string(source.Auth().Method()), ""
	}
	if normalized.Transport.QueryParam != "" {
		return "Query", normalized.Transport.QueryParam, ""
	}
	scheme = string(normalized.Transport.Scheme)
	if scheme != "" {
		scheme = strings.ToUpper(scheme[:1]) + scheme[1:]
	}
	return "Header", normalized.Transport.Header, scheme
}
