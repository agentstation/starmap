package openai

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

func FuzzModelsEnvelopeNoPanic(f *testing.F) {
	f.Add([]byte(`{"object":"list","data":[{"id":"model"}]}`))
	f.Add([]byte(`{"data":[{"id":"valid"},{"id":"invalid","max_model_len":"drift"}]}`))
	f.Add([]byte(`{"data":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > constants.MaxSourcePayloadBytes {
			t.Skip()
		}
		if err := sourcepayload.ValidateJSON(data); err != nil {
			return
		}
		var response Response
		if err := json.Unmarshal(data, &response); err != nil {
			return
		}
		_ = response.RecordReport.Err("openai-compatible models")
	})
}
