package catalogs

import (
	"bytes"
	"encoding/json"
	"io"
	"slices"
)

// strictJSONObjectMembers requires exact member names and rejects duplicate names after escape decoding.
func strictJSONObjectMembers(data []byte, field string, allowed []string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, validationError(field, nil, "must contain a JSON object")
	}
	members := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, validationError(field, nil, "contains an invalid member name")
		}
		name, ok := token.(string)
		if !ok {
			return nil, validationError(field, nil, "requires string member names")
		}
		if !slices.Contains(allowed, name) {
			return nil, validationError(field, nil, "contains an unknown object member")
		}
		if _, duplicate := members[name]; duplicate {
			return nil, validationError(field, nil, "cannot repeat an object member")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, validationError(field, nil, "contains an invalid member value")
		}
		members[name] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, validationError(field, nil, "must end the JSON object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, validationError(field, nil, "must contain exactly one JSON document")
	}
	return members, nil
}
