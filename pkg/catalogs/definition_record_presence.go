package catalogs

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
)

var definitionCapabilityRecords = []ModelRecord{
	ModelRecordFeatures, ModelRecordAttachments, ModelRecordGeneration,
	ModelRecordReasoning, ModelRecordReasoningTokens, ModelRecordVerbosity,
	ModelRecordTools, ModelRecordDelivery,
}

// RecordPresence reports an intrinsic capability record's observed presence.
// Records outside Capabilities return ValueMissing.
func (c ModelDefinitionCapabilities) RecordPresence(record ModelRecord) ValuePresence {
	var known bool
	switch record {
	case ModelRecordFeatures:
		known = c.Features != nil
	case ModelRecordAttachments:
		known = c.Attachments != nil
	case ModelRecordGeneration:
		known = c.Generation != nil
	case ModelRecordReasoning:
		known = c.Reasoning != nil
	case ModelRecordReasoningTokens:
		known = c.ReasoningTokens != nil
	case ModelRecordVerbosity:
		known = c.Verbosity != nil
	case ModelRecordTools:
		known = c.Tools != nil
	case ModelRecordDelivery:
		known = c.Delivery != nil
	default:
		return ValueMissing
	}
	if known {
		return ValueKnown
	}
	if c.recordUnknown&modelRecordBits[record] != 0 {
		return ValueUnknown
	}
	return ValueMissing
}

func definitionCapabilityMask() uint16 {
	var mask uint16
	for _, record := range definitionCapabilityRecords {
		mask |= modelRecordBits[record]
	}
	return mask
}

func definitionCapabilityKey(record ModelRecord) string {
	if record == ModelRecordDelivery {
		return "delivery"
	}
	return string(record)
}

// MarshalJSON retains explicit unknown records in the definition read view.
func (c ModelDefinitionCapabilities) MarshalJSON() ([]byte, error) {
	type plain ModelDefinitionCapabilities
	data, err := json.Marshal(plain(c))
	if err != nil || c.recordUnknown == 0 {
		return data, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	for _, record := range definitionCapabilityRecords {
		if c.RecordPresence(record) == ValueUnknown {
			raw[definitionCapabilityKey(record)] = json.RawMessage("null")
		}
	}
	return json.Marshal(raw)
}

// UnmarshalJSON restores capability presence and clears earlier claims.
func (c *ModelDefinitionCapabilities) UnmarshalJSON(data []byte) error {
	type plain ModelDefinitionCapabilities
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = ModelDefinitionCapabilities(decoded)
	for _, record := range definitionCapabilityRecords {
		if value, present := raw[definitionCapabilityKey(record)]; present && isJSONNull(value) {
			c.recordUnknown |= modelRecordBits[record]
		}
	}
	return nil
}

// MarshalYAML retains explicit unknown records without numeric conversion.
func (c ModelDefinitionCapabilities) MarshalYAML() (any, error) {
	type plain ModelDefinitionCapabilities
	if c.recordUnknown == 0 {
		return plain(c), nil
	}
	data, err := yaml.Marshal(plain(c))
	if err != nil {
		return nil, err
	}
	var fields yaml.MapSlice
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, record := range definitionCapabilityRecords {
		if c.RecordPresence(record) == ValueUnknown {
			fields = append(fields, yaml.MapItem{Key: definitionCapabilityKey(record), Value: nil})
		}
	}
	return fields, nil
}

// UnmarshalYAML restores capability presence and clears earlier claims.
func (c *ModelDefinitionCapabilities) UnmarshalYAML(data []byte) error {
	type plain ModelDefinitionCapabilities
	var decoded plain
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = ModelDefinitionCapabilities(decoded)
	for _, record := range definitionCapabilityRecords {
		if value, present := raw[definitionCapabilityKey(record)]; present && value == nil {
			c.recordUnknown |= modelRecordBits[record]
		}
	}
	return nil
}
