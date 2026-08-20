package transport

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

type recordAwareResponse struct {
	Data   []recordAwareModel
	Report sourcepayload.RecordReport
}

type recordAwareModel struct {
	ID string `json:"id"`
}

func (r *recordAwareResponse) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	records, report, err := sourcepayload.DecodeJSONArray[recordAwareModel](
		envelope.Data,
		"data",
		constants.MaxCatalogModels,
	)
	r.Data, r.Report = records, report
	return err
}

// TestF009ProviderResponseSupportsRecordAwareDecode proves the bounded
// transport permits a target to retain valid records and typed diagnostics.
func TestF009ProviderResponseSupportsRecordAwareDecode(t *testing.T) {
	var target recordAwareResponse
	err := DecodeResponse(
		jsonResponse(`{"data":[{"id":"valid"},{"id":["schema-drift"]}]}`),
		&target,
	)
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	if len(target.Data) != 1 || target.Data[0].ID != "valid" {
		t.Fatalf("valid records = %#v, want one valid sibling", target.Data)
	}
	if target.Report.Rejected != 1 || len(target.Report.Issues) != 1 {
		t.Fatalf("record report = %#v, want one rejected sibling", target.Report)
	}
}
