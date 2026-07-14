// Package databricks implements public foundation availability and private workspace inventory.
package databricks

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const maxWorkspacePages = 32

var foundationIDPattern = regexp.MustCompile(`\bdatabricks-[a-z0-9][a-z0-9-]*\b`)

// Client retrieves Databricks' public supported-foundation documentation.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	endpoint  string
	transport *transport.Client
}

// NewClient creates a Databricks public availability client.
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	return &Client{provider: &provider, endpoint: source.EndpointURL(), transport: transport.New(source.Auth())}
}

// ListModels parses exact Databricks endpoint IDs from the current first-party support matrix.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	configured, client, endpoint := c.provider, c.transport, c.endpoint
	c.mu.RUnlock()
	if configured == nil {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDDatabricks), Message: "provider not configured"}
	}
	if endpoint == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDDatabricks), Message: "catalog endpoint is required"}
	}
	response, err := client.Get(ctx, endpoint)
	if err != nil {
		return nil, &errors.APIError{Provider: string(catalogs.ProviderIDDatabricks), Endpoint: endpoint, Message: "request failed", Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, constants.MaxSourcePayloadBytes+1))
	if err != nil {
		return nil, errors.WrapIO("read", "Databricks foundation support matrix", err)
	}
	if len(body) > constants.MaxSourcePayloadBytes {
		return nil, &errors.ValidationError{Field: "databricks.foundation.body", Value: len(body), Message: "exceeds source payload limit"}
	}
	ids := slices.Compact(foundationIDPattern.FindAllString(string(body), -1))
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) < 5 {
		return nil, &errors.ValidationError{Field: "databricks.foundation.models", Value: len(ids), Message: "support matrix yielded too few exact model IDs"}
	}
	result := make([]catalogs.Model, 0, len(ids))
	for _, id := range ids {
		result = append(result, publicModel(id))
	}
	return result, nil
}

func publicModel(id string) catalogs.Model {
	access := &catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityDiscoverable, APIs: []catalogs.InvocationAPI{}}
	return catalogs.Model{
		ID: id, Name: id, Authors: []catalogs.Author{{ID: foundationAuthor(id), Name: foundationAuthor(id).String()}}, Status: catalogs.ModelStatusActive,
		InvocationAPIs: []catalogs.InvocationAPI{}, OfferingAccess: access,
		OfferingDeployment: catalogs.ProviderDeployment{Type: "pay-per-token", Tier: "foundation-model-api"},
		Extensions:         catalogs.SourceExtensions{string(catalogs.ProviderIDDatabricks): {Fields: catalogs.NormalizeExtensionFields(map[string]any{"source": "supported-foundation-matrix"})}},
	}
}

func foundationAuthor(id string) catalogs.AuthorID {
	switch {
	case strings.HasPrefix(id, "databricks-claude-"):
		return catalogs.AuthorIDAnthropic
	case strings.HasPrefix(id, "databricks-gemini-"), strings.HasPrefix(id, "databricks-gemma-"):
		return catalogs.AuthorIDGoogle
	case strings.HasPrefix(id, "databricks-gpt-"), strings.Contains(id, "gpt-oss"):
		return catalogs.AuthorIDOpenAI
	case strings.Contains(id, "llama"):
		return catalogs.AuthorIDMeta
	case strings.Contains(id, "qwen"), strings.Contains(id, "gte"):
		return catalogs.AuthorIDAlibabaQwen
	default:
		return catalogs.AuthorID(catalogs.ProviderIDDatabricks)
	}
}

// WorkspaceConfig is private Databricks workspace discovery configuration.
type WorkspaceConfig struct {
	Host               string                                `json:"-" yaml:"-"`
	Token              string                                `json:"-" yaml:"-"`
	WorkspaceID        string                                `json:"-" yaml:"-"`
	DefinitionByEntity map[string]catalogs.ModelDefinitionID `json:"-" yaml:"-"`
	Region             *catalogs.CloudRegion                 `json:"-" yaml:"-"`
}

// WorkspaceResult contains canonical records returned by one workspace observation.
type WorkspaceResult struct {
	Definitions []catalogs.ModelDefinition
	Offerings   []catalogs.ProviderOffering
}

// Catalog materializes workspace records in the single canonical catalog.
func (result WorkspaceResult) Catalog(provider catalogs.Provider) (*catalogs.Catalog, error) {
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

type endpointPage struct {
	Endpoints     []servingEndpoint `json:"endpoints"`
	NextPageToken string            `json:"next_page_token"`
}

type servingEndpoint struct {
	Name   string `json:"name"`
	Config struct {
		ServedEntities []servedEntity `json:"served_entities"`
		TrafficConfig  struct {
			Routes []trafficRoute `json:"routes"`
		} `json:"traffic_config"`
	} `json:"config"`
}

type servedEntity struct {
	Name          string `json:"name"`
	EntityName    string `json:"entity_name"`
	EntityVersion string `json:"entity_version"`
	ExternalModel *struct {
		Name     string `json:"name"`
		Provider string `json:"provider"`
		Task     string `json:"task"`
	} `json:"external_model"`
}

type trafficRoute struct {
	ServedModelName   string `json:"served_model_name"`
	TrafficPercentage int    `json:"traffic_percentage"`
}

// FetchWorkspace returns private serving endpoints as canonical contextual offerings.
func FetchWorkspace(ctx context.Context, config WorkspaceConfig) (WorkspaceResult, error) {
	base, err := validateWorkspaceConfig(config)
	if err != nil {
		return WorkspaceResult{}, err
	}
	client := &http.Client{Timeout: constants.DefaultTimeout}
	endpoints := make([]servingEndpoint, 0)
	next := ""
	for pageNumber := 0; pageNumber < maxWorkspacePages; pageNumber++ {
		page, fetchErr := fetchEndpointPage(ctx, client, base, config.Token, next)
		if fetchErr != nil {
			return WorkspaceResult{}, fetchErr
		}
		endpoints = append(endpoints, page.Endpoints...)
		if page.NextPageToken == "" {
			return workspaceOfferings(config, endpoints)
		}
		if page.NextPageToken == next {
			return WorkspaceResult{}, &errors.ValidationError{Field: "databricks.workspace.next_page_token", Value: next, Message: "cursor repeated"}
		}
		next = page.NextPageToken
	}
	return WorkspaceResult{}, &errors.ValidationError{Field: "databricks.workspace.pages", Value: maxWorkspacePages, Message: "page limit exceeded"}
}

func validateWorkspaceConfig(config WorkspaceConfig) (*url.URL, error) {
	if config.WorkspaceID == "" || config.Token == "" {
		return nil, &errors.ValidationError{Field: "databricks.workspace.config", Message: "workspace and token are required"}
	}
	base, err := url.Parse(config.Host)
	if err != nil {
		return nil, errors.WrapParse("url", "Databricks workspace host", err)
	}
	loopbackHTTP := base.Scheme == "http" && (base.Hostname() == "127.0.0.1" || base.Hostname() == "localhost")
	if (base.Scheme != "https" && !loopbackHTTP) || base.Host == "" {
		return nil, &errors.ValidationError{Field: "databricks.workspace.host", Value: config.Host, Message: "absolute HTTPS or loopback development URL is required"}
	}
	return base, nil
}

func fetchEndpointPage(ctx context.Context, client *http.Client, base *url.URL, token, pageToken string) (endpointPage, error) {
	requestURL := *base
	requestURL.Path = "/api/2.0/serving-endpoints"
	query := requestURL.Query()
	if pageToken != "" {
		query.Set("page_token", pageToken)
	}
	requestURL.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return endpointPage{}, errors.WrapResource("create", "Databricks workspace request", requestURL.String(), err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return endpointPage{}, &errors.APIError{Provider: string(catalogs.ProviderIDDatabricks), Endpoint: requestURL.String(), Message: "request failed", Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	var page endpointPage
	if err := transport.DecodeResponse(response, &page); err != nil {
		return endpointPage{}, &errors.APIError{Provider: string(catalogs.ProviderIDDatabricks), Endpoint: requestURL.String(), StatusCode: response.StatusCode, Message: "failed to decode response", Err: err}
	}
	if page.Endpoints == nil {
		return endpointPage{}, &errors.ValidationError{Field: "databricks.workspace.endpoints", Message: "required endpoint array is null"}
	}
	return page, nil
}

func workspaceOfferings(config WorkspaceConfig, endpoints []servingEndpoint) (WorkspaceResult, error) {
	offerings := make([]catalogs.ProviderOffering, 0)
	definitions := make(map[catalogs.ModelDefinitionID]catalogs.ModelDefinition)
	for _, endpoint := range endpoints {
		traffic := make(map[string]int)
		for _, route := range endpoint.Config.TrafficConfig.Routes {
			traffic[route.ServedModelName] = route.TrafficPercentage
		}
		for _, entity := range endpoint.Config.ServedEntities {
			identity, providerModelID := entityIdentity(entity)
			definitionID, found := config.DefinitionByEntity[identity]
			if !found {
				definitionID = catalogs.ModelDefinitionID(identity)
			}
			definitions[definitionID] = catalogs.ModelDefinition{ID: definitionID, Name: providerModelID}
			aliases := []string{endpoint.Name, entity.Name}
			if percentage, routed := traffic[entity.Name]; routed {
				aliases = append(aliases, "traffic="+strconv.Itoa(percentage)+"%")
			}
			offering := catalogs.ProviderOffering{
				ProviderID: catalogs.ProviderIDDatabricks, ProviderModelID: catalogs.ProviderModelID(providerModelID),
				DeploymentID: endpoint.Name + "/" + entity.Name, DefinitionID: definitionID, Aliases: aliases,
				Availability: catalogs.OfferingAvailabilityRestricted,
				Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}},
				Deployment:   catalogs.ProviderDeployment{Type: deploymentType(entity)}, Lifecycle: catalogs.OfferingLifecycleActive,
				Endpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeDatabricks, BaseURL: strings.TrimRight(config.Host, "/"), Path: "/serving-endpoints/" + endpoint.Name + "/invocations"},
			}
			if config.Region != nil {
				offering.Regions = []catalogs.CloudRegion{*config.Region}
			}
			if err := offering.Validate(); err != nil {
				return WorkspaceResult{}, err
			}
			offerings = append(offerings, offering)
		}
	}
	definitionList := make([]catalogs.ModelDefinition, 0, len(definitions))
	for _, definition := range definitions {
		definitionList = append(definitionList, definition)
	}
	slices.SortFunc(definitionList, func(left, right catalogs.ModelDefinition) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	slices.SortFunc(offerings, func(left, right catalogs.ProviderOffering) int {
		if compared := strings.Compare(left.DeploymentID, right.DeploymentID); compared != 0 {
			return compared
		}
		return strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID))
	})
	return WorkspaceResult{Definitions: definitionList, Offerings: offerings}, nil
}

func entityIdentity(entity servedEntity) (string, string) {
	if entity.ExternalModel != nil {
		return entity.ExternalModel.Provider + "/" + entity.ExternalModel.Name, entity.ExternalModel.Name
	}
	identity := entity.EntityName
	if entity.EntityVersion != "" {
		identity += "@" + entity.EntityVersion
	}
	return identity, identity
}

func deploymentType(entity servedEntity) string {
	if entity.ExternalModel != nil {
		return "external-model"
	}
	return "workspace-serving-endpoint"
}
