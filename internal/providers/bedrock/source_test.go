package bedrock

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsbedrock "github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/bedrock/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type fakeAPI struct {
	region          string
	mu              sync.Mutex
	foundationCalls int
	profileCalls    map[types.InferenceProfileType]int
	foundationErrs  []error
}

type staticPricingFetcher struct {
	catalog pricingCatalog
	err     error
}

func (f staticPricingFetcher) Fetch(context.Context) (pricingCatalog, error) { return f.catalog, f.err }

func (f *fakeAPI) ListFoundationModels(context.Context, *awsbedrock.ListFoundationModelsInput, ...func(*awsbedrock.Options)) (*awsbedrock.ListFoundationModelsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.foundationCalls++
	if len(f.foundationErrs) > 0 {
		err := f.foundationErrs[0]
		f.foundationErrs = f.foundationErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	return &awsbedrock.ListFoundationModelsOutput{ModelSummaries: []types.FoundationModelSummary{{
		ModelArn: aws.String("arn:aws:bedrock:" + f.region + "::foundation-model/anthropic.claude-3"),
		ModelId:  aws.String("anthropic.claude-3"), ModelName: aws.String("Claude 3"), ProviderName: aws.String("Anthropic"),
		InferenceTypesSupported: []types.InferenceType{types.InferenceTypeOnDemand},
		InputModalities:         []types.ModelModality{types.ModelModalityText, types.ModelModalityImage}, OutputModalities: []types.ModelModality{types.ModelModalityText},
		ResponseStreamingSupported: aws.Bool(true),
		ModelLifecycle:             &types.FoundationModelLifecycle{Status: types.FoundationModelLifecycleStatusActive},
	}}}, nil
}

func (f *fakeAPI) ListInferenceProfiles(_ context.Context, input *awsbedrock.ListInferenceProfilesInput, _ ...func(*awsbedrock.Options)) (*awsbedrock.ListInferenceProfilesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.profileCalls == nil {
		f.profileCalls = make(map[types.InferenceProfileType]int)
	}
	f.profileCalls[input.TypeEquals]++
	if input.TypeEquals == types.InferenceProfileTypeApplication {
		if f.region != "us-east-1" {
			return &awsbedrock.ListInferenceProfilesOutput{}, nil
		}
		return &awsbedrock.ListInferenceProfilesOutput{InferenceProfileSummaries: []types.InferenceProfileSummary{{
			InferenceProfileArn: aws.String("arn:aws:bedrock:us-east-1:123456789012:application-inference-profile/team"),
			InferenceProfileId:  aws.String("team"), InferenceProfileName: aws.String("Team"),
			Models: []types.InferenceProfileModel{{ModelArn: aws.String("arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-3")}},
			Status: types.InferenceProfileStatusActive, Type: types.InferenceProfileTypeApplication,
		}}}, nil
	}
	if aws.ToString(input.NextToken) == "" {
		return &awsbedrock.ListInferenceProfilesOutput{NextToken: aws.String("page-2")}, nil
	}
	return &awsbedrock.ListInferenceProfilesOutput{InferenceProfileSummaries: []types.InferenceProfileSummary{{
		InferenceProfileArn: aws.String("arn:aws:bedrock:" + f.region + "::inference-profile/us.anthropic.claude-3"),
		InferenceProfileId:  aws.String("us.anthropic.claude-3"), InferenceProfileName: aws.String("US Claude 3"),
		Models: []types.InferenceProfileModel{
			{ModelArn: aws.String("arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-3")},
			{ModelArn: aws.String("arn:aws:bedrock:us-west-2::foundation-model/anthropic.claude-3")},
		}, Status: types.InferenceProfileStatusActive, Type: types.InferenceProfileTypeSystemDefined,
	}}}, nil
}

func TestSourceMergesRegionalModelsAndNormalizesApplicationProfiles(t *testing.T) {
	clients := map[string]*fakeAPI{
		"us-east-1": {region: "us-east-1"},
		"us-west-2": {region: "us-west-2"},
	}
	source, err := NewSource([]string{"us-west-2", "us-east-1", "us-east-1"}, func(_ context.Context, region string) (API, error) {
		return clients[region], nil
	})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	source.now = func() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC) }

	result, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(result.Definitions) != 1 || result.Definitions[0].ID != "anthropic/claude-3" {
		t.Fatalf("definitions = %#v", result.Definitions)
	}
	if result.Definitions[0].Capabilities.Features == nil || len(result.Definitions[0].Capabilities.Features.Modalities.Input) != 2 || result.Definitions[0].Capabilities.Delivery == nil || len(result.Definitions[0].Capabilities.Delivery.Streaming) != 1 {
		t.Fatalf("foundation capabilities = %#v", result.Definitions[0].Capabilities)
	}
	if len(result.Offerings) != 3 {
		t.Fatalf("offerings = %#v", result.Offerings)
	}
	var foundation, system, application catalogs.ProviderOffering
	for _, offering := range result.Offerings {
		switch offering.ProviderModelID {
		case "anthropic.claude-3":
			foundation = offering
		case "us.anthropic.claude-3":
			system = offering
		case "team":
			application = offering
		}
	}
	if len(foundation.Regions) != 2 || foundation.Endpoint.Type != catalogs.EndpointTypeBedrock {
		t.Fatalf("regional foundation offering = %#v", foundation)
	}
	if system.InferenceProfile == nil || len(system.InferenceProfile.SourceRegions) != 2 || len(system.InferenceProfile.DestinationRegions) != 2 || system.InferenceProfile.Scope != "US" {
		t.Fatalf("system profile = %#v", system)
	}
	if application.DeploymentID != "arn:aws:bedrock:us-east-1:123456789012:application-inference-profile/team" || application.Deployment.Type != "application_inference_profile" || application.DefinitionID != "anthropic/claude-3" {
		t.Fatalf("application offering = %#v", application)
	}
}

func TestProvisionedOnlyFoundationModelIsDiscoverableNotRoutable(t *testing.T) {
	_, offering, err := foundationRecords("us-east-1", types.FoundationModelSummary{
		ModelId: aws.String("amazon.titan"), ModelName: aws.String("Titan"), ProviderName: aws.String("Amazon"),
		InferenceTypesSupported: []types.InferenceType{types.InferenceTypeProvisioned},
		InputModalities:         []types.ModelModality{types.ModelModalityText}, OutputModalities: []types.ModelModality{types.ModelModalityText},
		ModelLifecycle: &types.FoundationModelLifecycle{Status: types.FoundationModelLifecycleStatusActive},
	})
	if err != nil {
		t.Fatalf("foundationRecords: %v", err)
	}
	if offering.IsRoutable() || offering.Access.Routability != catalogs.OfferingRoutabilityDiscoverable || offering.Deployment.Tier != "provisioned" {
		t.Fatalf("provisioned-only offering = %#v", offering)
	}
	if _, found := offering.Modes["provisioned_throughput"]; !found {
		t.Fatalf("provisioned mode missing: %#v", offering.Modes)
	}
}

func TestSystemInferenceProfileRejectsMultipleCanonicalModels(t *testing.T) {
	_, err := systemProfileOffering("us-east-1", types.InferenceProfileSummary{
		InferenceProfileId: aws.String("us.mixed"), InferenceProfileName: aws.String("Mixed"),
		Models: []types.InferenceProfileModel{
			{ModelArn: aws.String("arn:aws:bedrock:us-east-1::foundation-model/author.model-a")},
			{ModelArn: aws.String("arn:aws:bedrock:us-west-2::foundation-model/author.model-b")},
		}, Status: types.InferenceProfileStatusActive, Type: types.InferenceProfileTypeSystemDefined,
	}, map[string]catalogs.ModelDefinitionID{"author.model-a": "author/model-a", "author.model-b": "author/model-b"})
	if err == nil {
		t.Fatal("systemProfileOffering accepted multiple canonical definitions")
	}
}

func TestSourceRetriesThrottlingWithinBound(t *testing.T) {
	throttle := &smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusTooManyRequests}}, Err: stderrors.New("throttled")}
	client := &fakeAPI{region: "us-east-1", foundationErrs: []error{throttle, nil}}
	source, err := NewSource([]string{"us-east-1"}, func(context.Context, string) (API, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	source.retry.BaseDelay = time.Nanosecond
	source.retry.MaxDelay = time.Nanosecond
	source.retry.JitterFraction = 0
	if _, err := source.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch after throttle: %v", err)
	}
	if client.foundationCalls != 2 {
		t.Fatalf("foundation calls = %d, want 2", client.foundationCalls)
	}
}

func TestSourceObservationReturnsCanonicalCredentialScopedCatalog(t *testing.T) {
	client := &fakeAPI{region: "us-east-1"}
	source, err := NewSource([]string{"us-east-1"}, func(context.Context, string) (API, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC) }
	pricingAt := time.Date(2026, 7, 3, 8, 58, 57, 0, time.UTC)
	source.pricing = staticPricingFetcher{catalog: pricingCatalog{
		Version: "version", PublishedAt: pricingAt,
		Prices: map[pricingKey]*catalogs.ModelPricing{{ServiceName: "claude 3", Region: "us-east-1", Mode: "regional"}: {
			Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: 3}},
		}},
	}}
	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.SourceID != sources.AmazonBedrockID || observation.Metrics.Scope != catalogmeta.ObservationScopeCredentialScoped || observation.Metrics.ProviderCoverage.Expected != 1 || observation.Metrics.ProviderCoverage.Observed != 1 || observation.Metrics.PricingObservedAt == nil || !observation.Metrics.PricingObservedAt.Equal(pricingAt) {
		t.Fatalf("observation identity/coverage = %#v", observation)
	}
	offering, err := observation.Catalog.Offering(ProviderID, "anthropic.claude-3")
	if err != nil {
		t.Fatalf("canonical Bedrock offering missing: %v", err)
	}
	if offering.Modes["regional/us-east-1"].Pricing.Tokens.Input.Per1M != 3 {
		t.Fatalf("regional pricing missing from observation: %#v", offering.Modes)
	}
	payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) == "" || !containsAny(string(payload), "application-inference-profile/team", "123456789012") {
		t.Fatalf("contextual observation omitted deployment identity: %s", payload)
	}
}

func TestSourceObservationDegradesSafelyWithoutCredentials(t *testing.T) {
	source, err := NewSource([]string{"us-east-1"}, func(context.Context, string) (API, error) {
		return nil, &pkgerrors.AuthenticationError{Provider: string(ProviderID), Method: "aws_sdk_default_chain", Message: "unavailable"}
	})
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC) }
	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("optional Observe returned fatal error: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded || observation.Completeness != sources.ObservationCompletenessPartial || len(observation.Issues) != 1 || observation.Issues[0].Code != sources.ObservationIssueCodeMissingCredentials {
		t.Fatalf("missing-credential observation = %#v", observation)
	}
	if observation.Catalog == nil || observation.Metrics.ProviderCoverage.Expected != 1 || observation.Metrics.ProviderCoverage.Observed != 0 {
		t.Fatalf("missing-credential catalog/coverage = %#v", observation)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
