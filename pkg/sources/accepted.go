package sources

import "slices"

// AcceptedSourceState binds accepted acquisition inputs to one active generation.
// A missing snapshot means the composition cannot report retained input ownership.
type AcceptedSourceState struct {
	// GenerationID identifies the active generation at the snapshot time.
	GenerationID string `json:"generation_id"`
	// Sources names accepted acquisition input, independently of the last attempt.
	Sources []ID `json:"sources"`
}

// Clone returns a snapshot with caller-owned source identifiers.
func (s AcceptedSourceState) Clone() AcceptedSourceState {
	s.Sources = append([]ID{}, s.Sources...)
	return s
}

// Valid reports whether the snapshot names a generation and unique acquisition sources.
func (s AcceptedSourceState) Valid() bool {
	if s.GenerationID == "" {
		return false
	}
	for i, id := range s.Sources {
		switch id {
		case LocalCatalogID, ModelsDevHTTPID, ModelsDevGitID, ProvidersID:
		default:
			return false
		}
		if slices.Contains(s.Sources[:i], id) {
			return false
		}
	}
	return true
}
