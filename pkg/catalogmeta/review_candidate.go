package catalogmeta

import "strings"

// ReviewCandidateCode identifies a stable model-review reason.
type ReviewCandidateCode string

const (
	// ReviewCandidateUnresolvedModelReference means a provider offering has no
	// reviewed provider-independent model identity.
	ReviewCandidateUnresolvedModelReference ReviewCandidateCode = "unresolved_model_reference"
)

// ReviewCandidate records one provider offering excluded from a canonical
// catalog generation. ProviderModelID remains the provider's exact opaque ID.
type ReviewCandidate struct {
	Code                   ReviewCandidateCode `json:"code" yaml:"code"`
	ProviderID             string              `json:"provider_id" yaml:"provider_id"`
	ProviderModelID        string              `json:"provider_model_id" yaml:"provider_model_id"`
	SourceID               SourceID            `json:"source" yaml:"source"`
	SourceObservationID    string              `json:"source_observation_id" yaml:"source_observation_id"`
	SourceRevision         ObservationRevision `json:"source_revision" yaml:"source_revision"`
	EvidenceChecksum       string              `json:"evidence_checksum" yaml:"evidence_checksum"`
	Reason                 string              `json:"reason" yaml:"reason"`
	PriorReviewedModelLink string              `json:"prior_reviewed_model_link" yaml:"prior_reviewed_model_link"`
}

// CompareReviewCandidates returns a stable lexical order for review candidates.
func CompareReviewCandidates(left, right ReviewCandidate) int {
	leftValues := []string{
		left.ProviderID,
		left.ProviderModelID,
		string(left.Code),
		left.SourceID.String(),
		left.SourceObservationID,
	}
	rightValues := []string{
		right.ProviderID,
		right.ProviderModelID,
		string(right.Code),
		right.SourceID.String(),
		right.SourceObservationID,
	}
	for index := range leftValues {
		if result := strings.Compare(leftValues[index], rightValues[index]); result != 0 {
			return result
		}
	}
	return 0
}
