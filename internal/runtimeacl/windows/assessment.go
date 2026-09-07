package windows

// Assessment reports descriptor compatibility with the private DACL policy.
// Compatibility does not establish effective access, ancestor safety, or a stable snapshot.
type Assessment struct {
	DACLState  string
	EntryCount int
	Status     string
	Reason     string
}

// Assess classifies an observed descriptor without resolving principal names or reading files.
func Assess(account string, descriptor Descriptor) Assessment {
	result := Assessment{EntryCount: len(descriptor.Entries), DACLState: "present", Status: "compatible"}
	switch {
	case !descriptor.Present:
		result.DACLState = "absent"
	case descriptor.Null:
		result.DACLState = "null"
	case len(descriptor.Entries) == 0:
		result.DACLState = "empty"
	}
	if Validate(account, descriptor.Owner, descriptor.Present, descriptor.Null, descriptor.Entries, "inspection") != nil {
		result.Status = "conflict"
		result.Reason = "native-private-policy-conflict"
	}
	return result
}
