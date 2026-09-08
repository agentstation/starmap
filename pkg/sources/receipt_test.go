package sources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func receiptObservation(t *testing.T) Observation {
	t.Helper()
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := NewObservation(ProvidersID, catalog, ObservationMetadata{
		ObservedAt:   time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC),
		Revision:     Revision{Kind: RevisionKindContentDigest},
		Completeness: ObservationCompletenessPartial, Status: ObservationStatusDegraded,
		Records: ObservationRecordCounts{Rejected: 1},
		Issues:  []ObservationIssue{{Scope: ObservationIssueScopeRecord, Code: ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "private-diagnostic-sentinel"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestObservationReceiptRoundTripExcludesDiagnostics(t *testing.T) {
	observation := receiptObservation(t)
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-diagnostic-sentinel") || strings.Contains(string(encoded), "\"message\"") {
		t.Fatal("receipt retained a diagnostic message")
	}
	var decoded ObservationReceipt
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	restored, err := decoded.Restore(observation.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != observation.ID || restored.Records != observation.Records || restored.Completeness != observation.Completeness || restored.Status != observation.Status {
		t.Fatal("receipt changed evidence identity or health")
	}
	if restored.Issues[0].Message != string(ObservationIssueCodeInvalidRecord) {
		t.Fatal("restored diagnostic is not a stable issue code")
	}
	observation.Issues[0].Subject = "caller-mutation"
	clone := receipt.Clone()
	clone.Issues[0].Subject = "clone-mutation"
	restored.Issues[0].Subject = "restored-mutation"
	if receipt.Issues[0].Subject != "provider/invalid" || decoded.Issues[0].Subject != "provider/invalid" {
		t.Fatal("receipt collections alias a caller-owned collection")
	}
}

func TestObservationReceiptRejectsTampering(t *testing.T) {
	observation := receiptObservation(t)
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*ObservationReceipt){
		"identity":      func(r *ObservationReceipt) { r.Link.ObservationID = "forged" },
		"source":        func(r *ObservationReceipt) { r.Link.Source = "different" },
		"time":          func(r *ObservationReceipt) { r.Link.ObservedAt = r.Link.ObservedAt.Add(time.Second) },
		"checksum":      func(r *ObservationReceipt) { r.Link.EvidenceChecksum = "invalid" },
		"revision":      func(r *ObservationReceipt) { r.Link.Revision.Value = "invalid" },
		"completeness":  func(r *ObservationReceipt) { r.Link.Completeness = ObservationCompletenessComplete },
		"status":        func(r *ObservationReceipt) { r.Link.Status = ObservationStatusSucceeded },
		"counts":        func(r *ObservationReceipt) { r.Records.Rejected++ },
		"issue-code":    func(r *ObservationReceipt) { r.Issues[0].Code = ObservationIssueCodeSchemaDrift },
		"issue-subject": func(r *ObservationReceipt) { r.Issues[0].Subject = "different" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			altered := receipt.Clone()
			change(&altered)
			if _, err := altered.Restore(observation.Catalog); err == nil {
				t.Fatal("tampered receipt accepted")
			}
		})
	}
	observation.ID = "forged"
	if _, err := observation.Receipt(); err == nil {
		t.Fatal("receipt accepted an invalid observation")
	}
}
