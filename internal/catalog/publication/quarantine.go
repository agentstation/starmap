package publication

import (
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/sources"
)

func quarantinedRecords(observation sources.Observation) *evidence.RecordQuarantine {
	if observation.Completeness != sources.ObservationCompletenessPartial || observation.Status != sources.ObservationStatusDegraded {
		return nil
	}
	report := evidence.RecordQuarantine{Records: observation.Records}
	for _, issue := range observation.Issues {
		report.Issues = append(report.Issues, evidence.QuarantinedRecord{Scope: issue.Scope, Code: issue.Code, Subject: issue.Subject})
	}
	if !report.Valid() {
		return nil
	}
	return &report
}

func usableEvidence(policy ScopePolicy, observation sources.Observation) bool {
	return complete(observation) || (policy.AllowRecordQuarantine && quarantinedRecords(observation) != nil)
}
