package catalogs

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
)

var offeringRecords = []ModelRecord{ModelRecordPricing, ModelRecordLimits, ModelRecordDeprecatedAt, ModelRecordRetiresAt}

// RecordPresence reports an optional provider service record's observed presence.
// Intrinsic capability records return ValueMissing.
func (o ProviderOffering) RecordPresence(record ModelRecord) ValuePresence {
	var known bool
	switch record {
	case ModelRecordPricing:
		known = o.Pricing != nil
	case ModelRecordLimits:
		known = o.Limits != nil
	case ModelRecordDeprecatedAt:
		known = o.DeprecatedAt != nil
	case ModelRecordRetiresAt:
		known = o.RetiresAt != nil
	default:
		return ValueMissing
	}
	if known {
		return ValueKnown
	}
	if o.recordUnknown&modelRecordBits[record] != 0 {
		return ValueUnknown
	}
	return ValueMissing
}

func offeringRecordMask() uint16 {
	var mask uint16
	for _, record := range offeringRecords {
		mask |= modelRecordBits[record]
	}
	return mask
}

// MarshalJSON retains explicit unknown records in the provider offering read view.
func (o ProviderOffering) MarshalJSON() ([]byte, error) {
	type plain ProviderOffering
	data, err := json.Marshal(plain(o))
	if err != nil || o.recordUnknown == 0 {
		return data, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	for _, record := range offeringRecords {
		if o.RecordPresence(record) == ValueUnknown {
			raw[string(record)] = json.RawMessage("null")
		}
	}
	return json.Marshal(raw)
}

// UnmarshalJSON restores provider record presence and clears earlier claims.
func (o *ProviderOffering) UnmarshalJSON(data []byte) error {
	type plain ProviderOffering
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*o = ProviderOffering(decoded)
	for _, record := range offeringRecords {
		if value, present := raw[string(record)]; present && isJSONNull(value) {
			o.recordUnknown |= modelRecordBits[record]
		}
	}
	return nil
}

// MarshalYAML retains explicit unknown records without numeric conversion.
func (o ProviderOffering) MarshalYAML() (any, error) {
	type plain ProviderOffering
	if o.recordUnknown == 0 {
		return plain(o), nil
	}
	data, err := yaml.Marshal(plain(o))
	if err != nil {
		return nil, err
	}
	var fields yaml.MapSlice
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, record := range offeringRecords {
		if o.RecordPresence(record) == ValueUnknown {
			fields = append(fields, yaml.MapItem{Key: string(record), Value: nil})
		}
	}
	return fields, nil
}

// UnmarshalYAML restores provider record presence and clears earlier claims.
func (o *ProviderOffering) UnmarshalYAML(data []byte) error {
	type plain ProviderOffering
	var decoded plain
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	*o = ProviderOffering(decoded)
	for _, record := range offeringRecords {
		if value, present := raw[string(record)]; present && value == nil {
			o.recordUnknown |= modelRecordBits[record]
		}
	}
	return nil
}
