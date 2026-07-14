// Package sourcepayload enforces bounded resource use before source decoding.
package sourcepayload

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

const (
	payloadField = "payload"
	jsonFormat   = "json"
	// MaxJSONNestingDepth bounds object/array nesting before JSON decode.
	MaxJSONNestingDepth = 64
)

// ValidateJSON enforces source byte and nesting limits before decoding.
func ValidateJSON(data []byte) error {
	if len(data) > constants.MaxSourcePayloadBytes {
		return &errors.ValidationError{Field: payloadField, Value: len(data), Message: "exceeds maximum source payload size"}
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
				return &errors.ValidationError{Field: payloadField, Value: depth, Message: "exceeds maximum JSON nesting depth"}
			}
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return nil
}

// ValidateExactJSON enforces resource bounds, valid single-document JSON, and
// unique member names in every object before typed decoding.
func ValidateExactJSON(data []byte) error {
	if err := ValidateJSON(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := validateExactJSONValue(decoder, "$"); err != nil {
		return err
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return &errors.ValidationError{Field: payloadField, Value: token, Message: "must contain exactly one JSON document"}
		}
		return &errors.ParseError{Format: jsonFormat, File: payloadField, Message: err.Error(), Err: err}
	}
	return nil
}

func validateExactJSONValue(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return &errors.ParseError{Format: jsonFormat, File: payloadField, Message: err.Error(), Err: err}
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return &errors.ParseError{Format: jsonFormat, File: payloadField, Message: keyErr.Error(), Err: keyErr}
			}
			key, ok := keyToken.(string)
			if !ok {
				return &errors.ValidationError{Field: path, Value: keyToken, Message: "object member name must be a string"}
			}
			memberPath := path + "." + key
			if _, found := seen[key]; found {
				return &errors.ValidationError{Field: memberPath, Message: "duplicate JSON member is not allowed"}
			}
			seen[key] = struct{}{}
			if valueErr := validateExactJSONValue(decoder, memberPath); valueErr != nil {
				return valueErr
			}
		}
		_, err = decoder.Token()
	case '[':
		for index := 0; decoder.More(); index++ {
			if valueErr := validateExactJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index)); valueErr != nil {
				return valueErr
			}
		}
		_, err = decoder.Token()
	default:
		return &errors.ValidationError{Field: path, Value: delimiter, Message: "unexpected JSON delimiter"}
	}
	if err != nil {
		return &errors.ParseError{Format: jsonFormat, File: payloadField, Message: err.Error(), Err: err}
	}
	return nil
}

// UnknownJSONFields returns deterministic path/digest evidence for top-level
// JSON members not declared by schema. Raw values are intentionally omitted.
func UnknownJSONFields(data []byte, schema any, prefix string) ([]UnknownJSONField, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.WrapParse(jsonFormat, "unknown-field inspection", err)
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
		tag := typ.Field(index).Tag.Get(jsonFormat)
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			result[name] = struct{}{}
		}
	}
	return result
}
