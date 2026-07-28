package modelsdev

import (
	"bytes"
	"encoding/json"

	"github.com/agentstation/starmap/pkg/catalogs"
)

var modelFeatureFields = []string{
	"attachment",
	"reasoning",
	"structured_output",
	"temperature",
	"tool_call",
}

func (m *Model) captureFeaturePresence(raw map[string]json.RawMessage) {
	for _, field := range modelFeatureFields {
		value, present := raw[field]
		if !present {
			continue
		}
		if m.featurePresence == nil {
			m.featurePresence = make(map[string]struct{})
		}
		m.featurePresence[field] = struct{}{}
		if !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if m.featureKnown == nil {
				m.featureKnown = make(map[string]struct{})
			}
			m.featureKnown[field] = struct{}{}
		}
	}
}

func (m *Model) featureValue(field string) (bool, catalogs.ValuePresence) {
	var value bool
	switch field {
	case "attachment":
		value = m.Attachment
	case "reasoning":
		value = m.Reasoning
	case "structured_output":
		value = m.StructuredOutput
	case "temperature":
		value = m.Temperature
	case "tool_call":
		value = m.ToolCall
	default:
		return false, catalogs.ValueMissing
	}
	if _, present := m.featurePresence[field]; present {
		if _, known := m.featureKnown[field]; known {
			return value, catalogs.ValueKnown
		}
		return false, catalogs.ValueUnknown
	}
	if value {
		return true, catalogs.ValueKnown
	}
	return false, catalogs.ValueMissing
}

// UnmarshalJSON retains missing, null, zero, and non-zero limit states.
func (l *Limit) UnmarshalJSON(data []byte) error {
	type plain Limit
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*l = Limit(decoded)
	for _, field := range []string{"context", "input", "output"} {
		value, present := raw[field]
		if !present {
			continue
		}
		if l.presence == nil {
			l.presence = make(map[string]struct{})
		}
		l.presence[field] = struct{}{}
		if !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if l.known == nil {
				l.known = make(map[string]struct{})
			}
			l.known[field] = struct{}{}
		}
	}
	return nil
}

func (l Limit) value(field string) (int64, catalogs.ValuePresence) {
	var value int
	switch field {
	case "context":
		value = l.Context
	case "input":
		value = l.Input
	case "output":
		value = l.Output
	default:
		return 0, catalogs.ValueMissing
	}
	if _, present := l.presence[field]; present {
		if _, known := l.known[field]; known {
			return int64(value), catalogs.ValueKnown
		}
		return 0, catalogs.ValueUnknown
	}
	if value != 0 {
		return int64(value), catalogs.ValueKnown
	}
	return 0, catalogs.ValueMissing
}
