// Package providers implements the provider-backed catalog source.
package providers

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

// ClientFactory creates a client for a provider.
type ClientFactory = sources.ProviderClientFactory

// SourceOption configures the provider source.
type SourceOption func(*sourceOptions)

type sourceOptions struct {
	clientFactory         ClientFactory
	credentialResolver    sources.ProviderCredentialResolver
	attemptSink           AttemptSink
	now                   func() time.Time
	maxConcurrency        int
	requireCanonicalLinks bool
}

// AttemptSink receives one terminal provider attempt as soon as it completes.
// It runs on the fetching goroutine, so an implementation must not block.
type AttemptSink func(sources.ProviderAttempt)

// Source fetches models from all provider APIs concurrently.
type Source struct {
	providers             catalogs.ProvidersReader // Provider configs injected during setup
	fetcher               *sources.ProviderFetcher
	credentials           *credentialMemo
	attemptSink           AttemptSink
	now                   func() time.Time
	maxConcurrency        int
	requireCanonicalLinks bool
}

var _ sources.Source = (*Source)(nil)

// New creates a new provider API source with the given provider configurations.
func New(providers catalogs.ProvidersReader, opts ...SourceOption) *Source {
	options := sourceOptions{
		credentialResolver:    auth.NewResolver(),
		now:                   time.Now,
		maxConcurrency:        constants.MaxConcurrentProviders,
		requireCanonicalLinks: true,
	}
	for _, opt := range opts {
		opt(&options)
	}
	// The pre-flight check and the fetcher share one memo, so each run resolves
	// the credential of one provider one time.
	memo := newCredentialMemo(options.credentialResolver)
	var resolver sources.ProviderCredentialResolver
	if memo != nil {
		resolver = memo
	}
	fetcherOptions := []sources.ProviderOption{
		sources.WithProviderCredentialResolver(resolver),
	}
	if options.clientFactory != nil {
		fetcherOptions = append(fetcherOptions, sources.WithProviderClientFactory(options.clientFactory))
	}
	return &Source{
		providers:             providers,
		fetcher:               sources.NewProviderFetcher(providers, fetcherOptions...),
		credentials:           memo,
		attemptSink:           options.attemptSink,
		now:                   options.now,
		maxConcurrency:        options.maxConcurrency,
		requireCanonicalLinks: options.requireCanonicalLinks,
	}
}

// WithAttemptSink observes each terminal provider attempt as it completes.
// Partial publication uses the sink to release finished providers before every
// provider answers.
func WithAttemptSink(sink AttemptSink) SourceOption {
	return func(options *sourceOptions) {
		options.attemptSink = sink
	}
}

// WithClock injects the attempt clock. Tests use it to keep timing exact.
func WithClock(now func() time.Time) SourceOption {
	return func(options *sourceOptions) {
		if now != nil {
			options.now = now
		}
	}
}

// WithCredentialResolver selects the deployment-owned catalog credential
// resolver used for each provider observation.
func WithCredentialResolver(resolver sources.ProviderCredentialResolver) SourceOption {
	return func(options *sourceOptions) {
		if resolver != nil {
			options.credentialResolver = resolver
		}
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
	attempt    sources.ProviderAttempt
}

// Observe returns a new immutable provider catalog without retaining result state.
func (s *Source) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	observation, _, err := s.ObserveAttempts(ctx, opts...)
	return observation, err
}

// ObserveAttempts returns the provider observation together with one terminal
// attempt per eligible provider. The attempts report which providers answered,
// which failed, and which acquisition skipped without a request.
func (s *Source) ObserveAttempts(
	ctx context.Context,
	opts ...sources.Option,
) (sources.Observation, []sources.ProviderAttempt, error) {
	ctx = logging.WithSource(ctx, s.ID().String())
	s.credentials.forget()
	// Apply options
	options := sources.Defaults().Apply(opts...)

	// Create a new catalog to build into
	catalog := catalogs.NewEmpty()

	// Set the default merge strategy for provider catalog (fresh API data)
	catalog.SetMergeStrategy(catalogs.MergeReplaceAll)

	// Check if we have provider configs
	if s.providers == nil {
		// Cannot fetch without provider configs
		observation, err := s.observation(catalog, nil, sources.ObservationRecordCounts{})
		return observation, nil, err
	}

	// Determine which providers to sync
	var providerIDs []catalogs.ProviderID
	if options.ProviderID != nil {
		providerIDs = []catalogs.ProviderID{*options.ProviderID}
	} else {
		// Get all provider IDs from the providers collection
		for _, p := range s.providers.List() {
			if p.Catalog == nil {
				continue
			}
			providerIDs = append(providerIDs, p.ID)
		}
	}

	// Get provider configs from injected providers
	var providerConfigs []*catalogs.Provider
	for _, id := range providerIDs {
		if p, found := s.providers.Get(id); found && p.Catalog != nil {
			providerConfigs = append(providerConfigs, p)
		}
	}

	if len(providerConfigs) == 0 {
		// No providers to observe.
		observation, err := s.observation(catalog, nil, sources.ObservationRecordCounts{})
		return observation, nil, err
	}

	// Add provider configurations to the catalog first
	issues := make([]sources.ObservationIssue, 0)
	for _, provider := range providerConfigs {
		// The configured catalog may contain embedded or last-known-good models.
		// Provider observations contain only models returned by this live call.
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
		wg.Go(func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := providerModels{providerID: provider.ID}
			started := s.now()

			logger := logging.WithProvider(ctx, string(provider.ID))
			state, credentialErr := s.preflight(logger, provider)
			if state != credentialReady {
				reason := sources.ProviderReasonCredentialUnavailable
				if state == credentialInvalid {
					reason = sources.ClassifyProviderReason(credentialErr)
				}
				logging.Ctx(logger).Info().
					Str("provider_id", string(provider.ID)).
					Str("reason", reason.String()).
					Msg("Provider skipped without a request")
				result.issues = append(result.issues, skipIssue(provider.ID, reason))
				result.attempt = skippedAttempt(provider.ID, reason, started, s.now())
				s.publishAttempt(result.attempt)
				resultChan <- result
				return
			}

			models, err := s.fetcher.FetchModels(logger, provider)
			if err != nil {
				var quarantineErr *sourcepayload.QuarantineError
				if errors.As(err, &quarantineErr) {
					result.models, result.rejected, result.issues = quarantineProviderModels(provider.ID, models)
					if s.requireCanonicalLinks {
						result.models, result.rejected, result.issues = linkReviewedProviderModels(
							provider,
							result.models,
							result.rejected,
							result.issues,
						)
					}
					result.rejected += quarantineErr.Report.Rejected
					result.issues = append(result.issues, providerRecordIssues(provider.ID, quarantineErr)...)
					result.attempt = succeededAttempt(provider.ID, len(result.models), started, s.now())
					s.publishAttempt(result.attempt)
					resultChan <- result
					return
				}
				logging.Ctx(logger).Warn().
					Err(err).
					Str("provider_id", string(provider.ID)).
					Msg("Provider observation degraded")
				result.issues = append(result.issues, classifyProviderFetchIssue(provider.ID, err))
				result.attempt = failedAttempt(provider.ID, err, started, s.now())
				s.publishAttempt(result.attempt)
				resultChan <- result
				return
			}

			result.models, result.rejected, result.issues = quarantineProviderModels(provider.ID, models)
			if s.requireCanonicalLinks {
				result.models, result.rejected, result.issues = linkReviewedProviderModels(
					provider,
					result.models,
					result.rejected,
					result.issues,
				)
			}
			result.attempt = succeededAttempt(provider.ID, len(result.models), started, s.now())
			s.publishAttempt(result.attempt)
			resultChan <- result

			logging.Ctx(logger).Info().
				Str("provider_id", string(provider.ID)).
				Int("model_count", len(models)).
				Msg("Fetched models")
		})
	}

	wg.Wait()
	close(resultChan)

	// Process results and update catalog
	records := sources.ObservationRecordCounts{}
	attempts := make([]sources.ProviderAttempt, 0, len(providerConfigs))
	for result := range resultChan {
		issues = append(issues, result.issues...)
		if result.attempt.ProviderID != "" {
			attempts = append(attempts, result.attempt)
		}
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

	slices.SortFunc(attempts, func(a, b sources.ProviderAttempt) int {
		return strings.Compare(string(a.ProviderID), string(b.ProviderID))
	})
	observation, err := s.observation(catalog, issues, records)
	return observation, attempts, err
}

// publishAttempt hands one terminal attempt to the configured sink.
func (s *Source) publishAttempt(attempt sources.ProviderAttempt) {
	if s.attemptSink == nil {
		return
	}
	s.attemptSink(attempt)
}

func linkReviewedProviderModels(
	provider *catalogs.Provider,
	models []*catalogs.Model,
	rejected int,
	issues []sources.ObservationIssue,
) ([]*catalogs.Model, int, []sources.ObservationIssue) {
	linked := make([]*catalogs.Model, 0, len(models))
	for _, model := range models {
		configured := provider.Models[model.ID]
		if configured == nil || configured.ModelRef == "" {
			unlinkedModel := catalogs.DeepCopyModel(*model)
			unlinkedModel.ModelRef = ""
			linked = append(linked, &unlinkedModel)
			continue
		}
		if _, _, err := catalogs.ParseModelDefinitionID(configured.ModelRef); err != nil {
			rejected++
			issues = append(issues, sources.ObservationIssue{
				Scope:   sources.ObservationIssueScopeRecord,
				Code:    sources.ObservationIssueCodeInvalidRecord,
				Subject: string(provider.ID) + "/" + model.ID,
				Message: "configured canonical model link is invalid: " + err.Error(),
			})
			continue
		}
		linkedModel := catalogs.DeepCopyModel(*model)
		linkedModel.ModelRef = configured.ModelRef
		linked = append(linked, &linkedModel)
	}
	return linked, rejected, issues
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
	catalog, err := catalogs.NewObservationCatalog(builder)
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

func providerRecordIssues(providerID catalogs.ProviderID, quarantine *sourcepayload.QuarantineError) []sources.ObservationIssue {
	if quarantine == nil {
		return nil
	}
	issues := make([]sources.ObservationIssue, 0, len(quarantine.Report.Issues)+1)
	for _, issue := range quarantine.Report.Issues {
		issues = append(issues, sources.ObservationIssue{
			Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord,
			Subject: string(providerID) + "/" + issue.Subject, Message: issue.Err.Error(),
		})
	}
	if quarantine.Report.Truncated {
		issues = append(issues, sources.ObservationIssue{
			Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodePayloadLimit,
			Subject: string(providerID), Message: "provider model count exceeds maximum; excess records quarantined",
		})
	}
	return issues
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
	// ProvidersSource does not hold persistent resources
	return nil
}

// Dependencies returns the list of external dependencies.
// Provider source has no external dependencies.
func (s *Source) Dependencies() []sources.Dependency {
	return nil
}

// IsOptional returns whether this source is optional.
// The provider source supplies the required core data.
func (s *Source) IsOptional() bool {
	return false
}
