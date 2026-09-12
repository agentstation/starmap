package runtime

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// providerReviewIdentity keeps independent account revisions and opaque model IDs separate.
type providerReviewIdentity struct {
	scope providerEvidenceKey
	model string
	code  evidence.ReviewCandidateCode
}

// selectCurrentProviderReviews retains one current review per offering and scope revision.
// Omission creates no replacement. Metadata reviews retain their existing source contract.
// The original observation history remains available for later replay and audit.
func selectCurrentProviderReviews(ctx context.Context, observations []sources.Observation, reviews []evidence.ReviewCandidate) ([]evidence.ReviewCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(reviews) == 0 {
		return reviews, nil
	}
	byID := make(map[string]sources.Observation, len(observations))
	for _, observation := range observations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		byID[observation.ID] = observation
	}
	winners := make(map[providerReviewIdentity]int)
	retained := make([]bool, len(reviews))
	for index, candidate := range reviews {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		retained[index] = true
		if candidate.SourceID != sources.ProvidersID {
			continue
		}
		observation, found := byID[candidate.SourceObservationID]
		if !found || observation.SourceID != candidate.SourceID || observation.EvidenceChecksum != candidate.EvidenceChecksum || observation.Revision != candidate.SourceRevision {
			return nil, &errors.ConflictError{Resource: "provider review evidence", Message: "review does not match its original observation"}
		}
		key := providerReviewIdentity{scope: observationProviderKey(observation, catalogs.ProviderID(candidate.ProviderID)), model: candidate.ProviderModelID, code: candidate.Code}
		previous, exists := winners[key]
		if !exists {
			winners[key] = index
			continue
		}
		prior := byID[reviews[previous].SourceObservationID]
		order := reconciler.CompareProviderObservations(prior, observation)
		if order == 0 {
			order = strings.Compare(prior.ID, observation.ID)
		}
		if order <= 0 {
			retained[previous] = false
			winners[key] = index
		} else {
			retained[index] = false
		}
	}
	selected := make([]evidence.ReviewCandidate, 0, len(reviews))
	for index, candidate := range reviews {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if retained[index] {
			selected = append(selected, candidate)
		}
	}
	return selected, nil
}
