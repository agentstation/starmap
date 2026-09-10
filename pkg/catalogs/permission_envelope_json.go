package catalogs

// validatePermissionObjectNames rejects duplicate members at both defined object levels.
// Escaped member names share the same identity after JSON decoding.
func validatePermissionObjectNames(data []byte) error {
	object, err := strictJSONObjectMembers(data, "permission.envelope", []string{"version", "head", "issued_at", "valid_until"})
	if err != nil {
		return err
	}
	head, found := object["head"]
	if !found {
		return validationError("permission.head", nil, "is required")
	}
	_, err = strictJSONObjectMembers(head, "permission.head", catalogAuthorityHeadJSONFields)
	return err
}
