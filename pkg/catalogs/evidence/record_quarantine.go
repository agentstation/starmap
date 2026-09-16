package evidence

import (
	"strings"
	"unicode/utf8"
)

// MaxQuarantinedRecords bounds the diagnostic records in one publication source.
const MaxQuarantinedRecords = 4096

// MaxQuarantineSubjectBytes bounds one diagnostic record identifier.
const MaxQuarantineSubjectBytes = 4096

// QuarantinedRecord identifies an invalid record without its payload or diagnostic message.
type QuarantinedRecord struct {
	Scope   ObservationIssueScope `json:"scope"`
	Code    ObservationIssueCode  `json:"code"`
	Subject string                `json:"subject"`
}

// RecordQuarantine reports isolated invalid records alongside accepted source data.
type RecordQuarantine struct {
	Records ObservationRecordCounts `json:"records"`
	Issues  []QuarantinedRecord     `json:"issues"`
}

// Valid requires useful accepted data and exclusively classified record failures.
// Transport, schema, truncation, and stale-fallback failures do not qualify.
func (r RecordQuarantine) Valid() bool {
	if r.Records.Accepted <= 0 || r.Records.Rejected <= 0 || len(r.Issues) != r.Records.Rejected || len(r.Issues) > MaxQuarantinedRecords {
		return false
	}
	seen := make(map[string]bool, len(r.Issues))
	for _, issue := range r.Issues {
		if issue.Scope != ObservationIssueScopeRecord || issue.Code != ObservationIssueCodeInvalidRecord ||
			strings.TrimSpace(issue.Subject) == "" || len(issue.Subject) > MaxQuarantineSubjectBytes || !utf8.ValidString(issue.Subject) {
			return false
		}
		if seen[issue.Subject] {
			return false
		}
		seen[issue.Subject] = true
	}
	return true
}
