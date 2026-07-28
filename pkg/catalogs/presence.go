package catalogs

// ValuePresence describes whether a source supplied a field value.
//
// Missing means the field was omitted and makes no claim. Unknown means the
// source explicitly reported that it does not know the value. Known means the
// source supplied a value, including false, zero, or an empty string.
type ValuePresence uint8

const (
	// ValueMissing means a field was omitted and makes no claim.
	ValueMissing ValuePresence = iota
	// ValueUnknown means a field was explicitly reported as unknown.
	ValueUnknown
	// ValueKnown means a field has a supplied value, including its zero value.
	ValueKnown
)
