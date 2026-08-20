package google

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

func FuzzAIStudioModelsEnvelopeNoPanic(f *testing.F) {
	f.Add([]byte(`{"models":[{"name":"models/model","displayName":"Model"}]}`))
	f.Add([]byte(`{"models":[{"name":"models/valid"},{"name":"models/invalid","inputTokenLimit":"drift"}]}`))
	f.Add([]byte(`{"models":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > constants.MaxSourcePayloadBytes {
			t.Skip()
		}
		if err := sourcepayload.ValidateJSON(data); err != nil {
			return
		}
		var response aiStudioModelsResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return
		}
		_ = response.RecordReport.Err("google AI Studio models")
	})
}
