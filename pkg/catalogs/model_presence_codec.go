package catalogs

import (
	"encoding/json"
	"reflect"

	"github.com/goccy/go-yaml"
)

// MarshalYAML renders the complete Boolean capability surface for the
// human-editable YAML workspace. Capabilities without an observed claim use
// the conservative false default, while explicitly unknown claims remain
// null. Immutable JSON generations retain the precise missing/unknown/known
// distinction.
func (f ModelFeatures) MarshalYAML() (any, error) {
	entries := make(yaml.MapSlice, 0, len(modelFeatures())+1)
	if f.Modalities.Input != nil || f.Modalities.Output != nil {
		entries = append(entries, yaml.MapItem{Key: "modalities", Value: f.Modalities})
	}
	for _, feature := range modelFeatures() {
		value, state := f.Support(feature)
		switch state {
		case ValueUnknown:
			entries = append(entries, yaml.MapItem{Key: string(feature), Value: nil})
		case ValueKnown:
			entries = append(entries, yaml.MapItem{Key: string(feature), Value: value})
		default:
			entries = append(entries, yaml.MapItem{Key: string(feature), Value: false})
		}
	}
	return entries, nil
}

// UnmarshalYAML restores per-capability presence from the human YAML record.
func (f *ModelFeatures) UnmarshalYAML(unmarshal func(any) error) error {
	type plain ModelFeatures
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*f = ModelFeatures(decoded)
	f.featurePresence = 0
	f.featureKnown = 0
	for _, feature := range modelFeatures() {
		value, present := raw[string(feature)]
		if !present {
			continue
		}
		state := ValueKnown
		if value == nil {
			state = ValueUnknown
		}
		f.markFeature(feature, state)
	}
	return nil
}

// MarshalJSON preserves feature presence in immutable catalog payloads.
func (f ModelFeatures) MarshalJSON() ([]byte, error) {
	values := make(map[string]any, len(modelFeatures())+1)
	if f.Modalities.Input != nil || f.Modalities.Output != nil {
		values["modalities"] = f.Modalities
	}
	for _, feature := range modelFeatures() {
		value, state := f.Support(feature)
		switch state {
		case ValueKnown:
			values[string(feature)] = value
		case ValueUnknown:
			values[string(feature)] = nil
		}
	}
	return json.Marshal(values)
}

// UnmarshalJSON restores feature presence from immutable catalog payloads.
func (f *ModelFeatures) UnmarshalJSON(data []byte) error {
	type plain ModelFeatures
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*f = ModelFeatures(decoded)
	f.featurePresence = 0
	f.featureKnown = 0
	for _, feature := range modelFeatures() {
		value, present := raw[string(feature)]
		if !present {
			continue
		}
		state := ValueKnown
		if isJSONNull(value) {
			state = ValueUnknown
		}
		f.markFeature(feature, state)
	}
	return nil
}

// MarshalYAML preserves explicit zero and unknown limits while omitting
// unobserved limits.
func (l ModelLimits) MarshalYAML() (any, error) {
	entries := make(yaml.MapSlice, 0, 3)
	for _, limit := range []ModelLimit{
		ModelLimitContextWindow,
		ModelLimitInputTokens,
		ModelLimitOutputTokens,
	} {
		value, state := l.Value(limit)
		switch state {
		case ValueKnown:
			entries = append(entries, yaml.MapItem{Key: string(limit), Value: value})
		case ValueUnknown:
			entries = append(entries, yaml.MapItem{Key: string(limit), Value: nil})
		}
	}
	return entries, nil
}

// UnmarshalYAML restores per-limit presence from the human YAML record.
func (l *ModelLimits) UnmarshalYAML(unmarshal func(any) error) error {
	type plain ModelLimits
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*l = ModelLimits(decoded)
	l.limitPresence = 0
	l.limitKnown = 0
	for _, limit := range []ModelLimit{
		ModelLimitContextWindow,
		ModelLimitInputTokens,
		ModelLimitOutputTokens,
	} {
		value, present := raw[string(limit)]
		if !present {
			continue
		}
		state := ValueKnown
		if value == nil {
			state = ValueUnknown
		}
		l.markLimit(limit, state)
	}
	return nil
}

// MarshalJSON preserves limit presence in immutable catalog payloads.
func (l ModelLimits) MarshalJSON() ([]byte, error) {
	values := make(map[string]any, 3)
	for _, limit := range []ModelLimit{
		ModelLimitContextWindow,
		ModelLimitInputTokens,
		ModelLimitOutputTokens,
	} {
		value, state := l.Value(limit)
		switch state {
		case ValueKnown:
			values[string(limit)] = value
		case ValueUnknown:
			values[string(limit)] = nil
		}
	}
	return json.Marshal(values)
}

// UnmarshalJSON restores limit presence from immutable catalog payloads.
func (l *ModelLimits) UnmarshalJSON(data []byte) error {
	type plain ModelLimits
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*l = ModelLimits(decoded)
	l.limitPresence = 0
	l.limitKnown = 0
	for _, limit := range []ModelLimit{
		ModelLimitContextWindow,
		ModelLimitInputTokens,
		ModelLimitOutputTokens,
	} {
		value, present := raw[string(limit)]
		if !present {
			continue
		}
		state := ValueKnown
		if isJSONNull(value) {
			state = ValueUnknown
		}
		l.markLimit(limit, state)
	}
	return nil
}

// MarshalYAML preserves explicit false and unknown open-weights claims.
func (m ModelMetadata) MarshalYAML() (any, error) {
	entries := yaml.MapSlice{{Key: "release_date", Value: m.ReleaseDate}}
	open, state := m.OpenWeightsValue()
	switch state {
	case ValueKnown:
		entries = append(entries, yaml.MapItem{Key: "open_weights", Value: open})
	case ValueUnknown:
		entries = append(entries, yaml.MapItem{Key: "open_weights", Value: nil})
	}
	if m.KnowledgeCutoff != nil {
		entries = append(entries, yaml.MapItem{Key: "knowledge_cutoff", Value: m.KnowledgeCutoff})
	}
	if len(m.Tags) > 0 {
		entries = append(entries, yaml.MapItem{Key: "tags", Value: m.Tags})
	}
	if m.Architecture != nil {
		entries = append(entries, yaml.MapItem{Key: "architecture", Value: m.Architecture})
	}
	return entries, nil
}

// UnmarshalYAML restores open-weights presence from the human YAML record.
func (m *ModelMetadata) UnmarshalYAML(unmarshal func(any) error) error {
	type plain ModelMetadata
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*m = ModelMetadata(decoded)
	if value, present := raw["open_weights"]; present {
		m.openWeightsPresence = ValueKnown
		if value == nil {
			m.openWeightsPresence = ValueUnknown
		}
	}
	return nil
}

// MarshalJSON preserves open-weights presence in immutable catalog payloads.
func (m ModelMetadata) MarshalJSON() ([]byte, error) {
	type plain ModelMetadata
	data, err := json.Marshal(plain(m))
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	open, state := m.OpenWeightsValue()
	switch state {
	case ValueKnown:
		raw["open_weights"], err = json.Marshal(open)
	case ValueUnknown:
		raw["open_weights"] = json.RawMessage("null")
	default:
		delete(raw, "open_weights")
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

// UnmarshalJSON restores open-weights presence from immutable catalog payloads.
func (m *ModelMetadata) UnmarshalJSON(data []byte) error {
	type plain ModelMetadata
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*m = ModelMetadata(decoded)
	if value, present := raw["open_weights"]; present {
		m.openWeightsPresence = ValueKnown
		if isJSONNull(value) {
			m.openWeightsPresence = ValueUnknown
		}
	}
	return nil
}

// MarshalYAML preserves an explicit empty or unknown description.
func (m Model) MarshalYAML() (any, error) {
	entries := yaml.MapSlice{
		{Key: "id", Value: m.ID},
	}
	if m.ModelRef != "" {
		entries = append(entries, yaml.MapItem{Key: "model", Value: m.ModelRef})
	}
	entries = append(entries, yaml.MapItem{Key: "name", Value: m.Name})
	if len(m.Authors) > 0 {
		entries = append(entries, yaml.MapItem{Key: "authors", Value: m.Authors})
	}
	description, state := m.DescriptionValue()
	switch state {
	case ValueKnown:
		entries = append(entries, yaml.MapItem{Key: "description", Value: description})
	case ValueUnknown:
		entries = append(entries, yaml.MapItem{Key: "description", Value: nil})
	}
	if m.Status != "" {
		entries = append(entries, yaml.MapItem{Key: "status", Value: m.Status})
	}
	for _, field := range []struct {
		key   string
		value any
	}{
		{key: "metadata", value: m.Metadata},
		{key: "lineage", value: m.Lineage},
		{key: "features", value: m.Features},
		{key: "attachments", value: m.Attachments},
		{key: "generation", value: m.Generation},
		{key: "reasoning", value: m.Reasoning},
		{key: "reasoning_tokens", value: m.ReasoningTokens},
		{key: "verbosity", value: m.Verbosity},
		{key: "tools", value: m.Tools},
		{key: "response", value: m.Delivery},
	} {
		if !isNilPresenceValue(field.value) {
			entries = append(entries, yaml.MapItem{Key: field.key, Value: field.value})
		}
	}
	if len(m.Modes) > 0 {
		entries = append(entries, yaml.MapItem{Key: "modes", Value: m.Modes})
	}
	for _, field := range []struct {
		key   string
		value any
	}{
		{key: "pricing", value: m.Pricing},
		{key: "limits", value: m.Limits},
	} {
		if !isNilPresenceValue(field.value) {
			entries = append(entries, yaml.MapItem{Key: field.key, Value: field.value})
		}
	}
	if len(m.Extensions) > 0 {
		entries = append(entries, yaml.MapItem{Key: "extensions", Value: m.Extensions})
	}
	entries = append(entries,
		yaml.MapItem{Key: "created_at", Value: m.CreatedAt},
		yaml.MapItem{Key: "updated_at", Value: m.UpdatedAt},
	)
	return entries, nil
}

func isNilPresenceValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}

// UnmarshalYAML restores description presence from the human YAML record.
func (m *Model) UnmarshalYAML(unmarshal func(any) error) error {
	type plain Model
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*m = Model(decoded)
	if value, present := raw["description"]; present {
		m.descriptionPresence = ValueKnown
		if value == nil {
			m.descriptionPresence = ValueUnknown
		}
	}
	return nil
}

// MarshalJSON preserves description presence in immutable catalog payloads.
func (m Model) MarshalJSON() ([]byte, error) {
	type plain Model
	data, err := json.Marshal(plain(m))
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	description, state := m.DescriptionValue()
	switch state {
	case ValueKnown:
		raw["description"], err = json.Marshal(description)
	case ValueUnknown:
		raw["description"] = json.RawMessage("null")
	default:
		delete(raw, "description")
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

// UnmarshalJSON restores description presence from immutable catalog payloads.
func (m *Model) UnmarshalJSON(data []byte) error {
	type plain Model
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*m = Model(decoded)
	if value, present := raw["description"]; present {
		m.descriptionPresence = ValueKnown
		if isJSONNull(value) {
			m.descriptionPresence = ValueUnknown
		}
	}
	return nil
}
