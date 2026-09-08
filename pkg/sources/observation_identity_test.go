package sources

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestObservationIdentitySeparatesIssueBoundaries(t *testing.T) {
	original := receiptObservation(t)
	first := original
	first.Issues = []ObservationIssue{{Scope: ObservationIssueScopeRecord, Code: ObservationIssueCodeInvalidRecord, Subject: "first\x00record\x00schema_drift\x00second", Message: "one issue"}}
	first.ID = observationID(first)
	second := original
	second.Issues = []ObservationIssue{
		{Scope: ObservationIssueScopeRecord, Code: ObservationIssueCodeInvalidRecord, Subject: "first", Message: "first issue"},
		{Scope: ObservationIssueScopeRecord, Code: ObservationIssueCodeSchemaDrift, Subject: "second", Message: "second issue"},
	}
	second.ID = observationID(second)
	if err := first.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := second.Validate(); err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Error("different structured issue records share an observation identity")
	}
	receipt, err := first.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	receipt.Issues = []ObservationIssueReceipt{
		{Scope: second.Issues[0].Scope, Code: second.Issues[0].Code, Subject: second.Issues[0].Subject},
		{Scope: second.Issues[1].Scope, Code: second.Issues[1].Code, Subject: second.Issues[1].Subject},
	}
	if _, err := receipt.Restore(original.Catalog); err == nil {
		t.Error("receipt restoration accepted changed issue boundaries")
	}
}

func TestObservationIdentityReadsLegacyFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/legacy-observation-receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Observation Observation     `json:"observation"`
		Payload     json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.DecodeSourceObservationPayload(fixture.Payload)
	if err != nil {
		t.Fatal(err)
	}
	observation := fixture.Observation
	observation.Catalog = catalog
	if err := observation.Validate(); err != nil {
		t.Fatal(err)
	}
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := receipt.Restore(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != observation.ID {
		t.Fatal("legacy identity changed during restoration")
	}
	current, err := NewObservation(observation.SourceID, catalog, ObservationMetadata{
		ObservedAt: observation.ObservedAt, Revision: observation.Revision,
		Completeness: observation.Completeness, Status: observation.Status,
		Records: observation.Records, Issues: observation.Issues,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(current.ID, "observation:v2:") || current.ID == observation.ID {
		t.Fatalf("new observation did not use versioned identity: %s", current.ID)
	}
}

func TestObservationIdentityRefusesAmbiguousLegacyFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Observation)
	}{
		{"source", func(o *Observation) { o.SourceID = "provider\x00extra" }},
		{"revision", func(o *Observation) { o.Revision = Revision{Kind: RevisionKindETag, Value: "version\x00extra"} }},
		{"subject", func(o *Observation) { o.Issues[0].Subject = "first\x00record\x00schema_drift\x00second" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := receiptObservation(t)
			test.mutate(&observation)
			observation.ID = legacyObservationID(observation)
			if err := observation.Validate(); err == nil {
				t.Fatal("accepted ambiguous legacy identity")
			}
			observation.ID = observationID(observation)
			if err := observation.Validate(); err != nil {
				t.Fatalf("refused unambiguous versioned identity: %v", err)
			}
		})
	}
}

func TestObservationIdentityRefusesUnknownVersion(t *testing.T) {
	observation := receiptObservation(t)
	observation.ID = strings.Replace(observation.ID, "observation:v2:", "observation:v4:", 1)
	if err := observation.Validate(); err == nil {
		t.Fatal("accepted unknown identity version")
	}
}
