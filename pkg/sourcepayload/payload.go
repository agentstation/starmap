// Package sourcepayload enforces bounded resource use before source decoding.
package sourcepayload

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

// UnknownJSONField is reviewable evidence for one additive source field that
// was not promoted into the typed schema.
type UnknownJSONField struct {
	Path     string `json:"path" yaml:"path"`
	Checksum string `json:"checksum" yaml:"checksum"`
}

// MaxJSONNestingDepth bounds object/array nesting before JSON decode.
const MaxJSONNestingDepth = 64

// ValidateJSON enforces source byte and nesting limits before decoding.
func ValidateJSON(data []byte) error {
	if len(data) > constants.MaxSourcePayloadBytes {
		return &errors.ValidationError{Field: "payload", Value: len(data), Message: "exceeds maximum source payload size"}
	}
	depth := 0
	inString := false
	escaped := false
	for _, character := range data {
		if inString {
			switch {
			case escaped:
				escaped = false
			case character == '\\':
				escaped = true
			case character == '"':
				inString = false
			}
			continue
		}
		if character == '"' {
			inString = true
			continue
		}
		switch character {
		case '{', '[':
			depth++
			if depth > MaxJSONNestingDepth {
				return &errors.ValidationError{Field: "payload", Value: depth, Message: "exceeds maximum JSON nesting depth"}
			}
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return nil
}

// UnknownJSONFields returns deterministic path/digest evidence for top-level
// JSON members not declared by schema. Raw values are intentionally omitted.
func UnknownJSONFields(data []byte, schema any, prefix string) ([]UnknownJSONField, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.WrapParse("json", "unknown-field inspection", err)
	}
	known := jsonFieldNames(reflect.TypeOf(schema))
	unknown := make([]UnknownJSONField, 0)
	for name, value := range raw {
		if _, exists := known[name]; exists {
			continue
		}
		path := name
		if prefix != "" {
			path = strings.TrimSuffix(prefix, ".") + "." + name
		}
		digest := sha256.Sum256(value)
		unknown = append(unknown, UnknownJSONField{Path: path, Checksum: "sha256:" + hex.EncodeToString(digest[:])})
	}
	sort.Slice(unknown, func(i, j int) bool { return unknown[i].Path < unknown[j].Path })
	return unknown, nil
}

// FingerprintValue returns path/digest evidence for an unrecognized typed value.
func FingerprintValue(path string, value any) UnknownJSONField {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return UnknownJSONField{Path: path, Checksum: "sha256:" + hex.EncodeToString(digest[:])}
}

func jsonFieldNames(typ reflect.Type) map[string]struct{} {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	result := make(map[string]struct{}, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		tag := typ.Field(index).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			result[name] = struct{}{}
		}
	}
	return result
}
