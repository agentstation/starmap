package catalogs

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
)

// MarshalJSON preserves Boolean presence and the legacy field order.
func (a ModelArchitecture) MarshalJSON() ([]byte, error) {
	type wireArchitecture struct {
		ParameterCount string           `json:"parameter_count,omitempty"`
		Type           ArchitectureType `json:"type,omitempty"`
		Tokenizer      Tokenizer        `json:"tokenizer,omitempty"`
		Quantization   Quantization     `json:"quantization,omitempty"`
		Quantized      any              `json:"quantized,omitempty"`
		FineTuned      any              `json:"fine_tuned,omitempty"`
		BaseModel      *string          `json:"base_model,omitempty"`
	}
	return json.Marshal(wireArchitecture{
		ParameterCount: a.ParameterCount, Type: a.Type, Tokenizer: a.Tokenizer,
		Quantization: a.Quantization, Quantized: architectureJSONClaim(a.QuantizedValue()),
		FineTuned: architectureJSONClaim(a.FineTunedValue()), BaseModel: a.BaseModel,
	})
}

func architectureJSONClaim(value bool, state ValuePresence) any {
	switch state {
	case ValueKnown:
		return value
	case ValueUnknown:
		var unknown *bool
		return unknown
	default:
		return nil
	}
}

// UnmarshalJSON restores architecture claim presence.
func (a *ModelArchitecture) UnmarshalJSON(data []byte) error {
	type plain ModelArchitecture
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*a = ModelArchitecture(decoded)
	for key, state := range map[string]*ValuePresence{"quantized": &a.quantizedPresence, "fine_tuned": &a.fineTunedPresence} {
		if value, present := fields[key]; present {
			*state = ValueKnown
			if isJSONNull(value) {
				*state = ValueUnknown
			}
		}
	}
	return nil
}

// MarshalYAML preserves architecture claim presence in the authoring workspace.
func (a ModelArchitecture) MarshalYAML() (any, error) {
	entries := yaml.MapSlice{}
	if a.ParameterCount != "" {
		entries = append(entries, yaml.MapItem{Key: "parameter_count", Value: a.ParameterCount})
	}
	if a.Type != "" {
		entries = append(entries, yaml.MapItem{Key: "type", Value: a.Type})
	}
	if a.Tokenizer != "" {
		entries = append(entries, yaml.MapItem{Key: "tokenizer", Value: a.Tokenizer})
	}
	if a.Quantization != "" {
		entries = append(entries, yaml.MapItem{Key: "quantization", Value: a.Quantization})
	}
	for _, key := range []string{"quantized", "fine_tuned"} {
		value, state := a.QuantizedValue()
		if key == "fine_tuned" {
			value, state = a.FineTunedValue()
		}
		switch state {
		case ValueKnown:
			entries = append(entries, yaml.MapItem{Key: key, Value: value})
		case ValueUnknown:
			entries = append(entries, yaml.MapItem{Key: key, Value: nil})
		}
	}
	if a.BaseModel != nil {
		entries = append(entries, yaml.MapItem{Key: "base_model", Value: a.BaseModel})
	}
	return entries, nil
}

// UnmarshalYAML restores architecture claim presence from an authoring record.
func (a *ModelArchitecture) UnmarshalYAML(unmarshal func(any) error) error {
	type plain ModelArchitecture
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var fields map[string]any
	if err := unmarshal(&fields); err != nil {
		return err
	}
	*a = ModelArchitecture(decoded)
	for key, state := range map[string]*ValuePresence{"quantized": &a.quantizedPresence, "fine_tuned": &a.fineTunedPresence} {
		if value, present := fields[key]; present {
			*state = ValueKnown
			if value == nil {
				*state = ValueUnknown
			}
		}
	}
	return nil
}
