package artifact

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

func TestPublicationReceiptQuarantineRequiresPolicyAndPreservesDiagnostics(t *testing.T) {
	receipt := publicationReceiptFixture(t)
	source := &receipt.Sources[0]
	source.Policy.AllowRecordQuarantine = true
	source.Attempt = "partial"
	source.Observation.Status = evidence.ObservationStatusDegraded
	source.Observation.Completeness = evidence.ObservationCompletenessPartial
	source.Quarantine = &evidence.RecordQuarantine{Records: evidence.ObservationRecordCounts{Accepted: 20, Rejected: 1},
		Issues: []evidence.QuarantinedRecord{{Scope: evidence.ObservationIssueScopeRecord, Code: evidence.ObservationIssueCodeInvalidRecord, Subject: "provider/model"}}}
	data, err := EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodePublicationReceipt(data)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Sources[0].Quarantine.Issues[0].Subject != "provider/model" || restored.Sources[0].Observation.Status != evidence.ObservationStatusDegraded {
		t.Fatal("receipt lost record identity or degraded status")
	}
	copied := restored.Copy()
	copied.Sources[0].Quarantine.Issues[0].Subject = "changed"
	if restored.Sources[0].Quarantine.Issues[0].Subject != "provider/model" {
		t.Fatal("receipt copy shares quarantine records")
	}
	for _, test := range []struct {
		name   string
		change func(*PublicationSourceReceipt)
	}{
		{"policy absent", func(s *PublicationSourceReceipt) { s.Policy.AllowRecordQuarantine = false }},
		{"diagnostics absent", func(s *PublicationSourceReceipt) { s.Quarantine = nil }},
		{"success claim", func(s *PublicationSourceReceipt) { s.Attempt = "succeeded" }},
		{"complete claim", func(s *PublicationSourceReceipt) {
			s.Observation.Completeness = evidence.ObservationCompletenessComplete
		}},
		{"no valid record", func(s *PublicationSourceReceipt) { s.Quarantine.Records.Accepted = 0 }},
		{"missing record diagnostic", func(s *PublicationSourceReceipt) { s.Quarantine.Records.Rejected++ }},
		{"duplicate record diagnostic", func(s *PublicationSourceReceipt) {
			s.Quarantine.Records.Rejected++
			s.Quarantine.Issues = append(s.Quarantine.Issues, s.Quarantine.Issues[0])
		}},
		{"empty subject", func(s *PublicationSourceReceipt) { s.Quarantine.Issues[0].Subject = "" }},
		{"source failure", func(s *PublicationSourceReceipt) { s.Quarantine.Issues[0].Scope = evidence.ObservationIssueScopeSource }},
		{"payload failure", func(s *PublicationSourceReceipt) {
			s.Quarantine.Issues[0].Code = evidence.ObservationIssueCodePayloadLimit
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := restored.Copy()
			test.change(&changed.Sources[0])
			if _, err := EncodePublicationReceipt(changed); err == nil {
				t.Fatal("invalid quarantine claim accepted")
			}
		})
	}
	source.Quarantine.Issues = make([]evidence.QuarantinedRecord, evidence.MaxQuarantinedRecords+1)
	oversized, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePublicationReceipt(oversized); err == nil {
		t.Fatal("oversized quarantine array accepted")
	}
}
