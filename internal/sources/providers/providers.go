// Package providers implements the provider-backed catalog source.
package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// ClientFactory creates a client for a provider.
type ClientFactory = sources.ProviderClientFactory

// SourceOption configures the provider source.
type SourceOption func(*sourceOptions)

type sourceOptions struct {
	clientFactory  ClientFactory
	maxConcurrency int
}

// Source fetches models from all provider APIs concurrently.
type Source struct {
	providers      catalogs.ProvidersReader // Provider configs injected during setup
	fetcher        *sources.ProviderFetcher
	maxConcurrency int
}

var _ sources.Source = (*Source)(nil)

// New creates a new provider API source with the given provider configurations.
func New(providers catalogs.ProvidersReader, opts ...SourceOption) *Source {
	options := sourceOptions{
		maxConcurrency: constants.MaxConcurrentProviders,
	}
	for _, opt := range opts {
		opt(&options)
	}
	fetcherOptions := make([]sources.ProviderOption, 0, 1)
	if options.clientFactory != nil {
		fetcherOptions = append(fetcherOptions, sources.WithProviderClientFactory(options.clientFactory))
	}
	return &Source{
		providers:      providers,
		fetcher:        sources.NewProviderFetcher(providers, fetcherOptions...),
		maxConcurrency: options.maxConcurrency,
	}
}

// WithClientFactory configures the factory used to create provider clients.
func WithClientFactory(factory ClientFactory) SourceOption {
	return func(s *sourceOptions) {
		s.clientFactory = factory
	}
}

// WithMaxConcurrency configures the maximum number of provider fetches in flight.
func WithMaxConcurrency(maxConcurrency int) SourceOption {
	return func(s *sourceOptions) {
		s.maxConcurrency = maxConcurrency
	}
}

// ID returns the ID of this source.
func (s *Source) ID() sources.ID { return sources.ProvidersID }

// Name returns the human-friendly name of this source.
func (s *Source) Name() string { return "Providers" }

// providerModels holds models fetched from a specific provider.
type providerModels struct {
	providerID catalogs.ProviderID
	models     []*catalogs.Model
	rejected   int
	issues     []sources.ObservationIssue
}

// Observe returns a new immutable provider catalog without retaining result state.
func (s *Source) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	ctx = logging.WithSource(ctx, s.ID().String())
	// Apply options
	options := sources.Defaults().Apply(opts...)

	// Create a new catalog to build into
	catalog := catalogs.NewEmpty()

	// Set the default merge strategy for provider catalog (fresh API data)
	catalog.SetMergeStrategy(catalogs.MergeReplaceAll)

	// Check if we have provider configs
	if s.providers == nil {
		// Can't fetch without provider configs
		return s.observation(catalog, nil, sources.ObservationRecordCounts{})
	}

	// Determine which providers to sync
	var providerIDs []catalogs.ProviderID
	if options.ProviderID != nil {
		providerIDs = []catalogs.ProviderID{*options.ProviderID}
	} else {
		// Get all provider IDs from the providers collection
		for _, p := range s.providers.List() {
			providerIDs = append(providerIDs, p.ID)
		}
	}

	// Get provider configs from injected providers
	var providerConfigs []*catalogs.Provider
	for _, id := range providerIDs {
		if p, found := s.providers.Get(id); found {
			providerConfigs = append(providerConfigs, p)
		}
	}

	if len(providerConfigs) == 0 {
		return s.observation(catalog, nil, sources.ObservationRecordCounts{}) // No providers to sync
	}

	// Add provider configurations to the catalog first
	issues := make([]sources.ObservationIssue, 0)
	for _, provider := range providerConfigs {
		// The configured catalog may contain embedded or last-known-good models.
		// Provider observations contain only models returned by this live call;
		// bootstrap data remains a separate local-catalog observation.
		providerConfig := catalogs.DeepCopyProvider(*provider)
		providerConfig.Models = nil
		if err := catalog.SetProvider(providerConfig); err != nil {
			logging.FromContext(ctx).Warn().
				Err(err).
				Str("provider_id", string(provider.ID)).
				Msg("Failed to add provider to catalog")
			issues = append(issues, providerIssue(provider.ID, sources.ObservationIssueCodeInvalidRecord, err))
		}
	}

	logger := logging.FromContext(ctx)
	logger.Info().
		Int("provider_count", len(providerConfigs)).
		Int("max_concurrency", s.effectiveMaxConcurrency(len(providerConfigs))).
		Msg("Syncing providers concurrently")

	// Sync all providers concurrently
	var wg sync.WaitGroup
	resultChan := make(chan providerModels, len(providerConfigs))
	semaphore := make(chan struct{}, s.effectiveMaxConcurrency(len(providerConfigs)))

	for _, provider := range providerConfigs {
		wg.Add(1)
		go func(p *catalogs.Provider) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := providerModels{providerID: p.ID}

			logger := logging.WithProvider(ctx, string(p.ID))
			models, err := s.fetcher.FetchModels(logger, p)
			if err != nil {
				logging.Ctx(logger).Warn().
					Err(err).
					Str("provider_id", string(p.ID)).
					Msg("Provider observation degraded")
				result.issues = append(result.issues, classifyProviderFetchIssue(p.ID, err))
				resultChan <- result
				return
			}

			result.models, result.rejected, result.issues = quarantineProviderModels(p.ID, models)
			resultChan <- result

			logging.Ctx(logger).Info().
				Str("provider_id", string(p.ID)).
				Int("model_count", len(models)).
				Msg("Fetched models")
		}(provider)
	}

	wg.Wait()
	close(resultChan)

	// Process results and update catalog
	records := sources.ObservationRecordCounts{}
	for result := range resultChan {
		issues = append(issues, result.issues...)
		records.Rejected += result.rejected
		if len(result.models) == 0 {
			continue
		}

		// Get the provider from catalog
		provider, err := catalog.Provider(result.providerID)
		if err != nil {
			logger.Warn().
				Err(err).
				Str("provider_id", string(result.providerID)).
				Msg("Failed to get provider from catalog")
			issues = append(issues, providerIssue(result.providerID, sources.ObservationIssueCodeInvalidRecord, err))
			records.Rejected += len(result.models)
			continue
		}

		// Initialize Models map if nil
		if provider.Models == nil {
			provider.Models = make(map[string]*catalogs.Model)
		}

		// Associate models with provider
		for _, model := range result.models {
			// Create copy to avoid modifying original
			modelCopy := model
			// Associate model with provider
			provider.Models[modelCopy.ID] = modelCopy
		}

		// Update the provider in the catalog with its models
		if err := catalog.SetProvider(provider); err != nil {
			logger.Warn().
				Err(err).
				Str("provider_id", string(result.providerID)).
				Msg("Failed to update provider with models")
			issues = append(issues, providerIssue(result.providerID, sources.ObservationIssueCodeInvalidRecord, err))
			records.Rejected += len(result.models)
			continue
		}
		records.Accepted += len(result.models)

		// Note: Saving is now handled by the catalog's Save() method
		// Sources should only create catalogs, not persist them
	}

	return s.observation(catalog, issues, records)
}

func (s *Source) effectiveMaxConcurrency(providerCount int) int {
	if providerCount <= 0 {
		return 1
	}
	if s.maxConcurrency <= 0 {
		return 1
	}
	if s.maxConcurrency > providerCount {
		return providerCount
	}
	return s.maxConcurrency
}

func (s *Source) observation(
	builder *catalogs.Builder,
	issues []sources.ObservationIssue,
	records sources.ObservationRecordCounts,
) (sources.Observation, error) {
	catalog, err := builder.Build()
	if err != nil {
		return sources.Observation{}, pkgerrors.WrapResource("publish", "provider source observation", "", err)
	}
	completeness := sources.ObservationCompletenessComplete
	status := sources.ObservationStatusSucceeded
	if len(issues) > 0 {
		completeness = sources.ObservationCompletenessPartial
		status = sources.ObservationStatusDegraded
	}
	return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
		ObservedAt:   time.Now().UTC(),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: completeness,
		Status:       status,
		Records:      records,
		Issues:       issues,
	})
}

func providerIssue(providerID catalogs.ProviderID, code sources.ObservationIssueCode, err error) sources.ObservationIssue {
	return sources.ObservationIssue{
		Scope:   sources.ObservationIssueScopeProvider,
		Code:    code,
		Subject: string(providerID),
		Message: err.Error(),
	}
}

func classifyProviderFetchIssue(providerID catalogs.ProviderID, err error) sources.ObservationIssue {
	code := sources.ObservationIssueCodeFetchFailed
	var authenticationErr *pkgerrors.AuthenticationError
	var configurationErr *pkgerrors.ConfigError
	var parseErr *pkgerrors.ParseError
	switch {
	case errors.As(err, &authenticationErr):
		code = sources.ObservationIssueCodeMissingCredentials
	case errors.As(err, &configurationErr):
		code = sources.ObservationIssueCodeConfiguration
	case errors.As(err, &parseErr):
		code = sources.ObservationIssueCodeSchemaDrift
	}
	return providerIssue(providerID, code, err)
}

func quarantineProviderModels(providerID catalogs.ProviderID, models []catalogs.Model) ([]*catalogs.Model, int, []sources.ObservationIssue) {
	total := len(models)
	accepted := make([]*catalogs.Model, 0, len(models))
	issues := make([]sources.ObservationIssue, 0)
	if len(models) > constants.MaxCatalogModels {
		issues = append(issues, sources.ObservationIssue{
			Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodePayloadLimit,
			Subject: string(providerID), Message: "provider model count exceeds maximum; excess records quarantined",
		})
		models = models[:constants.MaxCatalogModels]
	}
	seen := make(map[string]struct{}, len(models))
	for index := range models {
		model := models[index]
		modelID := model.ID
		subject := fmt.Sprintf("%s/record[%d]", providerID, index)
		var err error
		switch {
		case strings.TrimSpace(modelID) == "":
			err = &pkgerrors.ValidationError{Field: "model.id", Value: model.ID, Message: "is required"}
		case modelID != strings.TrimSpace(modelID):
			err = &pkgerrors.ValidationError{Field: "model.id", Value: model.ID, Message: "must not contain leading or trailing whitespace"}
		case strings.IndexFunc(modelID, unicode.IsControl) >= 0:
			err = &pkgerrors.ValidationError{Field: "model.id", Value: model.ID, Message: "must not contain control characters"}
		case strings.TrimSpace(model.Name) == "":
			subject = string(providerID) + "/" + modelID
			err = &pkgerrors.ValidationError{Field: "model.name", Value: model.Name, Message: "is required"}
		case strings.IndexFunc(model.Name, unicode.IsControl) >= 0:
			subject = string(providerID) + "/" + modelID
			err = &pkgerrors.ValidationError{Field: "model.name", Value: model.Name, Message: "must not contain control characters"}
		case hasProviderModelID(seen, modelID):
			subject = string(providerID) + "/" + modelID
			err = &pkgerrors.ValidationError{Field: "model.id", Value: modelID, Message: "must be unique within provider observation"}
		}
		if err != nil {
			issues = append(issues, sources.ObservationIssue{
				Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord,
				Subject: subject, Message: err.Error(),
			})
			continue
		}
		seen[modelID] = struct{}{}
		accepted = append(accepted, &model)
	}
	return accepted, total - len(accepted), issues
}

func hasProviderModelID(seen map[string]struct{}, id string) bool {
	_, exists := seen[id]
	return exists
}

// Cleanup releases any resources.
func (s *Source) Cleanup() error {
	// ProvidersSource doesn't hold persistent resources
	return nil
}

// Dependencies returns the list of external dependencies.
// Provider source has no external dependencies.
func (s *Source) Dependencies() []sources.Dependency {
	return nil
}

// IsOptional returns whether this source is optional.
// Provider source is required - it's the core data source.
func (s *Source) IsOptional() bool {
	return false
}
