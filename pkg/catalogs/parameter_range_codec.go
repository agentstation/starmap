package catalogs

import (
	"encoding/json"
	"github.com/goccy/go-yaml"
)

// MarshalJSON preserves observed range fields and legacy field order.
func (r FloatRange) MarshalJSON() ([]byte, error) {
	type plain FloatRange
	if r.absentFields|r.unknownFields == 0 {
		return json.Marshal(plain(r))
	}
	return encodeRangeJSON(r.Value)
}

// UnmarshalJSON restores range field presence and clears reused state.
func (r *FloatRange) UnmarshalJSON(data []byte) error {
	type plain FloatRange
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = FloatRange(decoded)
	for _, field := range []RangeField{RangeMinimum, RangeMaximum, RangeDefault} {
		value, present := raw[string(field)]
		if !present {
			r.UnsetValue(field)
		} else if isJSONNull(value) {
			r.SetValueUnknown(field)
		}
	}
	return nil
}

// MarshalYAML preserves observed range fields without numeric conversion.
func (r FloatRange) MarshalYAML() (any, error) {
	type plain FloatRange
	if r.absentFields|r.unknownFields == 0 {
		return plain(r), nil
	}
	return encodeRangeYAML(r.Value), nil
}

// UnmarshalYAML restores range field presence and clears reused state.
func (r *FloatRange) UnmarshalYAML(unmarshal func(any) error) error {
	type plain FloatRange
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*r = FloatRange(decoded)
	for _, field := range []RangeField{RangeMinimum, RangeMaximum, RangeDefault} {
		value, present := raw[string(field)]
		if !present {
			r.UnsetValue(field)
		} else if value == nil {
			r.SetValueUnknown(field)
		}
	}
	return nil
}

// MarshalJSON preserves observed range fields and legacy field order.
func (r IntRange) MarshalJSON() ([]byte, error) {
	type plain IntRange
	if r.absentFields|r.unknownFields == 0 {
		return json.Marshal(plain(r))
	}
	return encodeRangeJSON(r.Value)
}

// UnmarshalJSON restores range field presence and clears reused state.
func (r *IntRange) UnmarshalJSON(data []byte) error {
	type plain IntRange
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = IntRange(decoded)
	for _, field := range []RangeField{RangeMinimum, RangeMaximum, RangeDefault} {
		value, present := raw[string(field)]
		if !present {
			r.UnsetValue(field)
		} else if isJSONNull(value) {
			r.SetValueUnknown(field)
		}
	}
	return nil
}

// MarshalYAML preserves observed range fields without numeric conversion.
func (r IntRange) MarshalYAML() (any, error) {
	type plain IntRange
	if r.absentFields|r.unknownFields == 0 {
		return plain(r), nil
	}
	return encodeRangeYAML(r.Value), nil
}

// UnmarshalYAML restores range field presence and clears reused state.
func (r *IntRange) UnmarshalYAML(unmarshal func(any) error) error {
	type plain IntRange
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*r = IntRange(decoded)
	for _, field := range []RangeField{RangeMinimum, RangeMaximum, RangeDefault} {
		value, present := raw[string(field)]
		if !present {
			r.UnsetValue(field)
		} else if value == nil {
			r.SetValueUnknown(field)
		}
	}
	return nil
}

func encodeRangeJSON[T int | float64](read func(RangeField) (T, ValuePresence)) ([]byte, error) {
	result := []byte{'{'}
	for _, field := range []RangeField{RangeMinimum, RangeMaximum, RangeDefault} {
		value, state := read(field)
		if state == ValueMissing {
			continue
		}
		if len(result) > 1 {
			result = append(result, ',')
		}
		result = append(result, '"')
		result = append(result, string(field)...)
		result = append(result, '"', ':')
		if state == ValueUnknown {
			result = append(result, "null"...)
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		result = append(result, encoded...)
	}
	return append(result, '}'), nil
}

func encodeRangeYAML[T int | float64](read func(RangeField) (T, ValuePresence)) yaml.MapSlice {
	entries := make(yaml.MapSlice, 0, 3)
	for _, field := range []RangeField{RangeMinimum, RangeMaximum, RangeDefault} {
		value, state := read(field)
		if state == ValueMissing {
			continue
		}
		var encoded any = value
		if state == ValueUnknown {
			encoded = nil
		}
		entries = append(entries, yaml.MapItem{Key: string(field), Value: encoded})
	}
	return entries
}
