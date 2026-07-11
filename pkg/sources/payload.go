package sources

import "github.com/agentstation/starmap/pkg/sourcepayload"

// MaxJSONNestingDepth bounds object/array nesting before JSON decode.
const MaxJSONNestingDepth = sourcepayload.MaxJSONNestingDepth

// ValidateJSONPayload enforces source byte and nesting limits before decoding.
func ValidateJSONPayload(data []byte) error {
	return sourcepayload.ValidateJSON(data)
}
