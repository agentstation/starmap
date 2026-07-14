// Package watsonx implements IBM watsonx.ai foundation model inventory.
package watsonx

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const apiVersion = "2024-03-14"
const maxPages = 32

type response struct {
	Resources []model `json:"resources"`
	Next      *struct {
		Href string `json:"href"`
	} `json:"next"`
}

type task struct {
	ID string `json:"id"`
}
type lifecycleEntry struct {
	ID    string `json:"id"`
	Start string `json:"start"`
}

type model struct {
	ModelID          string           `json:"model_id"`
	Label            string           `json:"label"`
	Provider         string           `json:"provider"`
	Source           string           `json:"source"`
	ShortDescription string           `json:"short_description"`
	Tasks            []task           `json:"tasks"`
	Lifecycle        []lifecycleEntry `json:"lifecycle"`
	ModelLimits      *struct {
		MaxSequenceLength int64 `json:"max_sequence_length"`
	} `json:"model_limits"`
}

// Client retrieves IBM-curated regional foundation models.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	endpoint  string
	region    string
	transport *transport.Client
}

// NewClient creates a watsonx.ai client.
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	region, _ := source.Binding("region")
	return &Client{provider: &provider, endpoint: source.EndpointURL(), region: region, transport: transport.New(source.Auth())}
}

// ListModels traverses the opaque start-token pagination contract.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	configured, client, endpointValue, region := c.provider, c.transport, c.endpoint, c.region
	c.mu.RUnlock()
	if configured == nil || endpointValue == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDWatsonx), Message: "regional base URL is required"}
	}
	endpoint, err := url.Parse(endpointValue)
	if err != nil {
		return nil, errors.WrapParse("url", "watsonx foundation models", err)
	}
	query := endpoint.Query()
	query.Set("version", apiVersion)
	query.Set("limit", "200")
	endpoint.RawQuery = query.Encode()
	result := make([]catalogs.Model, 0)
	seen := make(map[string]struct{})
	for page := 0; page < maxPages; page++ {
		pageResult, fetchErr := fetchPage(ctx, client, endpoint.String())
		if fetchErr != nil {
			return nil, fetchErr
		}
		for _, source := range pageResult.Resources {
			converted, convertErr := convertModel(source, region)
			if convertErr != nil {
				return nil, convertErr
			}
			if _, exists := seen[converted.ID]; exists {
				return nil, &errors.ConflictError{Resource: "watsonx model", Message: "duplicate model ID across pages"}
			}
			seen[converted.ID] = struct{}{}
			result = append(result, converted)
		}
		if pageResult.Next == nil || pageResult.Next.Href == "" {
			slices.SortFunc(result, func(left, right catalogs.Model) int { return strings.Compare(left.ID, right.ID) })
			return result, nil
		}
		next, parseErr := url.Parse(pageResult.Next.Href)
		if parseErr != nil || next.Host != endpoint.Host || next.Scheme != endpoint.Scheme {
			return nil, &errors.ValidationError{Field: "watsonx.next.href", Value: pageResult.Next.Href, Message: "must remain on the configured regional origin"}
		}
		if next.String() == endpoint.String() {
			return nil, &errors.ValidationError{Field: "watsonx.next.href", Value: next.String(), Message: "cursor repeated"}
		}
		endpoint = next
	}
	return nil, &errors.ValidationError{Field: "watsonx.pages", Value: maxPages, Message: "page limit exceeded"}
}

func fetchPage(ctx context.Context, client *transport.Client, endpoint string) (response, error) {
	httpResponse, err := client.Get(ctx, endpoint)
	if err != nil {
		return response{}, &errors.APIError{Provider: string(catalogs.ProviderIDWatsonx), Endpoint: endpoint, Message: "request failed", Err: err}
	}
	defer func() { _ = httpResponse.Body.Close() }()
	var result response
	if err := transport.DecodeResponse(httpResponse, &result); err != nil {
		return response{}, &errors.APIError{Provider: string(catalogs.ProviderIDWatsonx), Endpoint: endpoint, StatusCode: httpResponse.StatusCode, Message: "failed to decode response", Err: err}
	}
	if result.Resources == nil {
		return response{}, &errors.ValidationError{Field: "watsonx.resources", Message: "required resource array is null"}
	}
	return result, nil
}

func convertModel(source model, region string) (catalogs.Model, error) {
	if strings.TrimSpace(source.ModelID) == "" || strings.TrimSpace(source.Provider) == "" {
		return catalogs.Model{}, &errors.ValidationError{Field: "watsonx.model", Value: source.ModelID, Message: "model_id and provider are required"}
	}
	name := source.Label
	if name == "" {
		name = source.ModelID
	}
	result := catalogs.Model{
		ID: source.ModelID, Name: name, Description: source.ShortDescription,
		Authors: []catalogs.Author{{ID: watsonxAuthor(source.Provider), Name: source.Provider}}, Status: lifecycle(source.Lifecycle),
		InvocationAPIs: invocationAPIs(source.Tasks), OfferingEndpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeWatsonx},
		OfferingDeployment: catalogs.ProviderDeployment{Type: "curated-multitenant", Tier: "pay-per-token"},
		Extensions:         catalogs.SourceExtensions{string(catalogs.ProviderIDWatsonx): {Fields: catalogs.NormalizeExtensionFields(map[string]any{"source": source.Source, "api_version": apiVersion})}},
	}
	if region != "" {
		result.OfferingRegions = []catalogs.CloudRegion{{ID: region}}
	}
	if source.ModelLimits != nil {
		if source.ModelLimits.MaxSequenceLength < 0 {
			return catalogs.Model{}, &errors.ValidationError{Field: "watsonx.model_limits.max_sequence_length", Value: source.ModelLimits.MaxSequenceLength, Message: "must not be negative"}
		}
		result.Limits = &catalogs.ModelLimits{ContextWindow: source.ModelLimits.MaxSequenceLength}
	}
	if len(result.InvocationAPIs) == 0 {
		result.OfferingAccess = &catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityDiscoverable, APIs: []catalogs.InvocationAPI{}}
	}
	return result, nil
}

func invocationAPIs(tasks []task) []catalogs.InvocationAPI {
	set := make(map[catalogs.InvocationAPI]struct{})
	for _, task := range tasks {
		switch task.ID {
		case "text_generation", "chat", "code":
			set[catalogs.InvocationAPIWatsonxGenerate] = struct{}{}
		case "embedding", "sentence_similarity":
			set[catalogs.InvocationAPIEmbeddings] = struct{}{}
		}
	}
	result := make([]catalogs.InvocationAPI, 0, len(set))
	for api := range set {
		result = append(result, api)
	}
	slices.Sort(result)
	return result
}

func lifecycle(values []lifecycleEntry) catalogs.ModelStatus {
	for _, value := range values {
		switch strings.ToLower(value.ID) {
		case "withdrawn", "deprecated":
			return catalogs.ModelStatusDeprecated
		case "available", "active":
			return catalogs.ModelStatusActive
		}
	}
	return catalogs.ModelStatusActive
}

func watsonxAuthor(value string) catalogs.AuthorID {
	switch strings.ToLower(value) {
	case "ibm":
		return catalogs.AuthorID("ibm")
	case "meta":
		return catalogs.AuthorIDMeta
	case "mistral ai", "mistral":
		return catalogs.AuthorIDMistralAI
	case "google":
		return catalogs.AuthorIDGoogle
	case "deepseek":
		return catalogs.AuthorIDDeepSeek
	default:
		return catalogs.AuthorID(strings.ToLower(strings.ReplaceAll(value, " ", "-")))
	}
}

// DeploymentConfig is private watsonx.ai project or deployment-space discovery configuration.
type DeploymentConfig struct {
	BaseURL           string                                `json:"-" yaml:"-"`
	Token             string                                `json:"-" yaml:"-"`
	ProjectID         string                                `json:"-" yaml:"-"`
	SpaceID           string                                `json:"-" yaml:"-"`
	DefinitionByAsset map[string]catalogs.ModelDefinitionID `json:"-" yaml:"-"`
	Region            *catalogs.CloudRegion                 `json:"-" yaml:"-"`
}

// DeploymentResult contains canonical records returned by one project or space observation.
type DeploymentResult struct {
	Definitions []catalogs.ModelDefinition
	Offerings   []catalogs.ProviderOffering
}

// Catalog materializes deployment records in the single canonical catalog.
func (result DeploymentResult) Catalog(provider catalogs.Provider) (*catalogs.Catalog, error) {
	builder := catalogs.NewEmpty()
	provider.Models = nil
	if err := builder.SetProvider(provider); err != nil {
		return nil, err
	}
	for _, definition := range result.Definitions {
		if err := builder.SetDefinition(definition); err != nil {
			return nil, err
		}
	}
	for _, offering := range result.Offerings {
		if err := builder.SetOffering(offering); err != nil {
			return nil, err
		}
	}
	return builder.Build()
}

type deploymentPage struct {
	Resources []deploymentResource `json:"resources"`
	Next      *struct {
		Href string `json:"href"`
	} `json:"next"`
}

type deploymentResource struct {
	Metadata struct {
		ID string `json:"id"`
	} `json:"metadata"`
	Entity struct {
		Name              string `json:"name"`
		DeployedAssetType string `json:"deployed_asset_type"`
		Asset             struct {
			ID string `json:"id"`
		} `json:"asset"`
		FoundationModel struct {
			ModelID string `json:"model_id"`
		} `json:"foundation_model"`
		Online struct {
			Parameters struct {
				ServingName     string `json:"serving_name"`
				FoundationModel struct {
					ModelID string `json:"model_id"`
				} `json:"foundation_model"`
			} `json:"parameters"`
		} `json:"online"`
	} `json:"entity"`
}

// FetchDeployments returns credential-scoped deployments as canonical contextual offerings.
func FetchDeployments(ctx context.Context, config DeploymentConfig) (DeploymentResult, error) {
	base, err := validateDeploymentConfig(config)
	if err != nil {
		return DeploymentResult{}, err
	}
	client := &http.Client{Timeout: constants.DefaultTimeout}
	resources := make([]deploymentResource, 0)
	endpoint := *base
	endpoint.Path = "/ml/v4/deployments"
	query := endpoint.Query()
	query.Set("version", apiVersion)
	if config.ProjectID != "" {
		query.Set("project_id", config.ProjectID)
	} else {
		query.Set("space_id", config.SpaceID)
	}
	endpoint.RawQuery = query.Encode()
	for page := 0; page < maxPages; page++ {
		result, fetchErr := fetchDeploymentPage(ctx, client, endpoint.String(), config.Token)
		if fetchErr != nil {
			return DeploymentResult{}, fetchErr
		}
		resources = append(resources, result.Resources...)
		if result.Next == nil || result.Next.Href == "" {
			return deploymentOfferings(config, resources)
		}
		next, parseErr := url.Parse(result.Next.Href)
		if parseErr != nil || next.Scheme != endpoint.Scheme || next.Host != endpoint.Host {
			return DeploymentResult{}, &errors.ValidationError{Field: "watsonx.deployments.next.href", Value: result.Next.Href, Message: "must remain on the configured regional origin"}
		}
		if next.String() == endpoint.String() {
			return DeploymentResult{}, &errors.ValidationError{Field: "watsonx.deployments.next.href", Value: next.String(), Message: "cursor repeated"}
		}
		endpoint = *next
	}
	return DeploymentResult{}, &errors.ValidationError{Field: "watsonx.deployments.pages", Value: maxPages, Message: "page limit exceeded"}
}

func validateDeploymentConfig(config DeploymentConfig) (*url.URL, error) {
	if config.Token == "" || (config.ProjectID == "") == (config.SpaceID == "") {
		return nil, &errors.ValidationError{Field: "watsonx.deployments.config", Message: "token and exactly one project or space are required"}
	}
	base, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, errors.WrapParse("url", "watsonx regional base URL", err)
	}
	loopbackHTTP := base.Scheme == "http" && (base.Hostname() == "127.0.0.1" || base.Hostname() == "localhost")
	if (base.Scheme != "https" && !loopbackHTTP) || base.Host == "" {
		return nil, &errors.ValidationError{Field: "watsonx.deployments.base_url", Value: config.BaseURL, Message: "absolute HTTPS or loopback development URL is required"}
	}
	return base, nil
}

func fetchDeploymentPage(ctx context.Context, client *http.Client, endpoint, token string) (deploymentPage, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return deploymentPage{}, errors.WrapResource("create", "watsonx deployment request", endpoint, err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return deploymentPage{}, &errors.APIError{Provider: string(catalogs.ProviderIDWatsonx), Endpoint: endpoint, Message: "request failed", Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	var result deploymentPage
	if err := transport.DecodeResponse(response, &result); err != nil {
		return deploymentPage{}, &errors.APIError{Provider: string(catalogs.ProviderIDWatsonx), Endpoint: endpoint, StatusCode: response.StatusCode, Message: "failed to decode deployment response", Err: err}
	}
	if result.Resources == nil {
		return deploymentPage{}, &errors.ValidationError{Field: "watsonx.deployments.resources", Message: "required resource array is null"}
	}
	return result, nil
}

func deploymentOfferings(config DeploymentConfig, resources []deploymentResource) (DeploymentResult, error) {
	offerings := make([]catalogs.ProviderOffering, 0, len(resources))
	definitions := make(map[catalogs.ModelDefinitionID]catalogs.ModelDefinition)
	for _, resource := range resources {
		identity, providerModelID := deploymentIdentity(resource)
		definitionID, found := config.DefinitionByAsset[identity]
		if !found {
			definitionID = catalogs.ModelDefinitionID(identity)
		}
		name := resource.Entity.Name
		if name == "" {
			name = providerModelID
		}
		definitions[definitionID] = catalogs.ModelDefinition{ID: definitionID, Name: name}
		aliases := make([]string, 0, 2)
		if resource.Entity.Name != "" {
			aliases = append(aliases, resource.Entity.Name)
		}
		if resource.Entity.Online.Parameters.ServingName != "" {
			aliases = append(aliases, resource.Entity.Online.Parameters.ServingName)
		}
		offering := catalogs.ProviderOffering{
			ProviderID: catalogs.ProviderIDWatsonx, ProviderModelID: catalogs.ProviderModelID(providerModelID),
			DeploymentID: resource.Metadata.ID, DefinitionID: definitionID, Aliases: aliases,
			Availability: catalogs.OfferingAvailabilityRestricted,
			Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIWatsonxGenerate}},
			Deployment:   catalogs.ProviderDeployment{Type: deploymentType(resource.Entity.DeployedAssetType)},
			Endpoint:     catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeWatsonx, BaseURL: strings.TrimRight(config.BaseURL, "/"), Path: "/ml/v1/deployments/" + resource.Metadata.ID + "/text/generation"},
			Lifecycle:    catalogs.OfferingLifecycleActive,
		}
		if config.Region != nil {
			offering.Regions = []catalogs.CloudRegion{*config.Region}
		}
		if err := offering.Validate(); err != nil {
			return DeploymentResult{}, err
		}
		offerings = append(offerings, offering)
	}
	definitionList := make([]catalogs.ModelDefinition, 0, len(definitions))
	for _, definition := range definitions {
		definitionList = append(definitionList, definition)
	}
	slices.SortFunc(definitionList, func(left, right catalogs.ModelDefinition) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	slices.SortFunc(offerings, func(left, right catalogs.ProviderOffering) int {
		return strings.Compare(left.DeploymentID, right.DeploymentID)
	})
	return DeploymentResult{Definitions: definitionList, Offerings: offerings}, nil
}

func deploymentIdentity(resource deploymentResource) (string, string) {
	modelID := resource.Entity.FoundationModel.ModelID
	if modelID == "" {
		modelID = resource.Entity.Online.Parameters.FoundationModel.ModelID
	}
	if modelID != "" {
		return modelID, modelID
	}
	return resource.Entity.Asset.ID, resource.Entity.Asset.ID
}

func deploymentType(value string) string {
	switch value {
	case "curated_foundation_model":
		return "on-demand-dedicated"
	case "custom_foundation_model":
		return "custom-dedicated"
	case "foundation_model":
		return "prompt-tuned-multitenant"
	default:
		return value
	}
}
