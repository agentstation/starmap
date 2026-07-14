// Package nativeproviders resolves configured logical sources backed by official cloud SDKs.
package nativeproviders

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/auth/cloudchains"
	"github.com/agentstation/starmap/internal/providers/azurefoundry"
	"github.com/agentstation/starmap/internal/providers/bedrock"
	"github.com/agentstation/starmap/internal/providers/databricks"
	"github.com/agentstation/starmap/internal/providers/oci"
	"github.com/agentstation/starmap/internal/providers/watsonx"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type options struct{ resolver *acquisition.Resolver }

// Option configures native provider-source construction.
type Option func(*options)

// WithResolver supplies the request-scoped resolver shared by one configured
// source set. It must not be mutated after construction.
func WithResolver(resolver *acquisition.Resolver) Option {
	return func(options *options) { options.resolver = resolver }
}

// New returns configured provider-native SDK sources. Credential resolution
// remains request-scoped and happens when each source is observed.
func New(providers catalogs.ProvidersReader, values ...Option) ([]sources.Source, error) {
	if providers == nil {
		return nil, nil
	}
	settings := options{}
	for _, value := range values {
		value(&settings)
	}
	resolver := settings.resolver
	if resolver == nil {
		cloud, err := cloudchains.NewRegistry()
		if err != nil {
			return nil, errors.WrapResource("initialize", "cloud credential chain registry", "", err)
		}
		resolver = acquisition.NewResolver(acquisition.WithAuthResolver(auth.NewResolver(auth.WithCloudChainRegistry(cloud))))
	}
	result := make([]sources.Source, 0, 3)
	for _, provider := range providers.List() {
		if provider.Catalog == nil {
			continue
		}
		for _, source := range provider.Catalog.Sources {
			id, name, supported := sourceIdentity(source.Endpoint.Type)
			if !supported {
				continue
			}
			result = append(result, &configuredSource{
				provider: catalogs.DeepCopyProvider(provider), sourceID: source.ID,
				id: id, name: name, optional: source.Optional, resolver: resolver,
			})
		}
	}
	return result, nil
}

func sourceIdentity(endpoint catalogs.EndpointType) (sources.ID, string, bool) {
	switch endpoint {
	case catalogs.EndpointTypeBedrock:
		return sources.AmazonBedrockID, "Amazon Bedrock", true
	case catalogs.EndpointTypeAzureOpenAI:
		return sources.MicrosoftFoundryID, "Microsoft Foundry and Azure OpenAI", true
	case catalogs.EndpointTypeOCI:
		return sources.OCIGenerativeAIID, "Oracle OCI Generative AI", true
	case catalogs.EndpointTypeDatabricksWorkspace:
		return sources.DatabricksWorkspaceID, "Databricks workspace serving endpoints", true
	case catalogs.EndpointTypeWatsonxDeployments:
		return sources.WatsonxDeploymentsID, "watsonx project or space deployments", true
	default:
		return "", "", false
	}
}

// Supports reports whether an endpoint is implemented by an official-SDK source adapter.
func Supports(endpoint catalogs.EndpointType) bool {
	_, _, supported := sourceIdentity(endpoint)
	return supported
}

type configuredSource struct {
	provider catalogs.Provider
	sourceID string
	id       sources.ID
	name     string
	optional bool
	resolver *acquisition.Resolver
}

func (source *configuredSource) ID() sources.ID { return source.id }

func (source *configuredSource) Name() string { return source.name }

func (source *configuredSource) ProviderID() catalogs.ProviderID { return source.provider.ID }

func (source *configuredSource) Observe(ctx context.Context, options ...sources.Option) (sources.Observation, error) {
	resolved, err := source.resolver.Resolve(ctx, &source.provider, source.sourceID)
	if err != nil {
		return source.unavailableObservation(err)
	}
	var delegate sources.Source
	var observation sources.Observation
	switch resolved.Config().Endpoint.Type {
	case catalogs.EndpointTypeBedrock:
		delegate, err = bedrock.NewResolvedSource(resolved)
	case catalogs.EndpointTypeAzureOpenAI:
		delegate, err = azurefoundry.NewResolvedSource(resolved)
	case catalogs.EndpointTypeOCI:
		delegate, err = oci.NewResolvedSource(resolved)
	case catalogs.EndpointTypeDatabricksWorkspace:
		observation, err = observeDatabricksWorkspace(ctx, resolved)
	case catalogs.EndpointTypeWatsonxDeployments:
		observation, err = observeWatsonxDeployments(ctx, resolved)
	default:
		err = &errors.ValidationError{Field: "provider.catalog.source.endpoint.type", Value: resolved.Config().Endpoint.Type, Message: "is not a native SDK source"}
	}
	if err != nil {
		return source.unavailableObservation(err)
	}
	if delegate != nil {
		observation, err = delegate.Observe(ctx, options...)
		if err != nil {
			return sources.Observation{}, err
		}
	}
	authMethod := string(resolved.Auth().Method())
	if authMethod == "" {
		authMethod = "none"
	}
	config := resolved.Config()
	configuredScope := catalogmeta.ObservationScope(config.ObservationScope.Scope(!resolved.Auth().Anonymous()))
	topology := config.Topology
	if topology == "" {
		topology = catalogs.ProviderSourceTopologySingleEndpoint
	}
	return sources.NewObservation(source.id, observation.Catalog, sources.ObservationMetadata{
		ObservedAt: observation.ObservedAt, Revision: observation.Revision,
		Completeness: observation.Completeness, Status: observation.Status,
		Records: observation.Records, Issues: append([]sources.ObservationIssue(nil), observation.Issues...),
		Scope: configuredScope, Kind: observation.Metrics.Kind,
		Coverage: observation.Metrics.ProviderCoverage, PricingObservedAt: observation.Metrics.PricingObservedAt,
		Acquisitions: []catalogmeta.AcquisitionProvenance{{
			ProviderID: string(source.provider.ID), SourceID: source.sourceID,
			AuthMethod: authMethod, Scope: configuredScope,
			Topology: catalogmeta.AcquisitionTopology(topology),
		}},
	})
}

func observeDatabricksWorkspace(ctx context.Context, resolved acquisition.Source) (sources.Observation, error) {
	token, found := resolved.Auth().APIKey()
	if !found {
		return sources.Observation{}, &errors.AuthenticationError{Provider: string(catalogs.ProviderIDDatabricks), Method: "api_key", Message: "workspace token is required", Err: errors.ErrAPIKeyRequired}
	}
	workspaceID, found := resolved.Binding("workspace_id")
	if !found {
		return sources.Observation{}, &errors.ConfigError{Component: "databricks/workspace", Message: "workspace_id binding is required", Err: errors.ErrNotFound}
	}
	provider := resolved.Provider()
	result, err := databricks.FetchWorkspace(ctx, databricks.WorkspaceConfig{
		Host: resolved.EndpointURL(), Token: token, WorkspaceID: workspaceID,
		DefinitionByEntity: definitionMap(provider),
	})
	if err != nil {
		return sources.Observation{}, err
	}
	catalog, err := result.Catalog(provider)
	if err != nil {
		return sources.Observation{}, err
	}
	return successfulContextualObservation(sources.DatabricksWorkspaceID, catalog, len(result.Offerings))
}

func observeWatsonxDeployments(ctx context.Context, resolved acquisition.Source) (sources.Observation, error) {
	token, found := resolved.Auth().APIKey()
	if !found {
		return sources.Observation{}, &errors.AuthenticationError{Provider: string(catalogs.ProviderIDWatsonx), Method: "api_key", Message: "deployment token is required", Err: errors.ErrAPIKeyRequired}
	}
	projectID, _ := resolved.Option("project_id")
	spaceID, _ := resolved.Option("space_id")
	if (projectID == "") == (spaceID == "") {
		return sources.Observation{}, &errors.ConfigError{Component: "watsonx/deployments", Message: "exactly one project_id or space_id option is required", Err: errors.ErrNotFound}
	}
	provider := resolved.Provider()
	config := watsonx.DeploymentConfig{
		BaseURL: resolved.EndpointURL(), Token: token, ProjectID: projectID, SpaceID: spaceID,
		DefinitionByAsset: definitionMap(provider),
	}
	if region, ok := resolved.Binding("region"); ok {
		value := catalogs.CloudRegion{ID: region}
		config.Region = &value
	}
	result, err := watsonx.FetchDeployments(ctx, config)
	if err != nil {
		return sources.Observation{}, err
	}
	catalog, err := result.Catalog(provider)
	if err != nil {
		return sources.Observation{}, err
	}
	return successfulContextualObservation(sources.WatsonxDeploymentsID, catalog, len(result.Offerings))
}

func successfulContextualObservation(id sources.ID, catalog *catalogs.Catalog, accepted int) (sources.Observation, error) {
	return sources.NewObservation(id, catalog, sources.ObservationMetadata{
		ObservedAt: time.Now().UTC(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Records: sources.ObservationRecordCounts{Accepted: accepted},
		Scope:   catalogmeta.ObservationScopeCredentialScoped, Kind: catalogmeta.SourceKindDirectInventory,
		Coverage: catalogmeta.ProviderCoverage{Expected: 1, Observed: 1},
	})
}

func definitionMap(provider catalogs.Provider) map[string]catalogs.ModelDefinitionID {
	result := make(map[string]catalogs.ModelDefinitionID, len(provider.Models))
	for id, model := range provider.Models {
		definitionID := catalogs.ModelDefinitionID(id)
		if model != nil && model.DefinitionID != "" {
			definitionID = model.DefinitionID
		}
		result[id] = definitionID
	}
	return result
}

func (source *configuredSource) unavailableObservation(cause error) (sources.Observation, error) {
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		return sources.Observation{}, err
	}
	code := sources.ObservationIssueCodeFetchFailed
	var authenticationError *errors.AuthenticationError
	if stderrors.As(cause, &authenticationError) || stderrors.Is(cause, errors.ErrNotFound) || stderrors.Is(cause, errors.ErrAPIKeyRequired) {
		code = sources.ObservationIssueCodeMissingCredentials
	}
	config := sourceConfig(source.provider, source.sourceID)
	topology := config.Topology
	if topology == "" {
		topology = catalogs.ProviderSourceTopologySingleEndpoint
	}
	kind := catalogmeta.SourceKindDirectInventory
	if config.Endpoint.Type == catalogs.EndpointTypeBedrock || config.Endpoint.Type == catalogs.EndpointTypeAzureOpenAI || config.Endpoint.Type == catalogs.EndpointTypeOCI {
		kind = catalogmeta.SourceKindRegionalSweep
	}
	return sources.NewObservation(source.id, catalog, sources.ObservationMetadata{
		ObservedAt: time.Now().UTC(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Scope: catalogmeta.ObservationScopeGlobalPublic, Kind: kind,
		Coverage: catalogmeta.ProviderCoverage{Expected: 1},
		Acquisitions: []catalogmeta.AcquisitionProvenance{{
			ProviderID: string(source.provider.ID), SourceID: source.sourceID,
			Scope:    catalogmeta.ObservationScopeGlobalPublic,
			Topology: catalogmeta.AcquisitionTopology(topology),
		}},
		Issues: []sources.ObservationIssue{{
			Scope: sources.ObservationIssueScopeSource, Code: code,
			Subject: string(source.provider.ID) + "/" + source.sourceID, Message: errors.SafeSummary(cause),
		}},
	})
}

func sourceConfig(provider catalogs.Provider, sourceID string) catalogs.ProviderSource {
	if provider.Catalog != nil {
		for _, source := range provider.Catalog.Sources {
			if source.ID == sourceID {
				return source
			}
		}
	}
	return catalogs.ProviderSource{}
}

func (source *configuredSource) Cleanup() error { return nil }

func (source *configuredSource) Dependencies() []sources.Dependency { return nil }

func (source *configuredSource) IsOptional() bool { return source.optional }
