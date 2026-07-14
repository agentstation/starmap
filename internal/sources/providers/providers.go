// Package providers implements the provider-backed catalog source.
package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/auth/cloudchains"
	"github.com/agentstation/starmap/internal/providers/registry"
	"github.com/agentstation/starmap/internal/sources/nativeproviders"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	providerModelIDField      = "model.id"
	validationRequiredMessage = "is required"
)

// ClientFactory creates a client for a provider.
type ClientFactory = sources.ProviderClientFactory

// SourceOption configures the provider source.
type SourceOption func(*sourceOptions)

type sourceOptions struct {
	clientFactory ClientFactory
	resolver      *acquisition.Resolver
}

// NewConfigured returns every executable provider acquisition source selected
// from the normalized provider catalog. Reusable HTTP connectors remain behind
// the provider fan-out source; official-SDK topologies remain independent
// sources so their observation scope and completeness are not collapsed.
func NewConfigured(providerConfigurations catalogs.ProvidersReader, opts ...SourceOption) ([]sources.Source, error) {
	if providerConfigurations == nil {
		return nil, nil
	}
	settings := sourceOptions{}
	for _, option := range opts {
		option(&settings)
	}
	resolver := settings.resolver
	if resolver == nil {
		cloud, err := cloudchains.NewRegistry()
		if err != nil {
			return nil, pkgerrors.WrapResource("initialize", "cloud credential chain registry", "", err)
		}
		resolver = acquisition.NewResolver(acquisition.WithAuthResolver(auth.NewResolver(auth.WithCloudChainRegistry(cloud))))
	}
	configured := make([]sources.Source, 0)
	fetcherOptions := []sources.ProviderOption{sources.WithProviderSourceResolver(resolver)}
	if settings.clientFactory != nil {
		fetcherOptions = append(fetcherOptions, sources.WithProviderClientFactory(settings.clientFactory))
	}
	fetcher := sources.NewProviderFetcher(providerConfigurations, fetcherOptions...)
	for _, provider := range providerConfigurations.List() {
		if provider.Catalog == nil {
			continue
		}
		for _, source := range provider.Catalog.Sources {
			if registry.Supports(source.Endpoint.Type) {
				configured = append(configured, &configuredConnectorSource{
					provider: catalogs.DeepCopyProvider(provider), sourceID: source.ID,
					optional: source.Optional, fetcher: fetcher,
				})
			}
		}
	}
	native, err := nativeproviders.New(providerConfigurations, nativeproviders.WithResolver(resolver))
	if err != nil {
		return nil, err
	}
	return append(configured, native...), nil
}

type configuredConnectorSource struct {
	provider catalogs.Provider
	sourceID string
	optional bool
	fetcher  *sources.ProviderFetcher
}

func (source *configuredConnectorSource) ID() sources.ID { return sources.ProvidersID }

func (source *configuredConnectorSource) Name() string {
	return fmt.Sprintf("%s / %s", source.provider.Name, source.sourceID)
}

func (source *configuredConnectorSource) ProviderID() catalogs.ProviderID { return source.provider.ID }

func (source *configuredConnectorSource) Observe(ctx context.Context, _ ...sources.Option) (sources.Observation, error) {
	builder := catalogs.NewEmpty()
	provider := catalogs.DeepCopyProvider(source.provider)
	provider.Models = nil
	if err := builder.SetProvider(provider); err != nil {
		return sources.Observation{}, pkgerrors.WrapResource("configure", "provider observation", string(provider.ID), err)
	}
	result, fetchErr := source.fetcher.FetchSource(ctx, &provider, source.sourceID)
	config := configuredProviderSource(provider, source.sourceID)
	scope := catalogmeta.ObservationScope(config.ObservationScope.Scope(false))
	records := sources.ObservationRecordCounts{}
	issues := make([]sources.ObservationIssue, 0, 1)
	coverage := catalogmeta.ProviderCoverage{Expected: 1}
	status := sources.ObservationStatusSucceeded
	completeness := sources.ObservationCompletenessComplete
	acquisition := catalogmeta.AcquisitionProvenance{
		ProviderID: string(provider.ID), SourceID: source.sourceID,
		Scope: scope, Topology: catalogmeta.AcquisitionTopology(normalizedSourceTopology(config.Topology)),
	}
	if fetchErr != nil {
		status = sources.ObservationStatusDegraded
		completeness = sources.ObservationCompletenessPartial
		issues = append(issues, classifyProviderSourceFetchIssue(provider.ID, source.sourceID, fetchErr))
	} else {
		scope = catalogmeta.ObservationScope(result.Scope)
		acquisition.Scope = scope
		acquisition.AuthMethod = string(result.AuthMethod)
		if acquisition.AuthMethod == "" {
			acquisition.AuthMethod = "none"
		}
		accepted, rejected, modelIssues := quarantineProviderModels(provider.ID, result.Models)
		records.Accepted = len(accepted)
		records.Rejected = rejected
		issues = append(issues, modelIssues...)
		if len(modelIssues) > 0 {
			status = sources.ObservationStatusDegraded
			completeness = sources.ObservationCompletenessPartial
		}
		if len(accepted) > 0 {
			provider.Models = make(map[string]*catalogs.Model, len(accepted))
			for _, model := range accepted {
				provider.Models[model.ID] = model
			}
			if err := builder.SetProvider(provider); err != nil {
				return sources.Observation{}, pkgerrors.WrapResource("populate", "provider observation", string(provider.ID), err)
			}
		}
		coverage.Observed = 1
	}
	catalog, err := builder.Build()
	if err != nil {
		return sources.Observation{}, pkgerrors.WrapResource("publish", "provider source observation", string(provider.ID)+"/"+source.sourceID, err)
	}
	return sources.NewObservation(source.ID(), catalog, sources.ObservationMetadata{
		ObservedAt: time.Now().UTC(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: completeness, Status: status, Records: records,
		Scope: scope, Kind: catalogmeta.SourceKindDirectInventory, Coverage: coverage,
		Acquisitions: []catalogmeta.AcquisitionProvenance{acquisition}, Issues: issues,
	})
}

func normalizedSourceTopology(topology catalogs.ProviderSourceTopology) catalogs.ProviderSourceTopology {
	if topology == "" {
		return catalogs.ProviderSourceTopologySingleEndpoint
	}
	return topology
}

func configuredProviderSource(provider catalogs.Provider, sourceID string) catalogs.ProviderSource {
	if provider.Catalog != nil {
		for _, source := range provider.Catalog.Sources {
			if source.ID == sourceID {
				return source
			}
		}
	}
	return catalogs.ProviderSource{}
}

func (source *configuredConnectorSource) Cleanup() error { return nil }

func (source *configuredConnectorSource) Dependencies() []sources.Dependency { return nil }

func (source *configuredConnectorSource) IsOptional() bool { return source.optional }

// WithClientFactory configures the factory used to create provider clients.
func WithClientFactory(factory ClientFactory) SourceOption {
	return func(s *sourceOptions) {
		s.clientFactory = factory
	}
}

// WithSourceResolver supplies one immutable request-scoped resolution context.
func WithSourceResolver(resolver *acquisition.Resolver) SourceOption {
	return func(options *sourceOptions) { options.resolver = resolver }
}

func providerIssue(providerID catalogs.ProviderID, code sources.ObservationIssueCode, err error) sources.ObservationIssue {
	return sources.ObservationIssue{
		Scope:   sources.ObservationIssueScopeProvider,
		Code:    code,
		Subject: string(providerID),
		Message: pkgerrors.SafeSummary(err),
	}
}

func classifyProviderSourceFetchIssue(providerID catalogs.ProviderID, sourceID string, err error) sources.ObservationIssue {
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
	issue := providerIssue(providerID, code, err)
	if sourceID != "" {
		issue.Subject += "/" + sourceID
	}
	return issue
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
			err = &pkgerrors.ValidationError{Field: providerModelIDField, Value: model.ID, Message: validationRequiredMessage}
		case modelID != strings.TrimSpace(modelID):
			err = &pkgerrors.ValidationError{Field: providerModelIDField, Value: model.ID, Message: "must not contain leading or trailing whitespace"}
		case strings.IndexFunc(modelID, unicode.IsControl) >= 0:
			err = &pkgerrors.ValidationError{Field: providerModelIDField, Value: model.ID, Message: "must not contain control characters"}
		case strings.TrimSpace(model.Name) == "":
			subject = string(providerID) + "/" + modelID
			err = &pkgerrors.ValidationError{Field: "model.name", Value: model.Name, Message: validationRequiredMessage}
		case strings.IndexFunc(model.Name, unicode.IsControl) >= 0:
			subject = string(providerID) + "/" + modelID
			err = &pkgerrors.ValidationError{Field: "model.name", Value: model.Name, Message: "must not contain control characters"}
		case hasProviderModelID(seen, modelID):
			subject = string(providerID) + "/" + modelID
			err = &pkgerrors.ValidationError{Field: providerModelIDField, Value: modelID, Message: "must be unique within provider observation"}
		}
		if err != nil {
			issues = append(issues, sources.ObservationIssue{
				Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord,
				Subject: subject, Message: pkgerrors.SafeSummary(err),
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
