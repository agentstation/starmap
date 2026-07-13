// Package watsonx implements IBM watsonx.ai foundation model inventory.
package watsonx

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

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
	transport *transport.Client
}

// NewClient creates a watsonx.ai client.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{provider: provider, transport: transport.New(provider)}
}

// IsAPIKeyRequired reports that regional discovery requires IBM auth.
func (c *Client) IsAPIKeyRequired() bool { return true }

// HasAPIKey reports whether token and regional base URL are resolved.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider != nil && c.provider.HasAPIKey() && c.provider.EnvVar("IBM_WATSONX_BASE_URL") != ""
}

// ListModels traverses the opaque start-token pagination contract.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	configured, client := c.provider, c.transport
	c.mu.RUnlock()
	if configured == nil || configured.EnvVar("IBM_WATSONX_BASE_URL") == "" {
		return nil, &errors.ConfigError{Component: "watsonx", Message: "regional base URL is required"}
	}
	endpoint, err := url.Parse(configured.CatalogEndpointURL())
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
		pageResult, fetchErr := fetchPage(ctx, client, configured, endpoint.String())
		if fetchErr != nil {
			return nil, fetchErr
		}
		for _, source := range pageResult.Resources {
			converted, convertErr := convertModel(source, configured.EnvVar("IBM_WATSONX_REGION"))
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

func fetchPage(ctx context.Context, client *transport.Client, provider *catalogs.Provider, endpoint string) (response, error) {
	httpResponse, err := client.Get(ctx, endpoint, provider)
	if err != nil {
		return response{}, &errors.APIError{Provider: "watsonx", Endpoint: endpoint, Message: "request failed", Err: err}
	}
	defer func() { _ = httpResponse.Body.Close() }()
	var result response
	if err := transport.DecodeResponse(httpResponse, &result); err != nil {
		return response{}, &errors.APIError{Provider: "watsonx", Endpoint: endpoint, StatusCode: httpResponse.StatusCode, Message: "failed to decode response", Err: err}
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
		Extensions:         catalogs.SourceExtensions{"watsonx": {Fields: catalogs.NormalizeExtensionFields(map[string]any{"source": source.Source, "api_version": apiVersion})}},
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

// FetchDeployments returns credential-scoped deployments without admitting them to the public catalog.
func FetchDeployments(ctx context.Context, config DeploymentConfig) (catalogs.CustomerInventory, error) {
	base, scope, err := validateDeploymentConfig(config)
	if err != nil {
		return catalogs.CustomerInventory{}, err
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
			return catalogs.CustomerInventory{}, fetchErr
		}
		resources = append(resources, result.Resources...)
		if result.Next == nil || result.Next.Href == "" {
			return deploymentInventory(config, scope, resources)
		}
		next, parseErr := url.Parse(result.Next.Href)
		if parseErr != nil || next.Scheme != endpoint.Scheme || next.Host != endpoint.Host {
			return catalogs.CustomerInventory{}, &errors.ValidationError{Field: "watsonx.deployments.next.href", Value: result.Next.Href, Message: "must remain on the configured regional origin"}
		}
		if next.String() == endpoint.String() {
			return catalogs.CustomerInventory{}, &errors.ValidationError{Field: "watsonx.deployments.next.href", Value: next.String(), Message: "cursor repeated"}
		}
		endpoint = *next
	}
	return catalogs.CustomerInventory{}, &errors.ValidationError{Field: "watsonx.deployments.pages", Value: maxPages, Message: "page limit exceeded"}
}

func validateDeploymentConfig(config DeploymentConfig) (*url.URL, catalogs.CustomerScope, error) {
	if config.Token == "" || len(config.DefinitionByAsset) == 0 || (config.ProjectID == "") == (config.SpaceID == "") {
		return nil, catalogs.CustomerScope{}, &errors.ValidationError{Field: "watsonx.deployments.config", Message: "token, explicit definition mapping, and exactly one project or space are required"}
	}
	base, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, catalogs.CustomerScope{}, errors.WrapParse("url", "watsonx regional base URL", err)
	}
	loopbackHTTP := base.Scheme == "http" && (base.Hostname() == "127.0.0.1" || base.Hostname() == "localhost")
	if (base.Scheme != "https" && !loopbackHTTP) || base.Host == "" {
		return nil, catalogs.CustomerScope{}, &errors.ValidationError{Field: "watsonx.deployments.base_url", Value: config.BaseURL, Message: "absolute HTTPS or loopback development URL is required"}
	}
	scope := catalogs.CustomerScope{ProjectID: config.ProjectID}
	if config.SpaceID != "" {
		scope.WorkspaceID = config.SpaceID
	}
	return base, scope, nil
}

func fetchDeploymentPage(ctx context.Context, client *http.Client, endpoint, token string) (deploymentPage, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return deploymentPage{}, errors.WrapResource("create", "watsonx deployment request", endpoint, err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return deploymentPage{}, &errors.APIError{Provider: "watsonx", Endpoint: endpoint, Message: "request failed", Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	var result deploymentPage
	if err := transport.DecodeResponse(response, &result); err != nil {
		return deploymentPage{}, &errors.APIError{Provider: "watsonx", Endpoint: endpoint, StatusCode: response.StatusCode, Message: "failed to decode deployment response", Err: err}
	}
	if result.Resources == nil {
		return deploymentPage{}, &errors.ValidationError{Field: "watsonx.deployments.resources", Message: "required resource array is null"}
	}
	return result, nil
}

func deploymentInventory(config DeploymentConfig, scope catalogs.CustomerScope, resources []deploymentResource) (catalogs.CustomerInventory, error) {
	inventory := catalogs.CustomerInventory{ProviderID: catalogs.ProviderIDWatsonx, Scope: scope, ObservedAt: time.Now().UTC()}
	for _, resource := range resources {
		identity, providerModelID := deploymentIdentity(resource)
		definitionID, found := config.DefinitionByAsset[identity]
		if !found {
			return catalogs.CustomerInventory{}, &errors.NotFoundError{Resource: "watsonx deployment definition", ID: identity}
		}
		aliases := make([]string, 0, 2)
		if resource.Entity.Name != "" {
			aliases = append(aliases, resource.Entity.Name)
		}
		if resource.Entity.Online.Parameters.ServingName != "" {
			aliases = append(aliases, resource.Entity.Online.Parameters.ServingName)
		}
		inventory.Deployments = append(inventory.Deployments, catalogs.CustomerDeployment{
			ID: resource.Metadata.ID, DefinitionID: definitionID, ProviderModelID: catalogs.ProviderModelID(providerModelID), Region: config.Region,
			Deployment: catalogs.ProviderDeployment{Type: deploymentType(resource.Entity.DeployedAssetType)},
			Endpoint:   strings.TrimRight(config.BaseURL, "/") + "/ml/v1/deployments/" + resource.Metadata.ID + "/text/generation", Aliases: aliases,
		})
	}
	if err := inventory.Validate(); err != nil {
		return catalogs.CustomerInventory{}, err
	}
	return inventory, nil
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
