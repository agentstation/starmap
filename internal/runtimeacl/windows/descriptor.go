package windows

// Descriptor owns the native security metadata used by the private DACL policy.
type Descriptor struct {
	Owner   string
	Present bool
	Null    bool
	Entries []Entry
}
