package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/internal/sourcepayload"
	"github.com/agentstation/starmap/internal/constants"
)

func FuzzModelsEnvelopeNoPanic(f *testing.F) {
	f.Add([]byte(`{"data":[{"id":"model","display_name":"Model"}]}`))
	f.Add([]byte(`{"data":[{"id":"valid"},{"id":"invalid","max_tokens":"drift"}]}`))
	f.Add([]byte(`{"data":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > constants.MaxSourcePayloadBytes {
			t.Skip()
		}
		if err := sourcepayload.ValidateJSON(data); err != nil {
			return
		}
		var response modelsResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return
		}
		_ = response.RecordReport.Err("anthropic models")
	})
}
