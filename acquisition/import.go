package acquisition

import (
	"context"
	"os"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/projection"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ImportResult describes a verified release observation and any local
// generation it produced after authority-aware reconciliation.
type ImportResult struct {
	// SourceGenerationID identifies the exact verified release generation.
	SourceGenerationID string
	// Publication identifies the locally committed reconciled generation.
	Publication starmap.Publication
	// Projection reports the post-commit human-workspace projection.
	Projection *projection.ProjectionResult
}

// ImportRelease verifies a portable catalog release, reconciles it as a trusted
// low-authority observation with the current catalog and human workspace, then
// atomically publishes the result. Verification failure cannot mutate the
// client. The release generation itself is never activated wholesale.
func (s *Syncer) ImportRelease(
	ctx context.Context,
	release catalogartifact.Release,
	verifier catalogartifact.PublisherVerifier,
) (*ImportResult, error) {
	if s == nil || s.client == nil {
		return nil, &errors.ValidationError{
			Field:   "acquisition.syncer",
			Message: "is required",
		}
	}
	generation, err := catalogartifact.VerifyRelease(ctx, release, verifier)
	if err != nil {
		return nil, err
	}
	releaseCatalog, err := catalogstore.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return nil, errors.WrapResource(
			"decode",
			"catalog release generation",
			generation.Manifest.GenerationID,
			err,
		)
	}
	releaseObservation, err := releaseObservation(generation, releaseCatalog)
	if err != nil {
		return nil, err
	}
	input, localObservation, err := observeImportWorkspace(
		ctx,
		s.client.WorkspacePath(),
	)
	if err != nil {
		return nil, err
	}
	observations := make([]sources.Observation, 0, 2)
	if localObservation != nil {
		observations = append(observations, *localObservation)
	}
	observations = append(observations, releaseObservation)

	var candidateCatalog *catalogs.Catalog
	publication, err := s.client.Update(ctx, func(
		updateCtx context.Context,
		current *catalogs.Catalog,
	) (*starmap.Candidate, error) {
		result, reconcileErr := pipeline.ReconcileObservations(
			updateCtx,
			current,
			observations,
		)
		if reconcileErr != nil {
			return nil, reconcileErr
		}
		if (result.Changeset == nil || !result.Changeset.HasChanges()) &&
			len(result.ReviewCandidates) == 0 && !input.RequiresSeed() {
			return nil, nil
		}
		candidateCatalog, reconcileErr = result.Catalog.Build()
		if reconcileErr != nil {
			return nil, errors.WrapResource(
				"publish",
				"reconciled catalog release",
				generation.Manifest.GenerationID,
				reconcileErr,
			)
		}
		links := make([]catalogs.SourceObservationLink, 0, len(observations))
		for _, observation := range observations {
			links = append(links, observation.Link())
		}
		return starmap.NewCandidate(candidateCatalog, starmap.CandidateEvidence{
			SourceObservations: links,
			ReviewCandidates:   result.ReviewCandidates,
		})
	})
	if err != nil {
		return nil, err
	}

	result := &ImportResult{
		SourceGenerationID: generation.Manifest.GenerationID,
		Publication:        publication,
	}
	if publication.Published && input.Path != "" {
		result.Projection = projectCommittedCatalog(
			ctx,
			candidateCatalog,
			input.Path,
			publication,
			input,
		)
	}
	return result, nil
}

func releaseObservation(
	generation catalogstore.Generation,
	catalog *catalogs.Catalog,
) (sources.Observation, error) {
	accepted := len(catalog.Providers().List()) +
		len(catalog.Authors().List()) +
		len(catalog.AuthoredModels())
	for _, provider := range catalog.Providers().List() {
		accepted += len(provider.Models)
	}
	observation, err := sources.NewObservation(
		sources.ReleaseArtifactID,
		catalog,
		sources.ObservationMetadata{
			ObservedAt: time.Now().UTC(),
			Revision: sources.Revision{
				Kind:  sources.RevisionKindSourceVersion,
				Value: generation.Manifest.GenerationID,
			},
			Completeness: sources.ObservationCompletenessComplete,
			Status:       sources.ObservationStatusSucceeded,
			Records:      sources.ObservationRecordCounts{Accepted: accepted},
		},
	)
	if err != nil {
		return sources.Observation{}, errors.WrapResource(
			"observe",
			"catalog release generation",
			generation.Manifest.GenerationID,
			err,
		)
	}
	return observation, nil
}

func observeImportWorkspace(
	ctx context.Context,
	path string,
) (workspace.InputExpectation, *sources.Observation, error) {
	input, err := workspace.ObserveInput(path)
	if err != nil {
		return workspace.InputExpectation{}, nil, err
	}
	if !input.Exists {
		return input, nil, nil
	}
	builder, err := catalogs.NewFromPath(input.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return input, nil, &errors.ConflictError{
				Resource: "human catalog workspace",
				Expected: input.Path,
				Actual:   "removed during release import",
			}
		}
		return input, nil, errors.WrapResource(
			"load",
			"human catalog workspace",
			input.Path,
			err,
		)
	}
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		return input, nil, errors.WrapResource(
			"observe",
			"human catalog workspace",
			input.Path,
			err,
		)
	}
	input, err = workspace.BindInputCatalog(input, catalog)
	if err != nil {
		return workspace.InputExpectation{}, nil, err
	}
	observation, err := local.New(
		local.WithCatalogReport(catalog, builder.LoadReport()),
	).Observe(ctx)
	if err != nil {
		return workspace.InputExpectation{}, nil, err
	}
	return input, &observation, nil
}
