package catalogs

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

// snapshotProvenance isolates dynamic values before a catalog becomes immutable.
// Source structs use the same generic JSON shape as restored provenance.
// The constructor supplies an owned builder with independent entry slices.
func snapshotProvenance(builder *Builder) error {
	records := builder.provenance
	records.mu.Lock()
	defer records.mu.Unlock()
	for key, entries := range records.provenance {
		for index := range entries {
			entry := &entries[index]
			var err error
			entry.Value, err = snapshotProvenanceValue(entry.Value, nil, 0)
			if err != nil {
				return errors.WrapResource("snapshot", "catalog provenance", key, err)
			}
			entry.PreviousValue, err = snapshotProvenanceValue(entry.PreviousValue, nil, 0)
			if err != nil {
				return errors.WrapResource("snapshot", "catalog provenance", key, err)
			}
			entry.Rejections = slices.Clone(entry.Rejections)
		}
	}
	return nil
}

func snapshotProvenanceValue(value any, path map[any]bool, depth int) (any, error) {
	if depth > sourcepayload.MaxJSONNestingDepth {
		return nil, &errors.ValidationError{Field: "provenance.value", Message: "exceeds the JSON nesting limit"}
	}
	var identity any
	switch value := value.(type) {
	case nil, bool, string, json.Number,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64:
		return value, nil
	case map[string]any:
		identity = reflect.ValueOf(value)
	case []any:
		identity = struct {
			pointer uintptr
			length  int
		}{reflect.ValueOf(value).Pointer(), len(value)}
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		if err := sourcepayload.ValidateJSONWithMaxBytes(encoded, resourcepolicy.MaxPayloadBytes); err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.UseNumber()
		var normalized any
		if err := decoder.Decode(&normalized); err != nil {
			return nil, err
		}
		return normalized, nil
	}
	if path == nil {
		path = make(map[any]bool)
	}
	if path[identity] {
		return nil, &errors.ValidationError{Field: "provenance.value", Message: "must not contain cycles"}
	}
	path[identity] = true
	defer delete(path, identity)
	switch value := value.(type) {
	case map[string]any:
		if value == nil {
			return value, nil
		}
		copied := make(map[string]any, len(value))
		for key, child := range value {
			var err error
			copied[key], err = snapshotProvenanceValue(child, path, depth+1)
			if err != nil {
				return nil, err
			}
		}
		return copied, nil
	case []any:
		if value == nil {
			return value, nil
		}
		copied := make([]any, len(value))
		for index, child := range value {
			var err error
			copied[index], err = snapshotProvenanceValue(child, path, depth+1)
			if err != nil {
				return nil, err
			}
		}
		return copied, nil
	}
	return value, nil
}

// copySnapshotProvenance isolates nested values in an already-owned entry slice.
func copySnapshotProvenance(entries []provenance.Entry) []provenance.Entry {
	for index := range entries {
		entries[index].Value = copySnapshotProvenanceValue(entries[index].Value)
		entries[index].PreviousValue = copySnapshotProvenanceValue(entries[index].PreviousValue)
		entries[index].Rejections = slices.Clone(entries[index].Rejections)
	}
	return entries
}

func copySnapshotProvenanceValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		if value == nil {
			return value
		}
		copied := make(map[string]any, len(value))
		for key, child := range value {
			copied[key] = copySnapshotProvenanceValue(child)
		}
		return copied
	case []any:
		if value == nil {
			return value
		}
		copied := make([]any, len(value))
		for index, child := range value {
			copied[index] = copySnapshotProvenanceValue(child)
		}
		return copied
	default:
		return value
	}
}
