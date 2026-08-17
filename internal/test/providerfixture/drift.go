package providerfixture

import (
	"encoding/json"
	"sort"

	"github.com/agentstation/starmap/pkg/errors"
)

// WireDrift reports the model fields that a recorded fixture and a live provider
// response do not share. Absent fields are fields the fixture exercises that the
// provider no longer returns. Added fields are fields the provider returns that
// the fixture does not record. Either direction means the fixture no longer
// mirrors the provider, so the mapping contract it proves is no longer current.
//
// Drift is a stronger currency signal than age: age reports only that a capture
// is old, while drift names what changed.
func WireDrift(recorded, live []byte) (absent, added []string, err error) {
	recordedFields, err := WireModelFields(recorded)
	if err != nil {
		return nil, nil, err
	}
	liveFields, err := WireModelFields(live)
	if err != nil {
		return nil, nil, err
	}
	return missingFields(recordedFields, liveFields), missingFields(liveFields, recordedFields), nil
}

// WireModelFields returns the sorted union of member names across every model
// object in a provider list response. Every governed provider returns its models
// in a top-level data array, so one reader serves each protocol.
func WireModelFields(payload []byte) ([]string, error) {
	var response struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, errors.WrapParse("json", "provider list response", err)
	}
	if response.Data == nil {
		return nil, &errors.ValidationError{
			Field:   "data",
			Message: "provider list response has a missing or null data array",
		}
	}
	present := make(map[string]struct{})
	for _, model := range response.Data {
		for field := range model {
			present[field] = struct{}{}
		}
	}
	fields := make([]string, 0, len(present))
	for field := range present {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields, nil
}

// missingFields returns the wanted fields that the available fields omit.
func missingFields(want, have []string) []string {
	present := make(map[string]struct{}, len(have))
	for _, field := range have {
		present[field] = struct{}{}
	}
	var absent []string
	for _, field := range want {
		if _, found := present[field]; !found {
			absent = append(absent, field)
		}
	}
	return absent
}
