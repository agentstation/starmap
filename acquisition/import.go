package acquisition

import (
	"context"
	"os"
	"reflect"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/projection"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// ImportResult describes a verified release observation and any local
// generation it produced after authority-aware reconciliation.
type ImportResult struct {
	// SourceGenerationID identifies the exact verified release generation.
	SourceGenerationID string
	// Publication identifies the locally committed reconciled generation.
	Publication starmap.Publication
	// Projection reports the post-commit human-workspace projection.
	Projection *projection.Result
}

// ImportRelease verifies a portable catalog release. It reconciles the release
// as a trusted, low-authority observation with the current catalog and human
// workspace, then publishes the result atomically. A verification failure cannot
// mutate the client. ImportRelease never activates the release wholesale.
// Independent membership scopes retain their original receipts. Conflicting
// records for one publisher and binding revision require explicit source selection.
func (s *Syncer) ImportRelease(
	ctx context.Context,
	release artifact.Release,
	verifier artifact.PublisherVerifier,
) (*ImportResult, error) {
	if s == nil || s.client == nil {
		return nil, &errors.ValidationError{
			Field:   "acquisition.syncer",
			Message: "is required",
		}
	}
	projectionOptions := &pkgsync.Options{CatalogPath: s.client.WorkspacePath(), SourceDirectories: s.sourceDirectories}
	if projectionOptions.CatalogPath != "" {
		directories, err := projectionSourceDirectories(projectionOptions.CatalogPath, projectionOptions)
		if err != nil {
			return nil, err
		}
		projectionOptions.SourceDirectories = directories
	}
	generation, err := artifact.VerifyRelease(ctx, release, verifier)
	if err != nil {
		return nil, err
	}
	releaseCatalog, err := catalogs.DecodeCatalogGeneration(generation)
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
	// Merge only scope records here. Reconciliation owns catalog fact authority.
	membership := catalogs.NewEmpty()
	if err := membership.SetMembershipScopes(releaseCatalog.MembershipScopes()); err != nil {
		return nil, err
	}

	var candidateCatalog *catalogs.Catalog
	publication, err := s.client.Update(ctx, func(
		updateCtx context.Context,
		current *catalogs.Catalog,
	) (*starmap.Candidate, error) {
		result, reconcileErr := reconciler.ReconcileObservations(
			updateCtx,
			current,
			observations,
		)
		if reconcileErr != nil {
			return nil, reconcileErr
		}
		if err := result.Catalog.MergeWith(membership); err != nil {
			return nil, err
		}
		scopes := result.Catalog.MembershipScopes()
		scopesChanged := !reflect.DeepEqual(current.MembershipScopes(), scopes)
		if (result.Changeset == nil || !result.Changeset.HasChanges()) &&
			len(result.ReviewCandidates) == 0 && !input.RequiresSeed() && !scopesChanged {
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
		var retained []catalogs.SourceObservationLink
		if len(current.MembershipScopes()) != 0 {
			// Update holds the mutation transaction, so this manifest matches current.
			previous, err := s.client.CurrentGeneration(updateCtx)
			if err != nil {
				return nil, err
			}
			retained = previous.Manifest.SourceObservations
		}
		links, reconcileErr = importMembershipEvidence(scopes, links, retained, generation.Manifest.SourceObservations)
		if reconcileErr != nil {
			return nil, reconcileErr
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
			projectionOptions,
		)
	}
	return result, nil
}

func releaseObservation(
	generation catalogs.Generation,
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
	var input workspace.InputExpectation
	var observation *sources.Observation
	err := workspace.Read(ctx, path, func(observed workspace.InputExpectation) error {
		input = observed
		if !input.Exists {
			return nil
		}
		builder, err := catalogs.NewFromPath(input.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return &errors.ConflictError{
					Resource: "human catalog workspace", Expected: input.Path,
					Actual: "removed during release import",
				}
			}
			return errors.WrapResource("load", "human catalog workspace", input.Path, err)
		}
		catalog, err := catalogs.NewObservationCatalog(builder)
		if err != nil {
			return errors.WrapResource("observe", "human catalog workspace", input.Path, err)
		}
		input, err = workspace.BindInputCatalog(input, catalog)
		if err != nil {
			return err
		}
		value, err := local.New(local.WithCatalogReport(catalog, builder.LoadReport())).Observe(ctx)
		if err != nil {
			return err
		}
		observation = &value
		return nil
	})
	if err != nil {
		return workspace.InputExpectation{}, nil, err
	}
	return input, observation, nil
}
