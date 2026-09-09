package sources

import "slices"

// SourceFailure identifies a source failure without exposing upstream messages or credentials.
type SourceFailure struct {
	// Source identifies the registered catalog source.
	Source ID `json:"source"`
	// Reason is the bounded cause of the source failure.
	Reason ProviderReason `json:"reason"`
}

// Valid reports whether the source and reason belong to the registered sets.
func (f SourceFailure) Valid() bool {
	return slices.Contains(IDs(), f.Source) && f.Reason.Valid()
}
