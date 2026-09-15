package artifact

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

const (
	// PublicationReceiptSchemaVersion identifies the admitted-source receipt format.
	PublicationReceiptSchemaVersion uint64 = 1
	// PublicationReceiptFilename names a receipt inside its separate immutable object.
	PublicationReceiptFilename = "starmap-catalog-run.json"
	// PublicationReceiptMediaType identifies the receipt's canonical JSON format.
	PublicationReceiptMediaType = "application/vnd.agentstation.starmap.catalog-run.v1+json"
	// MaxPublicationReceiptBytes bounds a canonical run receipt.
	MaxPublicationReceiptBytes = 4 << 20

	maxPublicationReceiptBytes = MaxPublicationReceiptBytes
	maxPublicationScopes       = 4096
	maxPublicationBindingID    = 4096
)

// PublicationArtifact binds the exact artifact and its immutable catalog content.
type PublicationArtifact struct {
	GenerationID    string `json:"generation_id"`
	CatalogChecksum string `json:"catalog_checksum"`
	PayloadChecksum string `json:"payload_checksum"`
	ArchiveChecksum string `json:"archive_checksum"`
}

// PublicationBinding identifies an acquisition binding without its private selectors.
// Checksum binds the complete declared binding, including its credential profile identity.
type PublicationBinding struct {
	ID         string              `json:"id"`
	Revision   string              `json:"revision"`
	ProviderID catalogs.ProviderID `json:"provider_id"`
	Checksum   string              `json:"checksum"`
}

// PublicationScopePolicy records the admission policy for one exact source scope.
type PublicationScopePolicy struct {
	Source                evidence.SourceID   `json:"source"`
	Binding               *PublicationBinding `json:"binding"`
	Required              bool                `json:"required"`
	Enabled               bool                `json:"enabled"`
	AllowMissing          bool                `json:"allow_missing"`
	AllowRecordQuarantine bool                `json:"allow_record_quarantine,omitempty"`
	MaxRetainedAge        time.Duration       `json:"max_retained_age_ns"`
	DisabledAction        string              `json:"disabled_action"`
}

// PublicationSourceReceipt distinguishes a source attempt from its admitted evidence.
// Attempt and EvidenceKind use the publication admission outcome names.
type PublicationSourceReceipt struct {
	Policy       PublicationScopePolicy          `json:"policy"`
	Attempt      string                          `json:"attempt"`
	EvidenceKind string                          `json:"evidence_kind"`
	Observation  *catalogs.SourceObservationLink `json:"observation"`
	Quarantine   *evidence.RecordQuarantine      `json:"quarantine,omitempty"`
}

// PublicationReceipt records admitted evidence for one verified catalog artifact.
// Branch promotion and channel publication retain their separate transition records.
type PublicationReceipt struct {
	SchemaVersion    uint64                     `json:"schema_version"`
	RunID            string                     `json:"run_id"`
	StartedAt        time.Time                  `json:"started_at"`
	CompletedAt      time.Time                  `json:"completed_at"`
	PolicyVersion    string                     `json:"policy_version"`
	Artifact         PublicationArtifact        `json:"artifact"`
	FreshAcquisition bool                       `json:"fresh_acquisition"`
	Sources          []PublicationSourceReceipt `json:"sources"`
	// Reviews report current unresolved offerings independently of the reused artifact.
	Reviews            []evidence.ReviewCandidate       `json:"reviews,omitempty"`
	ReviewObservations []catalogs.SourceObservationLink `json:"review_observations,omitempty"`
}

// Copy returns an independent receipt, including every source policy and observation.
func (r PublicationReceipt) Copy() PublicationReceipt {
	r.Sources = slices.Clone(r.Sources)
	for i := range r.Sources {
		if r.Sources[i].Quarantine != nil {
			report := *r.Sources[i].Quarantine
			report.Issues = slices.Clone(report.Issues)
			r.Sources[i].Quarantine = &report
		}
		if r.Sources[i].Policy.Binding != nil {
			binding := *r.Sources[i].Policy.Binding
			r.Sources[i].Policy.Binding = &binding
		}
		if r.Sources[i].Observation != nil {
			observation := *r.Sources[i].Observation
			r.Sources[i].Observation = &observation
		}
	}
	r.Reviews = slices.Clone(r.Reviews)
	r.ReviewObservations = slices.Clone(r.ReviewObservations)
	return r
}

// EncodePublicationReceipt validates and returns deterministic receipt bytes.
func EncodePublicationReceipt(receipt PublicationReceipt) ([]byte, error) {
	if err := receipt.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(receipt, "", channelIndent)
	if err != nil {
		return nil, publicationReceiptError("document", "cannot encode receipt")
	}
	data = append(data, '\n')
	if len(data) > maxPublicationReceiptBytes {
		return nil, publicationReceiptError("document", "exceeds the receipt byte limit")
	}
	return data, nil
}

// DecodePublicationReceipt accepts only validated canonical receipt bytes.
// Canonical encoding rejects duplicate, omitted, unknown, and differently cased fields.
func DecodePublicationReceipt(data []byte) (PublicationReceipt, error) {
	if len(data) == 0 || len(data) > maxPublicationReceiptBytes {
		return PublicationReceipt{}, publicationReceiptError("document", "must contain a bounded receipt")
	}
	receipt, err := decodePublicationReceipt(data)
	if err != nil {
		return PublicationReceipt{}, err
	}
	canonical, err := EncodePublicationReceipt(receipt)
	if err != nil {
		return PublicationReceipt{}, err
	}
	if !bytes.Equal(data, canonical) {
		return PublicationReceipt{}, publicationReceiptError("document", "must use canonical receipt encoding")
	}
	return receipt, nil
}

// VerifyPublicationReceipt checks the receipt digest and exact artifact binding.
// The caller must separately verify publisher provenance and artifact bytes.
func VerifyPublicationReceipt(data []byte, expectedChecksum string, expectedArtifact PublicationArtifact) (PublicationReceipt, error) {
	if len(data) == 0 || len(data) > maxPublicationReceiptBytes {
		return PublicationReceipt{}, publicationReceiptError("document", "must contain a bounded receipt")
	}
	if !publicationChecksum(expectedChecksum) || checksum(data) != expectedChecksum {
		return PublicationReceipt{}, publicationReceiptError("checksum", "does not match the selected receipt")
	}
	receipt, err := DecodePublicationReceipt(data)
	if err != nil {
		return PublicationReceipt{}, err
	}
	if receipt.Artifact != expectedArtifact {
		return PublicationReceipt{}, publicationReceiptError("artifact", "does not match the selected catalog artifact")
	}
	return receipt, nil
}
