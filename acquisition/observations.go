package acquisition

import (
	"context"
	"fmt"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// PublishObservations reconciles caller-verified catalog observations and
// publishes the resulting immutable generation. This is the non-network
// acquisition seam for operator and tenant catalog facts.
func (s *Syncer) PublishObservations(
	ctx context.Context,
	observations ...sources.Observation,
) (starmap.Publication, error) {
	if s == nil || s.client == nil {
		return starmap.Publication{}, &errors.ValidationError{
			Field:   "acquisition.syncer",
			Message: "is required",
		}
	}
	if len(observations) == 0 {
		return starmap.Publication{}, &errors.ValidationError{
			Field:   "acquisition.observations",
			Message: "at least one observation is required",
		}
	}
	for index, observation := range observations {
		if err := observation.Validate(); err != nil {
			return starmap.Publication{}, errors.WrapResource(
				"validate",
				"catalog observation",
				observationIdentity(index, observation),
				err,
			)
		}
	}

	return s.client.Update(ctx, func(
		updateCtx context.Context,
		current *catalogs.Catalog,
	) (*starmap.Candidate, error) {
		result, err := pipeline.ReconcileObservations(updateCtx, current, observations)
		if err != nil {
			return nil, err
		}
		if (result.Changeset == nil || !result.Changeset.HasChanges()) &&
			len(result.ReviewCandidates) == 0 {
			return nil, nil
		}
		catalog, err := result.Catalog.Build()
		if err != nil {
			return nil, errors.WrapResource("publish", "observed catalog candidate", "", err)
		}
		links := make([]catalogs.SourceObservationLink, 0, len(observations))
		for _, observation := range observations {
			links = append(links, observation.Link())
		}
		return starmap.NewCandidate(catalog, starmap.CandidateEvidence{
			SourceObservations: links,
			ReviewCandidates:   result.ReviewCandidates,
		})
	})
}

func observationIdentity(index int, observation sources.Observation) string {
	if observation.ID != "" {
		return observation.ID
	}
	return string(observation.SourceID) + "[" + fmt.Sprint(index) + "]"
}
