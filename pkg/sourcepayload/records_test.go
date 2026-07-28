package sourcepayload

import (
	stderrors "errors"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type recordFixture struct {
	ID    string `json:"id"`
	Limit int    `json:"limit"`
}

func TestDecodeJSONArrayQuarantinesMalformedSiblings(t *testing.T) {
	records, report, err := DecodeJSONArray[recordFixture](
		[]byte(`[{"id":"valid","limit":1},{"id":"invalid","limit":"drift"}]`),
		"records",
		10,
	)
	if err != nil {
		t.Fatalf("DecodeJSONArray: %v", err)
	}
	if len(records) != 1 || records[0].ID != "valid" {
		t.Fatalf("records = %#v", records)
	}
	if report.Accepted != 1 || report.Rejected != 1 || len(report.Issues) != 1 {
		t.Fatalf("report = %#v", report)
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(report.Issues[0].Err, &parseErr) {
		t.Fatalf("issue = %T: %v, want *errors.ParseError", report.Issues[0].Err, report.Issues[0].Err)
	}
	var quarantineErr *QuarantineError
	if !stderrors.As(report.Err("records"), &quarantineErr) {
		t.Fatal("report did not produce typed quarantine error")
	}
}

func TestDecodeJSONObjectIsDeterministicAndBounded(t *testing.T) {
	records, report, err := DecodeJSONObject[recordFixture](
		[]byte(`{"z":{"id":"z","limit":1},"a":{"id":"a","limit":1},"m":{"id":"m","limit":1}}`),
		"records",
		2,
	)
	if err != nil {
		t.Fatalf("DecodeJSONObject: %v", err)
	}
	if len(records) != 2 || records["a"].ID != "a" || records["m"].ID != "m" {
		t.Fatalf("records = %#v, want first two sorted identities", records)
	}
	if report.Accepted != 2 || report.Rejected != 1 || !report.Truncated {
		t.Fatalf("report = %#v, want bounded truncation", report)
	}
}

func TestDecodeRecordsRejectsMalformedEnvelope(t *testing.T) {
	records, report, err := DecodeJSONArray[recordFixture]([]byte(`{"id":"not-an-array"}`), "records", 10)
	if err == nil || records != nil ||
		report.Accepted != 0 || report.Rejected != 0 || len(report.Issues) != 0 || report.Truncated {
		t.Fatalf("result = %#v, %#v, %v; want fatal envelope error", records, report, err)
	}
}
