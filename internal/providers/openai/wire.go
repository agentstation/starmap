package openai

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

// Response is the bounded wire representation of an OpenAI-compatible model list.
type Response struct {
	Object        string                           `json:"object"`
	Data          []Model                          `json:"data"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
	RecordReport  sourcepayload.RecordReport       `json:"-"`
}

// UnmarshalJSON retains fingerprints for additive top-level fields.
func (r *Response) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Object string          `json:"object"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "$")
	if err != nil {
		return err
	}
	var records []Model
	var report sourcepayload.RecordReport
	if len(decoded.Data) != 0 && string(decoded.Data) != "null" {
		records, report, err = sourcepayload.DecodeJSONArray[Model](
			decoded.Data,
			"data",
			constants.MaxCatalogModels,
		)
		if err != nil {
			return err
		}
	}
	*r = Response{
		Object: decoded.Object, Data: records, UnknownFields: unknown,
		RecordReport: report,
	}
	return nil
}

// Model represents a model in the OpenAI API response.
type Model struct {
	ID      string  `json:"id"`
	Object  string  `json:"object"`
	OwnedBy string  `json:"owned_by"`
	Created int64   `json:"created"`
	Root    string  `json:"root,omitempty"`
	Parent  *string `json:"parent,omitempty"`
	Name    string  `json:"name,omitempty"`
	// Dynamic fields from provider-specific responses
	MaxModelLen                 *int64   `json:"max_model_len,omitempty"`
	ContextWindow               *int64   `json:"context_window,omitempty"`
	ContextLength               *int64   `json:"context_length,omitempty"`
	MaxCompletionTokens         *int64   `json:"max_completion_tokens,omitempty"`
	MaxOutputLength             *int64   `json:"max_output_length,omitempty"`
	InputTokenLimit             *int64   `json:"input_token_limit,omitempty"`
	OutputTokenLimit            *int64   `json:"output_token_limit,omitempty"`
	InputModalities             []string `json:"input_modalities,omitempty"`
	OutputModalities            []string `json:"output_modalities,omitempty"`
	SupportedFeatures           []string `json:"supported_features,omitempty"`
	SupportedSamplingParameters []string `json:"supported_sampling_parameters,omitempty"`
	// Provider-specific fields
	Active             *bool                            `json:"active,omitempty"`               // Groq-specific
	PublicApps         any                              `json:"public_apps,omitempty"`          // Groq-specific
	HuggingFaceID      string                           `json:"hugging_face_id,omitempty"`      // Groq/aggregator-specific
	Pricing            *ModelPricing                    `json:"pricing,omitempty"`              // Provider-specific pricing
	Kind               string                           `json:"kind,omitempty"`                 // Fireworks-specific
	SupportsChat       *bool                            `json:"supports_chat,omitempty"`        // Fireworks-specific
	SupportsTools      *bool                            `json:"supports_tools,omitempty"`       // Fireworks-specific
	SupportsImageInput *bool                            `json:"supports_image_input,omitempty"` // Fireworks-specific
	SupportsImageIn    *bool                            `json:"supports_image_in,omitempty"`    // Moonshot-specific
	SupportsVideoIn    *bool                            `json:"supports_video_in,omitempty"`    // Moonshot-specific
	SupportsReasoning  *bool                            `json:"supports_reasoning,omitempty"`   // Moonshot-specific
	Permission         []ModelPermission                `json:"permission,omitempty"`           // Moonshot/OpenAI permission metadata
	Metadata           *ModelMetadata                   `json:"metadata,omitempty"`
	UnknownFields      []sourcepayload.UnknownJSONField `json:"-"`
}

// UnmarshalJSON retains fingerprints for additive model fields.
func (m *Model) UnmarshalJSON(data []byte) error {
	type modelAlias Model
	var decoded modelAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "data[]")
	if err != nil {
		return err
	}
	*m = Model(decoded)
	m.UnknownFields = unknown
	return nil
}

// ModelMetadata represents nested metadata returned by some OpenAI-compatible providers.
type ModelMetadata struct {
	Description       string                `json:"description,omitempty"`
	ContextLength     *int64                `json:"context_length,omitempty"`
	MaxTokens         *int64                `json:"max_tokens,omitempty"`
	Tags              []string              `json:"tags,omitempty"`
	DefaultWidth      *int64                `json:"default_width,omitempty"`
	DefaultHeight     *int64                `json:"default_height,omitempty"`
	DefaultIterations *int64                `json:"default_iterations,omitempty"`
	Pricing           *ModelMetadataPricing `json:"pricing,omitempty"`
}

// ModelPricing represents top-level provider pricing returned by OpenAI-compatible APIs.
type ModelPricing struct {
	Request        *float64 `json:"request,omitempty"`
	Prompt         *float64 `json:"prompt,omitempty"`
	Completion     *float64 `json:"completion,omitempty"`
	InputCacheRead *float64 `json:"input_cache_read,omitempty"`
	Image          *float64 `json:"image,omitempty"`
}

// UnmarshalJSON accepts provider pricing values as either numbers or numeric strings.
func (p *ModelPricing) UnmarshalJSON(data []byte) error {
	var raw struct {
		Request        json.RawMessage `json:"request"`
		Prompt         json.RawMessage `json:"prompt"`
		Completion     json.RawMessage `json:"completion"`
		InputCacheRead json.RawMessage `json:"input_cache_read"`
		Image          json.RawMessage `json:"image"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var err error
	if p.Request, err = parseOptionalFloat(raw.Request, "request"); err != nil {
		return err
	}
	if p.Prompt, err = parseOptionalFloat(raw.Prompt, "prompt"); err != nil {
		return err
	}
	if p.Completion, err = parseOptionalFloat(raw.Completion, "completion"); err != nil {
		return err
	}
	if p.InputCacheRead, err = parseOptionalFloat(raw.InputCacheRead, "input_cache_read"); err != nil {
		return err
	}
	if p.Image, err = parseOptionalFloat(raw.Image, "image"); err != nil {
		return err
	}
	return nil
}

func parseOptionalFloat(raw json.RawMessage, field string) (*float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var numeric float64
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return &numeric, nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, fmt.Errorf("parse pricing.%s: %w", field, err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, fmt.Errorf("parse pricing.%s: %w", field, err)
	}
	return &parsed, nil
}

// ModelMetadataPricing represents nested provider pricing returned in metadata.
type ModelMetadataPricing struct {
	InputTokens     *float64 `json:"input_tokens,omitempty"`
	OutputTokens    *float64 `json:"output_tokens,omitempty"`
	CacheReadTokens *float64 `json:"cache_read_tokens,omitempty"`
	PerImageUnit    *float64 `json:"per_image_unit,omitempty"`
	InputCharacters *float64 `json:"input_characters,omitempty"`
	InputSeconds    *float64 `json:"input_seconds,omitempty"`
	OutputSeconds   *float64 `json:"output_seconds,omitempty"`
}

// ModelPermission represents provider permission metadata.
type ModelPermission struct {
	ID           string  `json:"id,omitempty"`
	Object       string  `json:"object,omitempty"`
	Created      int64   `json:"created,omitempty"`
	Organization string  `json:"organization,omitempty"`
	Group        *string `json:"group,omitempty"`
}

// Client implements the catalogs.Client interface with dynamic configuration.
