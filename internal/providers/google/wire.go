package google

import (
	"encoding/json"

	"github.com/agentstation/starmap/internal/sourcepayload"
	"github.com/agentstation/starmap/internal/constants"
)

type aiStudioModelsResponse struct {
	Models        []aiStudioModel                  `json:"models"`
	NextPageToken string                           `json:"nextPageToken,omitempty"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
	RecordReport  sourcepayload.RecordReport       `json:"-"`
}

func (r *aiStudioModelsResponse) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Models        json.RawMessage `json:"models"`
		NextPageToken string          `json:"nextPageToken,omitempty"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "$")
	if err != nil {
		return err
	}
	var records []aiStudioModel
	var report sourcepayload.RecordReport
	if len(decoded.Models) != 0 && string(decoded.Models) != "null" {
		records, report, err = sourcepayload.DecodeJSONArray[aiStudioModel](
			decoded.Models,
			"models",
			constants.MaxCatalogModels,
		)
		if err != nil {
			return err
		}
	}
	*r = aiStudioModelsResponse{
		Models: records, NextPageToken: decoded.NextPageToken,
		UnknownFields: unknown, RecordReport: report,
	}
	return nil
}

type aiStudioModel struct {
	Name                       string                           `json:"name"`
	DisplayName                string                           `json:"displayName"`
	Description                string                           `json:"description"`
	Version                    string                           `json:"version,omitempty"`
	InputTokenLimit            int32                            `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int32                            `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string                         `json:"supportedGenerationMethods,omitempty"`
	Temperature                *float64                         `json:"temperature,omitempty"`
	MaxTemperature             *float64                         `json:"maxTemperature,omitempty"`
	TopP                       *float64                         `json:"topP,omitempty"`
	TopK                       *int32                           `json:"topK,omitempty"`
	Thinking                   *bool                            `json:"thinking,omitempty"`
	UnknownFields              []sourcepayload.UnknownJSONField `json:"-"`
}

func (m *aiStudioModel) UnmarshalJSON(data []byte) error {
	type modelAlias aiStudioModel
	var decoded modelAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "models[]")
	if err != nil {
		return err
	}
	*m = aiStudioModel(decoded)
	m.UnknownFields = unknown
	return nil
}

// Client implements the catalogs.Client interface with dynamic configuration
// for both Google AI Studio and Vertex AI.
