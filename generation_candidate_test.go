package starmap

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestCandidateGenerationRequiresExplicitIdentityAndTime(t *testing.T) {
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	withoutID, err := NewCandidate(empty, CandidateEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := NewCandidate(empty, CandidateEvidence{}, WithCandidateGenerationID("deterministic"))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	for _, scenario := range []struct {
		name      string
		candidate *Candidate
		runID     string
		at        time.Time
	}{
		{"nil", nil, "run", at},
		{"implicit identity", withoutID, "run", at},
		{"empty run", candidate, "", at},
		{"zero time", candidate, "run", time.Time{}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if _, err := scenario.candidate.Generation(scenario.runID, scenario.at); err == nil {
				t.Fatal("invalid generation was accepted")
			}
		})
	}
	first, err := candidate.Generation("run", at)
	if err != nil {
		t.Fatal(err)
	}
	second, err := candidate.Generation("run", at.In(time.FixedZone("fixture", 3600)))
	if err != nil {
		t.Fatal(err)
	}
	one, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	two, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one, two) {
		t.Fatal("the same instant changed generation bytes")
	}
}
