package sources

import (
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ObservationIssueReceipt retains classified evidence without diagnostic messages.
type ObservationIssueReceipt struct {
	Scope   ObservationIssueScope `json:"scope"`
	Code    ObservationIssueCode  `json:"code"`
	Subject string                `json:"subject,omitempty"`
}

// ObservationReceipt retains the evidence needed to validate and restore an observation.
// It excludes the catalog payload and diagnostic messages that can contain credentials.
type ObservationReceipt struct {
	ProviderBinding *ProviderAcquisitionBinding    `json:"provider_binding,omitempty"`
	Link            catalogs.SourceObservationLink `json:"link"`
	Records         ObservationRecordCounts        `json:"records"`
	Issues          []ObservationIssueReceipt      `json:"issues,omitempty"`
}

// Receipt validates an observation and copies its durable evidence without diagnostic messages.
func (o Observation) Receipt() (ObservationReceipt, error) {
	if err := o.Validate(); err != nil {
		return ObservationReceipt{}, err
	}
	receipt := ObservationReceipt{ProviderBinding: cloneProviderBinding(o.ProviderBinding), Link: o.Link(), Records: o.Records}
	for _, issue := range o.Issues {
		receipt.Issues = append(receipt.Issues, ObservationIssueReceipt{Scope: issue.Scope, Code: issue.Code, Subject: issue.Subject})
	}
	return receipt, nil
}

// Clone returns receipt data whose mutable collections belong to the caller.
func (r ObservationReceipt) Clone() ObservationReceipt {
	r.Issues = slices.Clone(r.Issues)
	r.ProviderBinding = cloneProviderBinding(r.ProviderBinding)
	return r
}

// Restore validates the receipt identity and checksum against the supplied catalog.
// Restored diagnostics contain only stable issue codes. Original messages remain absent.
func (r ObservationReceipt) Restore(catalog *catalogs.Catalog) (Observation, error) {
	observation := Observation{
		ProviderBinding: cloneProviderBinding(r.ProviderBinding),
		ID:              r.Link.ObservationID, SourceID: r.Link.Source, ObservedAt: r.Link.ObservedAt,
		Revision: r.Link.Revision, Completeness: r.Link.Completeness, Status: r.Link.Status,
		EvidenceChecksum: r.Link.EvidenceChecksum, Catalog: catalog, Records: r.Records,
	}
	for _, issue := range r.Issues {
		observation.Issues = append(observation.Issues, ObservationIssue{
			Scope: issue.Scope, Code: issue.Code, Subject: issue.Subject, Message: string(issue.Code),
		})
	}
	if err := observation.Validate(); err != nil {
		return Observation{}, err
	}
	return observation, nil
}
