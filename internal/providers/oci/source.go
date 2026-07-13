// Package oci discovers OCI Generative AI regional models and isolates tenancy endpoints.
package oci

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/providerdata"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ProviderID is the canonical OCI Generative AI service channel.
const ProviderID = catalogs.ProviderIDOCI

const (
	defaultMaxPages   = 32
	defaultMaxRecords = 10000
)

// Config identifies one credential-scoped OCI region and realm.
type Config struct {
	Region        string
	Realm         string
	CompartmentID string
}

// Model is the OCI ListModels subset admitted by Starmap.
type Model struct {
	ID                   string
	DisplayName          string
	Vendor               string
	Version              string
	Capabilities         []string
	LifecycleState       string
	Type                 string
	BaseModelID          string
	TimeDeprecated       *time.Time
	TimeOnDemandRetired  *time.Time
	TimeDedicatedRetired *time.Time
}

// Endpoint is the private OCI ListEndpoints subset admitted to customer inventory.
type Endpoint struct {
	ID                   string
	ModelID              string
	DedicatedAIClusterID string
	DisplayName          string
	LifecycleState       string
	PrivateEndpointID    string
}

// API is the bounded native OCI control-plane surface used by Source.
type API interface {
	ListModels(context.Context, string) (sources.Page[Model], error)
	ListEndpoints(context.Context, string) (sources.Page[Endpoint], error)
}

// Result separates sanitized regional offerings from private tenancy inventory.
type Result struct {
	Definitions       []catalogs.ModelDefinition
	Offerings         []catalogs.ProviderOffering
	CustomerInventory []catalogs.CustomerInventory
}

// Source observes one explicitly configured OCI region.
type Source struct {
	config     Config
	client     API
	clientFunc func(context.Context, Config) (API, error)
	retry      sources.ProviderRetryPolicy
	pagination sources.PaginationPolicy
	now        func() time.Time
}

var _ sources.Source = (*Source)(nil)

// NewSource constructs an injected OCI regional source.
func NewSource(config Config, client API) (*Source, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, &errors.ValidationError{Field: "oci.client", Message: "is required"}
	}
	return newSource(config, client), nil
}

func newSource(config Config, client API) *Source {
	return &Source{config: config, client: client, retry: sources.DefaultProviderRetryPolicy(), pagination: sources.PaginationPolicy{MaxPages: defaultMaxPages, MaxRecords: defaultMaxRecords}, now: func() time.Time { return time.Now().UTC() }}
}

// NewCommercialSource constructs the optional default OC1 source from OCI configuration.
func NewCommercialSource() *Source {
	source := newSource(Config{Region: os.Getenv("OCI_REGION"), Realm: "oc1", CompartmentID: os.Getenv("OCI_COMPARTMENT_ID")}, nil)
	source.clientFunc = newSDKClient
	return source
}

// ID returns the stable OCI source identity.
func (s *Source) ID() sources.ID { return sources.OCIGenerativeAIID }

// Name returns the operator-facing source name.
func (s *Source) Name() string { return "Oracle OCI Generative AI" }

// Observe emits only sanitized base-model regional offerings.
func (s *Source) Observe(ctx context.Context, _ ...sources.Option) (sources.Observation, error) {
	result, fetchErr := s.Fetch(ctx, false)
	if fetchErr != nil {
		catalog, err := emptyPublicCatalog()
		if err != nil {
			return sources.Observation{}, err
		}
		code := sources.ObservationIssueCodeFetchFailed
		var authErr *errors.AuthenticationError
		if stderrors.As(fetchErr, &authErr) {
			code = sources.ObservationIssueCodeMissingCredentials
		}
		return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
			ObservedAt: s.now(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
			Scope: catalogmeta.ObservationScopeRegionalPublic, Kind: catalogmeta.SourceKindRegionalSweep,
			Coverage: catalogmeta.ProviderCoverage{Expected: 1},
			Issues:   []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: code, Subject: string(ProviderID), Message: fetchErr.Error()}},
		})
	}
	catalog, err := result.PublicCatalog()
	if err != nil {
		return sources.Observation{}, err
	}
	return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
		ObservedAt: s.now(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Records: sources.ObservationRecordCounts{Accepted: len(result.Offerings)}, Scope: catalogmeta.ObservationScopeRegionalPublic,
		Kind: catalogmeta.SourceKindRegionalSweep, Coverage: catalogmeta.ProviderCoverage{Expected: 1, Observed: 1},
	})
}

// Cleanup releases source resources. SDK clients are request safe.
func (s *Source) Cleanup() error { return nil }

// Dependencies reports no external executable dependency.
func (s *Source) Dependencies() []sources.Dependency { return nil }

// IsOptional keeps credential-free catalog generation operational.
func (s *Source) IsOptional() bool { return true }

// Fetch reads regional models and optionally private dedicated endpoints.
func (s *Source) Fetch(ctx context.Context, includeCustomerInventory bool) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateConfig(s.config); err != nil {
		return Result{}, &errors.AuthenticationError{Provider: string(ProviderID), Method: "oci_default_configuration", Message: "OCI regional discovery is not configured", Err: err}
	}
	client := s.client
	if client == nil {
		var err error
		client, err = s.clientFunc(ctx, s.config)
		if err != nil {
			return Result{}, err
		}
	}
	models, err := s.collectModels(ctx, client)
	if err != nil {
		return Result{}, err
	}
	result, index, err := recordsFromModels(s.config, s.now(), models)
	if err != nil {
		return Result{}, err
	}
	if includeCustomerInventory {
		endpoints, listErr := s.collectEndpoints(ctx, client)
		if listErr != nil {
			return Result{}, listErr
		}
		inventory, convertErr := customerInventory(s.config, s.now(), endpoints, index)
		if convertErr != nil {
			return Result{}, convertErr
		}
		result.CustomerInventory = []catalogs.CustomerInventory{inventory}
	}
	return result, nil
}

func validateConfig(config Config) error {
	if strings.TrimSpace(config.Region) == "" || strings.TrimSpace(config.Realm) == "" || strings.TrimSpace(config.CompartmentID) == "" {
		return &errors.ValidationError{Field: "oci.config", Message: "region, realm, and compartment are required"}
	}
	return nil
}

func (s *Source) collectModels(ctx context.Context, client API) ([]Model, error) {
	return sources.CollectPages(ctx, s.pagination, func(pageCtx context.Context, cursor string) (page sources.Page[Model], err error) {
		err = sources.RetryProviderCall(pageCtx, s.retry, func(callCtx context.Context) (sources.RetryHint, error) {
			page, err = client.ListModels(callCtx, cursor)
			return sources.RetryHint{}, err
		})
		return page, err
	})
}

func (s *Source) collectEndpoints(ctx context.Context, client API) ([]Endpoint, error) {
	return sources.CollectPages(ctx, s.pagination, func(pageCtx context.Context, cursor string) (page sources.Page[Endpoint], err error) {
		err = sources.RetryProviderCall(pageCtx, s.retry, func(callCtx context.Context) (sources.RetryHint, error) {
			page, err = client.ListEndpoints(callCtx, cursor)
			return sources.RetryHint{}, err
		})
		return page, err
	})
}

func recordsFromModels(config Config, now time.Time, models []Model) (Result, map[string]catalogs.ModelDefinitionID, error) {
	pricingCatalog, err := providerdata.LoadPricingCatalog(ProviderID)
	if err != nil {
		return Result{}, nil, errors.WrapResource("load", "OCI pricing catalog", string(ProviderID), err)
	}
	result := Result{}
	index := make(map[string]catalogs.ModelDefinitionID, len(models))
	seen := make(map[catalogs.OfferingKey]struct{})
	for _, model := range models {
		if strings.TrimSpace(model.ID) == "" || strings.TrimSpace(model.Vendor) == "" || strings.TrimSpace(model.Type) == "" {
			return Result{}, nil, &errors.ValidationError{Field: "oci.model", Value: model.ID, Message: "id, vendor, and type are required"}
		}
		definitionID := canonicalDefinitionID(model)
		index[model.ID] = definitionID
		if !strings.EqualFold(model.Type, "BASE") {
			continue
		}
		definition := catalogs.ModelDefinition{ID: definitionID, Name: firstNonempty(model.DisplayName, model.ID), AuthorIDs: []catalogs.AuthorID{canonicalAuthor(model.Vendor)}}
		apis := invocationAPIs(model.ID, model.Capabilities)
		access := catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityDiscoverable, APIs: []catalogs.InvocationAPI{}}
		if len(apis) > 0 {
			access.Routability = catalogs.OfferingRoutabilityRoutable
			access.APIs = apis
		}
		lifecycle, availability := modelLifecycle(model, now)
		offering := catalogs.ProviderOffering{
			ProviderID: ProviderID, ProviderModelID: catalogs.ProviderModelID(model.ID), DefinitionID: definitionID,
			Availability: availability, Access: access, Regions: []catalogs.CloudRegion{{ID: config.Region, Realm: config.Realm}},
			Deployment: catalogs.ProviderDeployment{Type: "on-demand", Tier: "pay-as-you-go"}, Lifecycle: lifecycle,
			Endpoint: endpointForModel(config.Region, model.ID),
		}
		if facts, found := pricingCatalog.Models[model.ID]; found {
			offering.Pricing = facts.Pricing
		}
		if err := definition.Validate(); err != nil {
			return Result{}, nil, err
		}
		if err := offering.Validate(); err != nil {
			return Result{}, nil, err
		}
		if _, found := seen[offering.Key()]; found {
			return Result{}, nil, &errors.ConflictError{Resource: "OCI model offering", Actual: fmt.Sprint(offering.Key()), Message: "duplicate model ID"}
		}
		seen[offering.Key()] = struct{}{}
		result.Definitions = append(result.Definitions, definition)
		result.Offerings = append(result.Offerings, offering)
	}
	slices.SortFunc(result.Definitions, func(left, right catalogs.ModelDefinition) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	slices.SortFunc(result.Offerings, func(left, right catalogs.ProviderOffering) int {
		return strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID))
	})
	return result, index, nil
}

func customerInventory(config Config, observedAt time.Time, endpoints []Endpoint, index map[string]catalogs.ModelDefinitionID) (catalogs.CustomerInventory, error) {
	inventory := catalogs.CustomerInventory{ProviderID: ProviderID, Scope: catalogs.CustomerScope{AccountID: config.CompartmentID}, ObservedAt: observedAt}
	for _, endpoint := range endpoints {
		definitionID, found := index[endpoint.ModelID]
		if !found {
			return catalogs.CustomerInventory{}, &errors.NotFoundError{Resource: "OCI endpoint model definition", ID: endpoint.ModelID}
		}
		if endpoint.ID == "" || endpoint.DedicatedAIClusterID == "" {
			return catalogs.CustomerInventory{}, &errors.ValidationError{Field: "oci.endpoint", Value: endpoint.ID, Message: "endpoint and dedicated cluster IDs are required"}
		}
		aliases := []string{}
		if endpoint.DisplayName != "" {
			aliases = append(aliases, endpoint.DisplayName)
		}
		if endpoint.PrivateEndpointID != "" {
			aliases = append(aliases, "private")
		}
		inventory.Deployments = append(inventory.Deployments, catalogs.CustomerDeployment{
			ID: endpoint.ID, DefinitionID: definitionID, ProviderModelID: catalogs.ProviderModelID(endpoint.ModelID), Region: &catalogs.CloudRegion{ID: config.Region, Realm: config.Realm},
			Deployment: catalogs.ProviderDeployment{Type: "dedicated-ai-cluster", Tier: endpoint.DedicatedAIClusterID}, Aliases: aliases,
		})
	}
	if err := inventory.Validate(); err != nil {
		return catalogs.CustomerInventory{}, err
	}
	return inventory, nil
}

// PublicCatalog materializes only globally publishable OCI base-model records.
func (r Result) PublicCatalog() (*catalogs.Catalog, error) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: ProviderID, Name: "Oracle OCI Generative AI"}); err != nil {
		return nil, err
	}
	authors := make(map[catalogs.AuthorID]struct{})
	for _, definition := range r.Definitions {
		if err := builder.SetDefinition(definition); err != nil {
			return nil, err
		}
		for _, author := range definition.AuthorIDs {
			authors[author] = struct{}{}
		}
	}
	for author := range authors {
		if err := builder.SetAuthor(catalogs.Author{ID: author, Name: string(author)}); err != nil {
			return nil, err
		}
	}
	for _, offering := range r.Offerings {
		if err := builder.SetOffering(offering); err != nil {
			return nil, err
		}
	}
	return builder.Build()
}

func emptyPublicCatalog() (*catalogs.Catalog, error) { return catalogs.NewEmpty().Build() }

func canonicalDefinitionID(model Model) catalogs.ModelDefinitionID {
	if !strings.EqualFold(model.Type, "BASE") {
		return catalogs.ModelDefinitionID("oci-private/" + strings.ReplaceAll(model.ID, ".", "-"))
	}
	id := strings.TrimPrefix(model.ID, strings.ToLower(model.Vendor)+".")
	return catalogs.ModelDefinitionID(string(canonicalAuthor(model.Vendor)) + "/" + strings.ReplaceAll(id, ".", "-"))
}

func canonicalAuthor(value string) catalogs.AuthorID {
	slug := strings.ToLower(strings.NewReplacer(" ", "-", "_", "-", ".", "-").Replace(strings.TrimSpace(value)))
	switch slug {
	case "meta", "meta-llama":
		return catalogs.AuthorIDMeta
	case "cohere":
		return catalogs.AuthorIDCohere
	case "openai":
		return catalogs.AuthorIDOpenAI
	case "xai", "x-ai":
		return catalogs.AuthorIDXAI
	case "google":
		return catalogs.AuthorIDGoogle
	default:
		return catalogs.AuthorID(slug)
	}
}

func invocationAPIs(modelID string, capabilities []string) []catalogs.InvocationAPI {
	if supportsOpenAIResponses(modelID) {
		return []catalogs.InvocationAPI{catalogs.InvocationAPIResponses}
	}
	set := make(map[catalogs.InvocationAPI]struct{})
	for _, capability := range capabilities {
		switch strings.ToUpper(capability) {
		case "TEXT_GENERATION", "TEXT_SUMMARIZATION", "CHAT", "IMAGE_TEXT_TO_TEXT":
			set[catalogs.InvocationAPIOCIInference] = struct{}{}
		case "TEXT_EMBEDDINGS":
			set[catalogs.InvocationAPIEmbeddings] = struct{}{}
		case "TEXT_RERANK":
			set[catalogs.InvocationAPIRerank] = struct{}{}
		}
	}
	result := make([]catalogs.InvocationAPI, 0, len(set))
	for api := range set {
		result = append(result, api)
	}
	slices.Sort(result)
	return result
}

func supportsOpenAIResponses(modelID string) bool {
	switch modelID {
	case "google.gemini-2.5-pro", "google.gemini-2.5-flash", "google.gemini-2.5-flash-lite",
		"openai.gpt-oss-120b", "openai.gpt-oss-20b", "xai.grok-4.3":
		return true
	default:
		return false
	}
}

func endpointForModel(region, modelID string) catalogs.ProviderOfferingEndpoint {
	baseURL := "https://inference.generativeai." + region + ".oci.oraclecloud.com"
	if supportsOpenAIResponses(modelID) {
		baseURL += "/openai/v1"
	}
	return catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOCI, BaseURL: baseURL}
}

func modelLifecycle(model Model, now time.Time) (catalogs.OfferingLifecycle, catalogs.OfferingAvailability) {
	if !strings.EqualFold(model.LifecycleState, "ACTIVE") || model.TimeDeprecated != nil && !model.TimeDeprecated.After(now) || model.TimeOnDemandRetired != nil && !model.TimeOnDemandRetired.After(now) {
		return catalogs.OfferingLifecycleDeprecated, catalogs.OfferingAvailabilityUnavailable
	}
	return catalogs.OfferingLifecycleActive, catalogs.OfferingAvailabilityAvailable
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
