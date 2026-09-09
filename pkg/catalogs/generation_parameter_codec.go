package catalogs

import (
	"encoding/json"
	"github.com/goccy/go-yaml"
)

// MarshalJSON retains explicit unknown generation controls.
func (g ModelGeneration) MarshalJSON() ([]byte, error) {
	type plain ModelGeneration
	data, err := json.Marshal(plain(g))
	if err != nil || g.unknownParameters == 0 {
		return data, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	for _, parameter := range generationParameters {
		if g.ParameterPresence(parameter) == ValueUnknown {
			raw[string(parameter)] = json.RawMessage("null")
		}
	}
	return json.Marshal(raw)
}

// UnmarshalJSON restores generation control presence and clears reused state.
func (g *ModelGeneration) UnmarshalJSON(data []byte) error {
	type plain ModelGeneration
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*g = ModelGeneration(decoded)
	for _, parameter := range generationParameters {
		if value, present := raw[string(parameter)]; present && isJSONNull(value) {
			g.SetParameterUnknown(parameter)
		}
	}
	return nil
}

// MarshalYAML retains unknown controls without numeric conversion.
func (g ModelGeneration) MarshalYAML() (any, error) {
	type plain ModelGeneration
	if g.unknownParameters == 0 {
		return plain(g), nil
	}
	data, err := yaml.Marshal(plain(g))
	if err != nil {
		return nil, err
	}
	var fields yaml.MapSlice
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, parameter := range generationParameters {
		if g.ParameterPresence(parameter) == ValueUnknown {
			fields = append(fields, yaml.MapItem{Key: string(parameter), Value: nil})
		}
	}
	return fields, nil
}

// UnmarshalYAML restores generation control presence and clears reused state.
func (g *ModelGeneration) UnmarshalYAML(unmarshal func(any) error) error {
	type plain ModelGeneration
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*g = ModelGeneration(decoded)
	for _, parameter := range generationParameters {
		if value, present := raw[string(parameter)]; present && value == nil {
			g.SetParameterUnknown(parameter)
		}
	}
	return nil
}
