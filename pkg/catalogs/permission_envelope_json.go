package catalogs

import (
	"bytes"
	"encoding/json"
	"io"
	"slices"
)

// validatePermissionObjectNames rejects duplicate members at both defined object levels.
// Escaped member names share the same identity after JSON decoding.
func validatePermissionObjectNames(data []byte) error {
	object, err := permissionObjectMembers(data, []string{"version", "head", "issued_at", "valid_until"})
	if err != nil {
		return err
	}
	head, found := object["head"]
	if !found {
		return validationError("permission.head", nil, "is required")
	}
	_, err = permissionObjectMembers(head, []string{
		"authority_id", "policy_id", "sequence", "generation_id", "payload_checksum",
		"required_permission_revision", "permission_schema_version",
	})
	return err
}

func permissionObjectMembers(data []byte, allowed []string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, validationError("permission.envelope", nil, "must contain permission objects")
	}
	members := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, validationError("permission.envelope", nil, "contains an invalid member name")
		}
		name, ok := token.(string)
		if !ok {
			return nil, validationError("permission.envelope", nil, "requires string member names")
		}
		if !slices.Contains(allowed, name) {
			return nil, validationError("permission.envelope", nil, "contains an unknown object member")
		}
		if _, duplicate := members[name]; duplicate {
			return nil, validationError("permission.envelope", nil, "cannot repeat an object member")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, validationError("permission.envelope", nil, "contains an invalid member value")
		}
		members[name] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, validationError("permission.envelope", nil, "must end each permission object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, validationError("permission.envelope", nil, "must contain exactly one JSON document")
	}
	return members, nil
}
