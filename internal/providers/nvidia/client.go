// Package nvidia implements NVIDIA API Catalog and private NIM inventory.
package nvidia

import (
	"context"
	"encoding/json"
	"net/url"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const defaultModelsURL = "https://integrate.api.nvidia.com/v1/models"

type modelList struct {
	Object string  `json:"object"`
	Data   []model `json:"data"`
}

type model struct {
	ID            string                           `json:"id"`
	Object        string                           `json:"object"`
	Created       int64                            `json:"created"`
	OwnedBy       string                           `json:"owned_by"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
}

func (m *model) UnmarshalJSON(data []byte) error {
	type alias model
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "data[]")
	if err != nil {
		return err
	}
	*m = model(decoded)
	m.UnknownFields = unknown
	return nil
}

// Client retrieves NVIDIA's public API Catalog inventory.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	transport *transport.Client
}

// NewClient creates an NVIDIA inventory client.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{provider: provider, transport: transport.New(provider)}
}

// IsAPIKeyRequired reports whether public inventory authentication is required.
func (c *Client) IsAPIKeyRequired() bool { return false }

// HasAPIKey reports whether a hosted-invocation key is resolved.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider != nil && c.provider.HasAPIKey()
}

// ListModels returns public catalog records as discoverable-only offerings.
// The list payload mixes API families and does not identify each invocation
// contract, so this method deliberately does not invent chat routes.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	configured, client := c.provider, c.transport
	c.mu.RUnlock()
	if configured == nil {
		return nil, &errors.ConfigError{Component: "nvidia", Message: "provider not configured"}
	}
	endpoint := configured.CatalogEndpointURL()
	if endpoint == "" {
		endpoint = defaultModelsURL
	}
	models, _, err := fetchModels(ctx, client, configured, endpoint)
	if err != nil {
		return nil, err
	}
	result := make([]catalogs.Model, 0, len(models))
	for _, source := range models {
		converted, convertErr := publicModel(source)
		if convertErr != nil {
			return nil, convertErr
		}
		result = append(result, converted)
	}
	slices.SortFunc(result, func(left, right catalogs.Model) int { return strings.Compare(left.ID, right.ID) })
	return result, nil
}

func fetchModels(ctx context.Context, client *transport.Client, provider *catalogs.Provider, endpoint string) ([]model, time.Time, error) {
	response, err := client.Get(ctx, endpoint, provider)
	if err != nil {
		return nil, time.Time{}, &errors.APIError{Provider: "nvidia", Endpoint: endpoint, Message: "request failed", Err: err}
	}
	var envelope modelList
	if err := transport.DecodeResponse(response, &envelope); err != nil {
		return nil, time.Time{}, &errors.APIError{Provider: "nvidia", Endpoint: endpoint, StatusCode: response.StatusCode, Message: "failed to decode response", Err: err}
	}
	if envelope.Object != "list" || envelope.Data == nil {
		return nil, time.Time{}, &errors.ValidationError{Field: "nvidia.models", Value: envelope.Object, Message: "requires object=list and a non-null data array"}
	}
	observedAt, _ := time.Parse(time.RFC1123, response.Header.Get("Date"))
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	return envelope.Data, observedAt.UTC(), nil
}

func publicModel(source model) (catalogs.Model, error) {
	if strings.TrimSpace(source.ID) == "" || strings.TrimSpace(source.OwnedBy) == "" {
		return catalogs.Model{}, &errors.ValidationError{Field: "nvidia.model", Value: source.ID, Message: "id and owned_by are required"}
	}
	fields := map[string]any{"owned_by": source.OwnedBy, "inventory_contract": "mixed-api-discovery"}
	if len(source.UnknownFields) > 0 {
		fields["unknown_fields"] = source.UnknownFields
	}
	access := &catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityDiscoverable, APIs: []catalogs.InvocationAPI{}}
	return catalogs.Model{
		ID: source.ID, Name: source.ID, Authors: []catalogs.Author{{ID: nvidiaAuthorID(source.OwnedBy), Name: source.OwnedBy}}, Status: catalogs.ModelStatusActive,
		InvocationAPIs: []catalogs.InvocationAPI{}, OfferingAccess: access,
		OfferingEndpoint:   catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOpenAI, BaseURL: "https://integrate.api.nvidia.com/v1"},
		OfferingDeployment: catalogs.ProviderDeployment{Type: "nvidia-hosted", Tier: "developer-catalog"},
		Extensions:         catalogs.SourceExtensions{"nvidia": {Fields: catalogs.NormalizeExtensionFields(fields)}},
	}, nil
}

// NIMInventoryConfig identifies one credential-scoped customer NIM deployment.
// BaseURL and scope are private operational inputs and never enter Catalog.
type NIMInventoryConfig struct {
	BaseURL          string
	AccountID        string
	DeploymentID     string
	Region           *catalogs.CloudRegion
	Aliases          []string
	DefinitionByName map[string]catalogs.ModelDefinitionID
}

// FetchCustomerNIM reads a private NIM's served names into customer inventory.
func FetchCustomerNIM(ctx context.Context, config NIMInventoryConfig) (catalogs.CustomerInventory, error) {
	if strings.TrimSpace(config.AccountID) == "" || strings.TrimSpace(config.DeploymentID) == "" || len(config.DefinitionByName) == 0 {
		return catalogs.CustomerInventory{}, &errors.ValidationError{Field: "nvidia.nim.config", Message: "account, deployment, and explicit definition mapping are required"}
	}
	parsed, err := url.Parse(config.BaseURL)
	if err != nil {
		return catalogs.CustomerInventory{}, errors.WrapParse("url", "NVIDIA NIM base URL", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return catalogs.CustomerInventory{}, &errors.ValidationError{Field: "nvidia.nim.base_url", Value: config.BaseURL, Message: "absolute URL is required"}
	}
	parsed.Path = path.Join(parsed.Path, "/v1/models")
	privateProvider := &catalogs.Provider{ID: catalogs.ProviderIDNVIDIA, Name: "Customer NIM", Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: parsed.String()}}}
	models, observedAt, err := fetchModels(ctx, transport.New(privateProvider), privateProvider, parsed.String())
	if err != nil {
		return catalogs.CustomerInventory{}, err
	}
	inventory := catalogs.CustomerInventory{ProviderID: catalogs.ProviderIDNVIDIA, Scope: catalogs.CustomerScope{AccountID: config.AccountID}, ObservedAt: observedAt}
	for _, served := range models {
		definitionID, found := config.DefinitionByName[served.ID]
		if !found {
			return catalogs.CustomerInventory{}, &errors.NotFoundError{Resource: "NVIDIA NIM definition mapping", ID: served.ID}
		}
		inventory.Deployments = append(inventory.Deployments, catalogs.CustomerDeployment{
			ID: config.DeploymentID + "/" + served.ID, DefinitionID: definitionID, ProviderModelID: catalogs.ProviderModelID(served.ID),
			Region: config.Region, Deployment: catalogs.ProviderDeployment{Type: "customer-hosted-nim"}, Endpoint: config.BaseURL,
			Aliases: append([]string(nil), config.Aliases...),
		})
	}
	if err := inventory.Validate(); err != nil {
		return catalogs.CustomerInventory{}, err
	}
	return inventory, nil
}

func nvidiaAuthorID(value string) catalogs.AuthorID {
	switch strings.ToLower(value) {
	case "deepseek-ai":
		return catalogs.AuthorIDDeepSeek
	case "meta":
		return catalogs.AuthorIDMeta
	case "minimaxai":
		return catalogs.AuthorIDMiniMax
	case "mistralai", "nv-mistralai":
		return catalogs.AuthorIDMistralAI
	case "moonshotai":
		return catalogs.AuthorID("moonshot-ai")
	case "qwen":
		return catalogs.AuthorIDAlibabaQwen
	case "z-ai":
		return catalogs.AuthorIDZhipuAI
	default:
		return catalogs.AuthorID(strings.ToLower(value))
	}
}
