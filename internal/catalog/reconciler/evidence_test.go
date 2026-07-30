package reconciler

import (
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceObservationEvidenceHealthReasonSummarizesIssueCounts(t *testing.T) {
	evidence := sourceObservationEvidence{
		completeness: sources.ObservationCompletenessPartial,
		status:       sources.ObservationStatusDegraded,
		records:      sources.ObservationRecordCounts{Accepted: 467, Rejected: 32},
		issues: []sources.ObservationIssue{
			{Code: sources.ObservationIssueCodeInvalidRecord},
			{Code: sources.ObservationIssueCodeConfiguration},
			{Code: sources.ObservationIssueCodeInvalidRecord},
		},
	}

	got := evidence.healthReason()
	want := "observation health status=degraded completeness=partial accepted=467 rejected=32 issues=configuration:1,invalid_record:2"
	if got != want {
		t.Fatalf("healthReason() = %q, want %q", got, want)
	}
}
