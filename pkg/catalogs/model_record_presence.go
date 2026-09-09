package catalogs

import "slices"

// ModelRecord identifies one optional model record.
type ModelRecord string

// Optional model records.
const (
	ModelRecordDeprecatedAt    ModelRecord = "deprecated_at"
	ModelRecordRetiresAt       ModelRecord = "retires_at"
	ModelRecordMetadata        ModelRecord = "metadata"
	ModelRecordLineage         ModelRecord = "lineage"
	ModelRecordFeatures        ModelRecord = "features"
	ModelRecordAttachments     ModelRecord = "attachments"
	ModelRecordGeneration      ModelRecord = "generation"
	ModelRecordReasoning       ModelRecord = "reasoning"
	ModelRecordReasoningTokens ModelRecord = "reasoning_tokens"
	ModelRecordVerbosity       ModelRecord = "verbosity"
	ModelRecordTools           ModelRecord = "tools"
	ModelRecordDelivery        ModelRecord = "response"
	ModelRecordPricing         ModelRecord = "pricing"
	ModelRecordLimits          ModelRecord = "limits"
)

var modelRecordOrder = []ModelRecord{
	ModelRecordDeprecatedAt,
	ModelRecordRetiresAt,
	ModelRecordMetadata,
	ModelRecordLineage,
	ModelRecordFeatures,
	ModelRecordAttachments,
	ModelRecordGeneration,
	ModelRecordReasoning,
	ModelRecordReasoningTokens,
	ModelRecordVerbosity,
	ModelRecordTools,
	ModelRecordDelivery,
	ModelRecordPricing,
	ModelRecordLimits,
}

var modelRecordBits = map[ModelRecord]uint16{
	ModelRecordDeprecatedAt:    1 << 0,
	ModelRecordRetiresAt:       1 << 1,
	ModelRecordMetadata:        1 << 2,
	ModelRecordLineage:         1 << 3,
	ModelRecordFeatures:        1 << 4,
	ModelRecordAttachments:     1 << 5,
	ModelRecordGeneration:      1 << 6,
	ModelRecordReasoning:       1 << 7,
	ModelRecordReasoningTokens: 1 << 8,
	ModelRecordVerbosity:       1 << 9,
	ModelRecordTools:           1 << 10,
	ModelRecordDelivery:        1 << 11,
	ModelRecordPricing:         1 << 12,
	ModelRecordLimits:          1 << 13,
}

// PublishedModelRecords returns every optional record in published order.
func PublishedModelRecords() []ModelRecord { return slices.Clone(modelRecordOrder) }

// RecordPresence distinguishes missing, unknown, and known optional records.
// Non-nil records represent known data, including explicitly empty records.
func (m *Model) RecordPresence(record ModelRecord) ValuePresence {
	if m == nil {
		return ValueMissing
	}
	var value any
	switch record {
	case ModelRecordDeprecatedAt:
		value = m.DeprecatedAt
	case ModelRecordRetiresAt:
		value = m.RetiresAt
	case ModelRecordMetadata:
		value = m.Metadata
	case ModelRecordLineage:
		value = m.Lineage
	case ModelRecordFeatures:
		value = m.Features
	case ModelRecordAttachments:
		value = m.Attachments
	case ModelRecordGeneration:
		value = m.Generation
	case ModelRecordReasoning:
		value = m.Reasoning
	case ModelRecordReasoningTokens:
		value = m.ReasoningTokens
	case ModelRecordVerbosity:
		value = m.Verbosity
	case ModelRecordTools:
		value = m.Tools
	case ModelRecordDelivery:
		value = m.Delivery
	case ModelRecordPricing:
		value = m.Pricing
	case ModelRecordLimits:
		value = m.Limits
	default:
		return ValueMissing
	}
	if !isNilPresenceValue(value) {
		return ValueKnown
	}
	if m.recordUnknown&modelRecordBits[record] != 0 {
		return ValueUnknown
	}
	return ValueMissing
}

// SetRecordUnknown replaces one record with an explicit unknown claim.
// It returns false for a nil receiver or an unsupported record.
func (m *Model) SetRecordUnknown(record ModelRecord) bool {
	if m == nil {
		return false
	}
	switch record {
	case ModelRecordDeprecatedAt:
		m.DeprecatedAt = nil
	case ModelRecordRetiresAt:
		m.RetiresAt = nil
	case ModelRecordMetadata:
		m.Metadata = nil
	case ModelRecordLineage:
		m.Lineage = nil
	case ModelRecordFeatures:
		m.Features = nil
	case ModelRecordAttachments:
		m.Attachments = nil
	case ModelRecordGeneration:
		m.Generation = nil
	case ModelRecordReasoning:
		m.Reasoning = nil
	case ModelRecordReasoningTokens:
		m.ReasoningTokens = nil
	case ModelRecordVerbosity:
		m.Verbosity = nil
	case ModelRecordTools:
		m.Tools = nil
	case ModelRecordDelivery:
		m.Delivery = nil
	case ModelRecordPricing:
		m.Pricing = nil
	case ModelRecordLimits:
		m.Limits = nil
	default:
		return false
	}
	m.recordUnknown |= modelRecordBits[record]
	return true
}

// UnsetRecord removes one record claim. It does not withdraw the offering.
func (m *Model) UnsetRecord(record ModelRecord) bool {
	if !m.SetRecordUnknown(record) {
		return false
	}
	m.recordUnknown &^= modelRecordBits[record]
	return true
}
