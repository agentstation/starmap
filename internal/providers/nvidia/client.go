// Package nvidia implements NVIDIA API Catalog inventory.
package nvidia

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

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

// Client retrieves NVIDIA OpenAI-compatible model inventory.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	endpoint  string
	transport *transport.Client
}

// NewClient creates an NVIDIA inventory client.
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	return &Client{provider: &provider, endpoint: source.EndpointURL(), transport: transport.New(source.Auth())}
}

// ListModels returns normalized inventory without inventing invocation policy.
// The selected logical source's offering defaults distinguish the mixed public
// API Catalog from a caller-configured NIM deployment.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	configured, client, endpoint := c.provider, c.transport, c.endpoint
	c.mu.RUnlock()
	if configured == nil {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDNVIDIA), Message: "provider not configured"}
	}
	if endpoint == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDNVIDIA), Message: "catalog endpoint is required"}
	}
	models, _, err := fetchModels(ctx, client, endpoint)
	if err != nil {
		return nil, err
	}
	return normalizePublicModels(models)
}

// DecodeModels validates and normalizes one captured NVIDIA public catalog
// response through the same schema and conversion path as live acquisition.
func (c *Client) DecodeModels(payload []byte) ([]catalogs.Model, error) {
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return nil, err
	}
	var envelope modelList
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, errors.WrapParse("json", "NVIDIA models response fixture", err)
	}
	if envelope.Object != "list" || envelope.Data == nil {
		return nil, &errors.ValidationError{Field: "nvidia.models", Value: envelope.Object, Message: "requires object=list and a non-null data array"}
	}
	return normalizePublicModels(envelope.Data)
}

func normalizePublicModels(models []model) ([]catalogs.Model, error) {
	result := make([]catalogs.Model, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, source := range models {
		converted, convertErr := normalizeModel(source)
		if convertErr != nil {
			return nil, convertErr
		}
		if _, found := seen[converted.ID]; found {
			return nil, &errors.ConflictError{Resource: "nvidia model", Actual: converted.ID, Message: "duplicate model identity"}
		}
		seen[converted.ID] = struct{}{}
		result = append(result, converted)
	}
	slices.SortFunc(result, func(left, right catalogs.Model) int { return strings.Compare(left.ID, right.ID) })
	return result, nil
}

func fetchModels(ctx context.Context, client *transport.Client, endpoint string) ([]model, time.Time, error) {
	response, err := client.Get(ctx, endpoint)
	if err != nil {
		return nil, time.Time{}, &errors.APIError{Provider: string(catalogs.ProviderIDNVIDIA), Endpoint: endpoint, Message: "request failed", Err: err}
	}
	var envelope modelList
	if err := transport.DecodeResponse(response, &envelope); err != nil {
		return nil, time.Time{}, &errors.APIError{Provider: string(catalogs.ProviderIDNVIDIA), Endpoint: endpoint, StatusCode: response.StatusCode, Message: "failed to decode response", Err: err}
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

func normalizeModel(source model) (catalogs.Model, error) {
	if strings.TrimSpace(source.ID) == "" || strings.TrimSpace(source.OwnedBy) == "" {
		return catalogs.Model{}, &errors.ValidationError{Field: "nvidia.model", Value: source.ID, Message: "id and owned_by are required"}
	}
	fields := map[string]any{"owned_by": source.OwnedBy, "inventory_contract": "mixed-api-discovery"}
	if len(source.UnknownFields) > 0 {
		fields["unknown_fields"] = source.UnknownFields
	}
	return catalogs.Model{
		ID: source.ID, Name: source.ID, Authors: []catalogs.Author{{ID: nvidiaAuthorID(source.OwnedBy), Name: source.OwnedBy}}, Status: catalogs.ModelStatusActive,
		Extensions: catalogs.SourceExtensions{string(catalogs.ProviderIDNVIDIA): {Fields: catalogs.NormalizeExtensionFields(fields)}},
	}, nil
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
