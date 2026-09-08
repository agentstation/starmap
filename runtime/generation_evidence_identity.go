package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
)

// effectiveEvidenceChecksum binds catalog bytes and publication evidence to one identity.
// A receipt can change without supplying a selected catalog field.
func effectiveEvidenceChecksum(payloadChecksum string, candidate starmap.CandidateEvidence) (string, error) {
	if len(candidate.SourceObservations) == 0 && len(candidate.ReviewCandidates) == 0 {
		return payloadChecksum, nil
	}
	observations := append([]catalogs.SourceObservationLink{}, candidate.SourceObservations...)
	slices.SortFunc(observations, func(left, right catalogs.SourceObservationLink) int {
		if order := strings.Compare(left.Source.String(), right.Source.String()); order != 0 {
			return order
		}
		return strings.Compare(left.ObservationID, right.ObservationID)
	})
	reviews := append([]evidence.ReviewCandidate{}, candidate.ReviewCandidates...)
	slices.SortFunc(reviews, evidence.CompareReviewCandidates)
	raw, err := json.Marshal(struct {
		Domain          string
		PayloadChecksum string
		Observations    []catalogs.SourceObservationLink
		Reviews         []evidence.ReviewCandidate
	}{"starmap-effective-evidence:v1", payloadChecksum, observations, reviews})
	if err != nil {
		return "", errors.WrapResource("encode", "effective generation evidence", "", err)
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
