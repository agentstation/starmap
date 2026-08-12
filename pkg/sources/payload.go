package sources

import sourcepayload "github.com/agentstation/starmap/internal/sources/payload"

// MaxJSONNestingDepth bounds object/array nesting before JSON decode.
const MaxJSONNestingDepth = sourcepayload.MaxJSONNestingDepth

// ValidateJSONPayload enforces source byte and nesting limits before decoding.
func ValidateJSONPayload(data []byte) error {
	return sourcepayload.ValidateJSON(data)
}
