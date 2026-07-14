// Package bedrock discovers Amazon Bedrock regional and contextual offerings.
package bedrock

import (
	"context"
	stderrors "errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsbedrock "github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/bedrock/types"
	"github.com/aws/smithy-go/transport/http"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ProviderID is the canonical Amazon Bedrock service-channel identity.
const ProviderID catalogs.ProviderID = "amazon-bedrock"

const (
	defaultMaxProfilePages = 32
	defaultMaxProfiles     = 10000
	modeRegional           = "regional"
	tierOnDemand           = "on_demand"
)

// API is the native Bedrock control-plane surface used by Source.
type API interface {
	ListFoundationModels(context.Context, *awsbedrock.ListFoundationModelsInput, ...func(*awsbedrock.Options)) (*awsbedrock.ListFoundationModelsOutput, error)
	ListInferenceProfiles(context.Context, *awsbedrock.ListInferenceProfilesInput, ...func(*awsbedrock.Options)) (*awsbedrock.ListInferenceProfilesOutput, error)
}

// ClientFactory creates a region-scoped native Bedrock client.
type ClientFactory func(context.Context, string) (API, error)

// Result contains canonical definitions and offerings acquired from Bedrock.
type Result struct {
	Definitions       []catalogs.ModelDefinition
	Offerings         []catalogs.ProviderOffering
	PricingObservedAt *time.Time
	PricingVersion    string
	PricingMatched    int
}

// Source performs a deterministic, bounded sweep of configured AWS regions.
type Source struct {
	regions    []string
	clients    ClientFactory
	retry      sources.ProviderRetryPolicy
	pagination sources.PaginationPolicy
	now        func() time.Time
	pricing    pricingFetcher
	configErr  error
}

var _ sources.Source = (*Source)(nil)

// NewSource constructs a Bedrock source. Regions are explicit because AWS model
// availability is regional and account region enablement is customer policy.
func NewSource(regions []string, clients ClientFactory) (*Source, error) {
	regions = normalizedRegions(regions)
	if len(regions) == 0 {
		return nil, &errors.ValidationError{Field: "bedrock.regions", Message: "at least one region is required"}
	}
	if clients == nil {
		return nil, &errors.ValidationError{Field: "bedrock.clients", Message: "resolved client factory is required"}
	}
	return newSource(regions, clients), nil
}

func newSource(regions []string, clients ClientFactory) *Source {
	return &Source{
		regions: regions, clients: clients, retry: sources.DefaultProviderRetryPolicy(),
		pagination: sources.PaginationPolicy{MaxPages: defaultMaxProfilePages, MaxRecords: defaultMaxProfiles},
		now:        func() time.Time { return time.Now().UTC() },
	}
}

type awsCloudSession interface{ Config() aws.Config }

// NewResolvedSource constructs a Bedrock source from one fully resolved logical source.
func NewResolvedSource(resolved acquisition.Source) (*Source, error) {
	if resolved.ProviderID() != ProviderID || resolved.Config().Endpoint.Type != catalogs.EndpointTypeBedrock {
		return nil, &errors.ValidationError{Field: "bedrock.source", Value: resolved.String(), Message: "must be an Amazon Bedrock source"}
	}
	session, ok := resolved.Auth().CloudSession().(awsCloudSession)
	if !ok {
		return nil, &errors.AuthenticationError{Provider: string(ProviderID), Method: "cloud_chain", Message: "resolved AWS SDK session is required", Err: errors.ErrAPIKeyRequired}
	}
	regions := slices.Clone(resolved.Config().Scopes["regions"].Values)
	if region, found := resolved.Binding("region"); found {
		regions = append(regions, region)
	}
	base := session.Config()
	source, err := NewSource(regions, func(_ context.Context, region string) (API, error) {
		configuration := base
		configuration.Region = region
		configuration.RetryMaxAttempts = 1
		return awsbedrock.NewFromConfig(configuration), nil
	})
	if err != nil {
		return nil, err
	}
	source.pricing = newHTTPPricingFetcher()
	return source, nil
}

// ID returns the stable native Bedrock source identity.
func (s *Source) ID() sources.ID { return sources.AmazonBedrockID }

// Name returns the operator-facing source name.
func (s *Source) Name() string { return "Amazon Bedrock" }

// Observe returns one credential-scoped canonical Bedrock observation.
func (s *Source) Observe(ctx context.Context, _ ...sources.Option) (sources.Observation, error) {
	result, fetchErr := s.Fetch(ctx)
	if fetchErr != nil {
		catalog, err := emptyCatalog()
		if err != nil {
			return sources.Observation{}, err
		}
		issueCode := sources.ObservationIssueCodeFetchFailed
		var authenticationErr *errors.AuthenticationError
		if stderrors.As(fetchErr, &authenticationErr) {
			issueCode = sources.ObservationIssueCodeMissingCredentials
		}
		return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
			ObservedAt: s.now(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
			Records: sources.ObservationRecordCounts{}, Scope: catalogmeta.ObservationScopeCredentialScoped, Kind: catalogmeta.SourceKindRegionalSweep,
			Coverage: catalogmeta.ProviderCoverage{Expected: len(s.regions)},
			Issues:   []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: issueCode, Subject: string(ProviderID), Message: errors.SafeSummary(fetchErr)}},
		})
	}
	catalog, err := result.Catalog()
	if err != nil {
		return sources.Observation{}, err
	}
	return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
		ObservedAt: s.now(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Records: sources.ObservationRecordCounts{Accepted: len(result.Offerings)},
		Scope:   catalogmeta.ObservationScopeCredentialScoped, Kind: catalogmeta.SourceKindRegionalSweep,
		Coverage:          catalogmeta.ProviderCoverage{Expected: len(s.regions), Observed: len(s.regions)},
		PricingObservedAt: result.PricingObservedAt,
	})
}

// Cleanup releases source resources. SDK clients are request-scoped.
func (s *Source) Cleanup() error { return nil }

// Dependencies reports no external executable dependency.
func (s *Source) Dependencies() []sources.Dependency { return nil }

// IsOptional keeps credential-free public generation operational.
func (s *Source) IsOptional() bool { return true }

// Catalog materializes only globally publishable Bedrock records.
func (r Result) Catalog() (*catalogs.Catalog, error) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: ProviderID, Name: "Amazon Bedrock"}); err != nil {
		return nil, err
	}
	authors := make(map[catalogs.AuthorID]struct{})
	for _, definition := range r.Definitions {
		if err := builder.SetDefinition(definition); err != nil {
			return nil, err
		}
		for _, authorID := range definition.AuthorIDs {
			authors[authorID] = struct{}{}
		}
	}
	for authorID := range authors {
		if err := builder.SetAuthor(catalogs.Author{ID: authorID, Name: string(authorID)}); err != nil {
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

func emptyCatalog() (*catalogs.Catalog, error) {
	return catalogs.NewEmpty().Build()
}

// Fetch discovers foundation models and both system and application inference profiles.
func (s *Source) Fetch(ctx context.Context) (Result, error) {
	if s.configErr != nil {
		return Result{}, errors.WrapResource("load", "Bedrock region catalog", string(ProviderID), s.configErr)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	definitions := make(map[catalogs.ModelDefinitionID]catalogs.ModelDefinition)
	offerings := make(map[catalogs.OfferingKey]catalogs.ProviderOffering)
	for _, region := range s.regions {
		client, err := s.clients(ctx, region)
		if err != nil {
			return Result{}, errors.WrapResource("create", "Bedrock client", region, err)
		}
		models, err := s.listFoundationModels(ctx, client, region)
		if err != nil {
			return Result{}, err
		}
		modelDefinitions := make(map[string]catalogs.ModelDefinitionID, len(models))
		for _, model := range models {
			definition, offering, convertErr := foundationRecords(region, model)
			if convertErr != nil {
				return Result{}, convertErr
			}
			if existing, found := definitions[definition.ID]; found && !reflect.DeepEqual(existing, definition) {
				return Result{}, &errors.ConflictError{Resource: "Bedrock model definition", Expected: fmt.Sprintf("%#v", existing), Actual: fmt.Sprintf("%#v", definition), Message: "regional observations disagree on provider-independent model facts"}
			}
			definitions[definition.ID] = definition
			modelDefinitions[aws.ToString(model.ModelId)] = definition.ID
			mergeOffering(offerings, offering)
		}
		systemProfiles, err := s.listProfiles(ctx, client, types.InferenceProfileTypeSystemDefined)
		if err != nil {
			return Result{}, errors.WrapResource("list", "Bedrock system inference profiles", region, err)
		}
		for _, profile := range systemProfiles {
			offering, convertErr := systemProfileOffering(region, profile, modelDefinitions)
			if convertErr != nil {
				return Result{}, convertErr
			}
			mergeOffering(offerings, offering)
		}
		applicationProfiles, listErr := s.listProfiles(ctx, client, types.InferenceProfileTypeApplication)
		if listErr != nil {
			return Result{}, errors.WrapResource("list", "Bedrock application inference profiles", region, listErr)
		}
		applicationOfferings, convertErr := applicationProfileOfferings(region, applicationProfiles, modelDefinitions)
		if convertErr != nil {
			return Result{}, convertErr
		}
		for _, offering := range applicationOfferings {
			mergeOffering(offerings, offering)
		}
	}
	result := Result{Definitions: definitionValues(definitions), Offerings: offeringValues(offerings)}
	if s.pricing != nil {
		prices, err := s.pricing.Fetch(ctx)
		if err != nil {
			return Result{}, errors.WrapResource("fetch", "Bedrock pricing", PriceListURL, err)
		}
		result.PricingObservedAt = &prices.PublishedAt
		result.PricingVersion = prices.Version
		result.PricingMatched = applyPricing(&result, prices)
	}
	return result, nil
}

func (s *Source) listFoundationModels(ctx context.Context, client API, region string) ([]types.FoundationModelSummary, error) {
	var output *awsbedrock.ListFoundationModelsOutput
	err := sources.RetryProviderCall(ctx, s.retry, func(callCtx context.Context) (sources.RetryHint, error) {
		var callErr error
		output, callErr = client.ListFoundationModels(callCtx, &awsbedrock.ListFoundationModelsInput{})
		return retryHint(callErr), callErr
	})
	if err != nil {
		return nil, errors.WrapResource("list", "Bedrock foundation models", region, err)
	}
	return output.ModelSummaries, nil
}

func (s *Source) listProfiles(ctx context.Context, client API, profileType types.InferenceProfileType) ([]types.InferenceProfileSummary, error) {
	return sources.CollectPages(ctx, s.pagination, func(pageCtx context.Context, cursor string) (sources.Page[types.InferenceProfileSummary], error) {
		input := &awsbedrock.ListInferenceProfilesInput{MaxResults: aws.Int32(1000), TypeEquals: profileType}
		if cursor != "" {
			input.NextToken = aws.String(cursor)
		}
		var output *awsbedrock.ListInferenceProfilesOutput
		err := sources.RetryProviderCall(pageCtx, s.retry, func(callCtx context.Context) (sources.RetryHint, error) {
			var callErr error
			output, callErr = client.ListInferenceProfiles(callCtx, input)
			return retryHint(callErr), callErr
		})
		if err != nil {
			return sources.Page[types.InferenceProfileSummary]{}, err
		}
		return sources.Page[types.InferenceProfileSummary]{Records: output.InferenceProfileSummaries, NextCursor: aws.ToString(output.NextToken)}, nil
	})
}

func foundationRecords(region string, summary types.FoundationModelSummary) (catalogs.ModelDefinition, catalogs.ProviderOffering, error) {
	modelID := strings.TrimSpace(aws.ToString(summary.ModelId))
	name := strings.TrimSpace(aws.ToString(summary.ModelName))
	providerName := strings.TrimSpace(aws.ToString(summary.ProviderName))
	if modelID == "" || name == "" || providerName == "" {
		return catalogs.ModelDefinition{}, catalogs.ProviderOffering{}, &errors.ValidationError{Field: "bedrock.foundation_model", Value: modelID, Message: "model id, name, and provider name are required"}
	}
	definitionID := canonicalDefinitionID(providerName, modelID)
	inputModalities, err := bedrockModalities(summary.InputModalities)
	if err != nil {
		return catalogs.ModelDefinition{}, catalogs.ProviderOffering{}, err
	}
	outputModalities, err := bedrockModalities(summary.OutputModalities)
	if err != nil {
		return catalogs.ModelDefinition{}, catalogs.ProviderOffering{}, err
	}
	delivery := &catalogs.ModelDelivery{Protocols: []catalogs.ModelResponseProtocol{catalogs.ModelResponseProtocolHTTP}}
	if aws.ToBool(summary.ResponseStreamingSupported) {
		delivery.Streaming = []catalogs.ModelStreaming{catalogs.ModelStreamingChunked}
	}
	definition := catalogs.ModelDefinition{
		ID: definitionID, Name: name, AuthorIDs: []catalogs.AuthorID{catalogs.AuthorID(slug(providerName))},
		Capabilities: catalogs.ModelDefinitionCapabilities{
			Features: &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{Input: inputModalities, Output: outputModalities}},
			Delivery: delivery,
		},
	}
	lifecycle := catalogs.OfferingLifecycleActive
	availability := catalogs.OfferingAvailabilityAvailable
	if summary.ModelLifecycle != nil && summary.ModelLifecycle.Status == types.FoundationModelLifecycleStatusLegacy {
		lifecycle = catalogs.OfferingLifecycleDeprecated
		availability = catalogs.OfferingAvailabilityRestricted
	}
	onDemand := supportsInferenceType(summary.InferenceTypesSupported, types.InferenceTypeOnDemand)
	access := catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityDiscoverable}
	if onDemand {
		access.Routability = catalogs.OfferingRoutabilityRoutable
		access.APIs = []catalogs.InvocationAPI{catalogs.InvocationAPIBedrockInvokeModel}
	}
	var modes map[string]catalogs.ProviderOfferingMode
	if supportsInferenceType(summary.InferenceTypesSupported, types.InferenceTypeProvisioned) {
		modes = map[string]catalogs.ProviderOfferingMode{"provisioned_throughput": {}}
	}
	tier := "provisioned"
	if onDemand {
		tier = tierOnDemand
	}
	offering := catalogs.ProviderOffering{
		ProviderID: ProviderID, ProviderModelID: catalogs.ProviderModelID(modelID), DefinitionID: definitionID,
		Availability: availability,
		Access:       access,
		Regions:      []catalogs.CloudRegion{cloudRegion(region, false)}, Deployment: catalogs.ProviderDeployment{Type: "foundation_model", Tier: tier},
		Endpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeBedrock}, Lifecycle: lifecycle, Modes: modes,
	}
	if err := definition.Validate(); err != nil {
		return catalogs.ModelDefinition{}, catalogs.ProviderOffering{}, err
	}
	if err := offering.Validate(); err != nil {
		return catalogs.ModelDefinition{}, catalogs.ProviderOffering{}, err
	}
	return definition, offering, nil
}

func systemProfileOffering(sourceRegion string, profile types.InferenceProfileSummary, definitions map[string]catalogs.ModelDefinitionID) (catalogs.ProviderOffering, error) {
	profileID := strings.TrimSpace(aws.ToString(profile.InferenceProfileId))
	modelIDs, destinations := profileModels(profile.Models)
	if profileID == "" || len(modelIDs) == 0 || len(destinations) == 0 {
		return catalogs.ProviderOffering{}, &errors.ValidationError{Field: "bedrock.system_inference_profile", Value: profileID, Message: "profile id and destination model ARNs are required"}
	}
	definitionID := definitions[modelIDs[0]]
	if definitionID == "" {
		return catalogs.ProviderOffering{}, &errors.NotFoundError{Resource: "Bedrock foundation model", ID: modelIDs[0]}
	}
	for _, modelID := range modelIDs[1:] {
		if candidate := definitions[modelID]; candidate == "" || candidate != definitionID {
			return catalogs.ProviderOffering{}, &errors.ConflictError{Resource: "Bedrock inference profile model", Expected: string(definitionID), Actual: string(candidate), Message: "one inference profile must route one canonical model definition"}
		}
	}
	availability := catalogs.OfferingAvailabilityAvailable
	lifecycle := catalogs.OfferingLifecycleActive
	if profile.Status != types.InferenceProfileStatusActive {
		availability = catalogs.OfferingAvailabilityUnavailable
		lifecycle = catalogs.OfferingLifecycleRetired
	}
	offering := catalogs.ProviderOffering{
		ProviderID: ProviderID, ProviderModelID: catalogs.ProviderModelID(profileID), DefinitionID: definitionID,
		Availability: availability,
		Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIBedrockInvokeModel}},
		Regions:      []catalogs.CloudRegion{cloudRegion(sourceRegion, false)}, Deployment: catalogs.ProviderDeployment{Type: "cross_region", Tier: tierOnDemand},
		InferenceProfile: &catalogs.CrossRegionInferenceProfile{ID: profileID, Scope: profileScope(profileID), SourceRegions: []string{sourceRegion}, DestinationRegions: destinations},
		Endpoint:         catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeBedrock}, Lifecycle: lifecycle,
	}
	return offering, offering.Validate()
}

func applicationProfileOfferings(region string, profiles []types.InferenceProfileSummary, definitions map[string]catalogs.ModelDefinitionID) ([]catalogs.ProviderOffering, error) {
	offerings := make([]catalogs.ProviderOffering, 0, len(profiles))
	var accountID string
	for _, profile := range profiles {
		profileID := strings.TrimSpace(aws.ToString(profile.InferenceProfileId))
		profileARN := strings.TrimSpace(aws.ToString(profile.InferenceProfileArn))
		modelIDs, _ := profileModels(profile.Models)
		if profileID == "" || profileARN == "" || len(modelIDs) == 0 {
			return nil, &errors.ValidationError{Field: "bedrock.application_inference_profile", Value: profileID, Message: "profile id, ARN, and model are required"}
		}
		profileAccountID := arnPart(profileARN, 4)
		if accountID == "" {
			accountID = profileAccountID
		} else if accountID != profileAccountID {
			return nil, &errors.ConflictError{Resource: "Bedrock account scope", Expected: accountID, Actual: profileAccountID}
		}
		definitionID := definitions[modelIDs[0]]
		if definitionID == "" {
			return nil, &errors.NotFoundError{Resource: "Bedrock foundation model", ID: modelIDs[0]}
		}
		availability, lifecycle := catalogs.OfferingAvailabilityAvailable, catalogs.OfferingLifecycleActive
		if profile.Status != types.InferenceProfileStatusActive {
			availability, lifecycle = catalogs.OfferingAvailabilityUnavailable, catalogs.OfferingLifecycleRetired
		}
		offering := catalogs.ProviderOffering{
			ProviderID: ProviderID, ProviderModelID: catalogs.ProviderModelID(profileID), DeploymentID: profileARN, DefinitionID: definitionID,
			Availability: availability,
			Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIBedrockInvokeModel}},
			Regions:      []catalogs.CloudRegion{{ID: region, Realm: realm(region)}}, Deployment: catalogs.ProviderDeployment{Type: "application_inference_profile", Tier: tierOnDemand},
			Endpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeBedrock}, Lifecycle: lifecycle,
		}
		if err := offering.Validate(); err != nil {
			return nil, err
		}
		offerings = append(offerings, offering)
	}
	return offerings, nil
}

func mergeOffering(values map[catalogs.OfferingKey]catalogs.ProviderOffering, incoming catalogs.ProviderOffering) {
	key := incoming.Key()
	existing, found := values[key]
	if !found {
		values[key] = incoming
		return
	}
	existing.Regions = unionRegions(existing.Regions, incoming.Regions)
	if existing.InferenceProfile != nil && incoming.InferenceProfile != nil {
		existing.InferenceProfile.SourceRegions = unionStrings(existing.InferenceProfile.SourceRegions, incoming.InferenceProfile.SourceRegions)
		existing.InferenceProfile.DestinationRegions = unionStrings(existing.InferenceProfile.DestinationRegions, incoming.InferenceProfile.DestinationRegions)
	}
	values[key] = existing
}

func profileModels(models []types.InferenceProfileModel) ([]string, []string) {
	modelIDs := make([]string, 0, len(models))
	regions := make([]string, 0, len(models))
	for _, model := range models {
		arn := aws.ToString(model.ModelArn)
		if id := strings.TrimPrefix(arnPart(arn, 5), "foundation-model/"); id != "" {
			modelIDs = append(modelIDs, id)
		}
		if region := arnPart(arn, 3); region != "" {
			regions = append(regions, region)
		}
	}
	return unionStrings(nil, modelIDs), unionStrings(nil, regions)
}

func retryHint(err error) sources.RetryHint {
	var responseErr *http.ResponseError
	if stderrors.As(err, &responseErr) {
		return sources.RetryHint{StatusCode: responseErr.HTTPStatusCode()}
	}
	return sources.RetryHint{}
}

func canonicalDefinitionID(providerName, modelID string) catalogs.ModelDefinitionID {
	author := slug(providerName)
	model := modelID
	for _, separator := range []string{".", ":", "/"} {
		if remainder, found := strings.CutPrefix(strings.ToLower(model), author+separator); found {
			model = remainder
			break
		}
	}
	return catalogs.ModelDefinitionID(author + "/" + model)
}

func slug(value string) string {
	return strings.Trim(strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, value), "-")
}

func supportsInferenceType(values []types.InferenceType, wanted types.InferenceType) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func bedrockModalities(values []types.ModelModality) ([]catalogs.ModelModality, error) {
	result := make([]catalogs.ModelModality, 0, len(values))
	for _, value := range values {
		var modality catalogs.ModelModality
		switch value {
		case types.ModelModalityText:
			modality = catalogs.ModelModalityText
		case types.ModelModalityImage:
			modality = catalogs.ModelModalityImage
		case types.ModelModalityEmbedding:
			modality = catalogs.ModelModalityEmbedding
		default:
			return nil, &errors.ValidationError{Field: "bedrock.model_modality", Value: value, Message: "is not mapped"}
		}
		if !slices.Contains(result, modality) {
			result = append(result, modality)
		}
	}
	return result, nil
}

func cloudRegion(region string, destination bool) catalogs.CloudRegion {
	return catalogs.CloudRegion{ID: region, Realm: realm(region), Destination: destination}
}

func realm(region string) string {
	switch {
	case strings.HasPrefix(region, "us-gov-"):
		return "aws-us-gov"
	case strings.HasPrefix(region, "cn-"):
		return "aws-cn"
	default:
		return "aws"
	}
}

func profileScope(profileID string) string {
	prefix, _, found := strings.Cut(profileID, ".")
	if !found {
		return modeRegional
	}
	switch prefix {
	case "us", "eu", "apac", "global":
		return strings.ToUpper(prefix)
	default:
		return modeRegional
	}
}

func arnPart(value string, index int) string {
	parts := strings.SplitN(value, ":", 6)
	if len(parts) != 6 || index < 0 || index >= len(parts) {
		return ""
	}
	return parts[index]
}

func normalizedRegions(regions []string) []string {
	result := make([]string, 0, len(regions))
	for _, region := range regions {
		if region = strings.TrimSpace(region); region != "" && !slices.Contains(result, region) {
			result = append(result, region)
		}
	}
	slices.Sort(result)
	return result
}

func unionStrings(left, right []string) []string {
	result := append(append([]string(nil), left...), right...)
	slices.Sort(result)
	return slices.Compact(result)
}

func unionRegions(left, right []catalogs.CloudRegion) []catalogs.CloudRegion {
	byID := make(map[string]catalogs.CloudRegion, len(left)+len(right))
	for _, region := range append(append([]catalogs.CloudRegion(nil), left...), right...) {
		byID[region.ID] = region
	}
	result := make([]catalogs.CloudRegion, 0, len(byID))
	for _, region := range byID {
		result = append(result, region)
	}
	slices.SortFunc(result, func(left, right catalogs.CloudRegion) int { return strings.Compare(left.ID, right.ID) })
	return result
}

func definitionValues(values map[catalogs.ModelDefinitionID]catalogs.ModelDefinition) []catalogs.ModelDefinition {
	result := make([]catalogs.ModelDefinition, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	slices.SortFunc(result, func(left, right catalogs.ModelDefinition) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	return result
}

func offeringValues(values map[catalogs.OfferingKey]catalogs.ProviderOffering) []catalogs.ProviderOffering {
	result := make([]catalogs.ProviderOffering, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	slices.SortFunc(result, func(left, right catalogs.ProviderOffering) int {
		return strings.Compare(fmt.Sprint(left.ProviderID, "/", left.ProviderModelID), fmt.Sprint(right.ProviderID, "/", right.ProviderModelID))
	})
	return result
}
