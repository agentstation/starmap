// Package snowflake implements Snowflake Cortex session model inventory.
package snowflake

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type sessionModel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Model      string `json:"model"`
	Status     string `json:"status"`
	Deprecated bool   `json:"deprecated"`
}

type modelEnvelope struct{ Models []sessionModel }

func (e *modelEnvelope) UnmarshalJSON(data []byte) error {
	var object struct {
		Models json.RawMessage `json:"models"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &object); err == nil && (object.Models != nil || object.Data != nil) {
		raw := object.Models
		if raw == nil {
			raw = object.Data
		}
		return decodeModels(raw, &e.Models)
	}
	return decodeModels(data, &e.Models)
}

func decodeModels(data []byte, target *[]sessionModel) error {
	records := make([]sessionModel, 0)
	if err := json.Unmarshal(data, &records); err == nil {
		*target = records
		return nil
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return err
	}
	records = make([]sessionModel, 0, len(names))
	for _, name := range names {
		records = append(records, sessionModel{Name: name})
	}
	*target = records
	return nil
}

// Client retrieves models eligible in the configured Snowflake session.
type Client struct {
	mu          sync.RWMutex
	provider    *catalogs.Provider
	endpoint    string
	region      string
	crossRegion string
	transport   *transport.Client
}

// NewClient creates a Snowflake Cortex client.
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	region, _ := source.Binding("region")
	crossRegion, _ := source.Option("cross_region")
	return &Client{provider: &provider, endpoint: source.EndpointURL(), region: region, crossRegion: crossRegion, transport: transport.New(source.Auth())}
}

// ListModels returns models eligible for the configured account session.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	configured, client, endpoint, region, crossRegion := c.provider, c.transport, c.endpoint, c.region, c.crossRegion
	c.mu.RUnlock()
	if configured == nil || endpoint == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDSnowflake), Message: "account URL is required"}
	}
	response, err := client.Get(ctx, endpoint)
	if err != nil {
		return nil, &errors.APIError{Provider: string(catalogs.ProviderIDSnowflake), Endpoint: endpoint, Message: "request failed", Err: err}
	}
	var envelope modelEnvelope
	if err := transport.DecodeResponse(response, &envelope); err != nil {
		return nil, &errors.APIError{Provider: string(catalogs.ProviderIDSnowflake), Endpoint: endpoint, StatusCode: response.StatusCode, Message: "failed to decode response", Err: err}
	}
	if envelope.Models == nil {
		return nil, &errors.ValidationError{Field: "snowflake.models", Message: "required model array is null"}
	}
	result := make([]catalogs.Model, 0, len(envelope.Models))
	for _, source := range envelope.Models {
		converted := convertModel(source, region, crossRegion)
		if converted.ID == "" {
			return nil, &errors.ValidationError{Field: "snowflake.model", Value: source, Message: "model name is required"}
		}
		result = append(result, converted)
	}
	slices.SortFunc(result, func(left, right catalogs.Model) int { return strings.Compare(left.ID, right.ID) })
	return result, nil
}

func convertModel(source sessionModel, region, crossRegion string) catalogs.Model {
	id := source.Name
	if id == "" {
		id = source.ID
	}
	if id == "" {
		id = source.Model
	}
	status := catalogs.ModelStatusActive
	if source.Deprecated || strings.EqualFold(source.Status, "deprecated") {
		status = catalogs.ModelStatusDeprecated
	}
	result := catalogs.Model{
		ID: id, Name: id, Authors: []catalogs.Author{{ID: snowflakeAuthor(id), Name: snowflakeAuthor(id).String()}}, Status: status,
		InvocationAPIs:     []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions, catalogs.InvocationAPISnowflakeComplete},
		OfferingEndpoint:   catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeSnowflake},
		OfferingDeployment: catalogs.ProviderDeployment{Type: "snowflake-hosted", Tier: "cortex-inference"},
	}
	region = strings.TrimSpace(region)
	if region != "" {
		result.OfferingRegions = []catalogs.CloudRegion{{ID: region}}
	}
	crossRegion = strings.TrimSpace(crossRegion)
	if crossRegion != "" && !strings.EqualFold(crossRegion, "DISABLED") {
		profile := &catalogs.CrossRegionInferenceProfile{ID: crossRegion, Scope: "snowflake-cross-region", DestinationRegions: []string{strings.ToLower(crossRegion)}}
		if region != "" {
			profile.SourceRegions = []string{region}
		}
		result.OfferingInferenceProfile = profile
	}
	return result
}

func snowflakeAuthor(id string) catalogs.AuthorID {
	switch {
	case strings.HasPrefix(id, "claude-"):
		return catalogs.AuthorIDAnthropic
	case strings.HasPrefix(id, "openai-"):
		return catalogs.AuthorIDOpenAI
	case strings.HasPrefix(id, "grok-"):
		return catalogs.AuthorIDXAI
	case strings.HasPrefix(id, "gemini-"):
		return catalogs.AuthorIDGoogle
	case strings.HasPrefix(id, "deepseek-"):
		return catalogs.AuthorIDDeepSeek
	case strings.HasPrefix(id, "llama"):
		return catalogs.AuthorIDMeta
	case strings.HasPrefix(id, "mistral-"):
		return catalogs.AuthorIDMistralAI
	default:
		return catalogs.AuthorID(catalogs.ProviderIDSnowflake)
	}
}
