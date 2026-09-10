package runtime

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
)

// preparedPublication binds the journal identity to the exact generation that commit activates.
type preparedPublication struct {
	state      starmap.CatalogState
	generation *catalogs.Generation
	expected   string
}

func (r *Runtime) preparePublication(ctx context.Context, state starmap.CatalogState, evidence starmap.CandidateEvidence, source *sourceLayer) (preparedPublication, error) {
	if err := ctx.Err(); err != nil {
		return preparedPublication{}, err
	}
	if err := r.State().Catalog.CanonicalAliases().ValidateSuccessor(state.Catalog.CanonicalAliases()); err != nil {
		return preparedPublication{}, err
	}
	prepared := preparedPublication{state: state}
	origin := r.config.origin
	if origin == nil {
		return prepared, nil
	}
	if source != nil && source.Manifest != nil && source.Manifest.AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		return preparedPublication{}, originError("an upstream authority must retain its original identity")
	}
	current, err := origin.publisher.Current(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return preparedPublication{}, err
	}
	if err == nil {
		if current.Manifest.GenerationID != r.client.CurrentGenerationID() {
			return preparedPublication{}, &errors.ConflictError{Resource: "origin serving catalog", Expected: r.client.CurrentGenerationID(), Actual: current.Manifest.GenerationID}
		}
		prepared.expected = current.Manifest.GenerationID
		if current.Manifest.AuthorityHead == (catalogs.CatalogAuthorityHead{}) {
			if !origin.config.Bootstrap {
				return preparedPublication{}, originError("an existing ordinary catalog requires explicit bootstrap")
			}
		} else {
			if err := origin.validateCurrent(current); err != nil {
				return preparedPublication{}, err
			}
			unchanged, err := origin.matchesSource(current, state)
			if err != nil {
				return preparedPublication{}, err
			}
			if unchanged {
				prepared.state = r.client.CurrentCatalogState()
				prepared.generation = &current
				return prepared, nil
			}
		}
	}
	var opts []starmap.CandidateOption
	if state.GenerationID != "" {
		opts = append(opts, starmap.WithCandidateGenerationID(state.GenerationID))
	}
	candidate, err := starmap.NewCandidate(state.Catalog, evidence, opts...)
	if err != nil {
		return preparedPublication{}, err
	}
	input, err := r.client.PrepareGeneration(ctx, candidate)
	if err != nil {
		return preparedPublication{}, err
	}
	generation, err := origin.publisher.PrepareCatalog(ctx, input, prepared.expected)
	if err != nil {
		return preparedPublication{}, err
	}
	prepared.generation = &generation
	prepared.state.GenerationID = generation.Manifest.GenerationID
	prepared.state.PayloadChecksum = generation.Manifest.Payload.Checksum
	prepared.state.GeneratedAt = generation.Manifest.GeneratedAt
	return prepared, nil
}

func (origin *authorityOrigin) validateCurrent(current catalogs.Generation) error {
	if _, err := catalogs.DecodeCatalogGeneration(current); err != nil {
		return err
	}
	head := current.Manifest.AuthorityHead
	if head.AuthorityID != origin.config.AuthorityID || head.PolicyID != origin.config.PolicyID || !head.SupportsPermissions() {
		return originError("stored authority does not match the configured identity and supported permission schema")
	}
	return nil
}

// matchesSource proves an unchanged source identity against the origin's immutable digest.
// Restoring the ordinary manifest retains its exact observation times, evidence, and sync-run identity.
func (origin *authorityOrigin) matchesSource(current catalogs.Generation, state starmap.CatalogState) (bool, error) {
	if state.PayloadChecksum != current.Manifest.Payload.Checksum {
		return false, nil
	}
	if state.GenerationID == current.Manifest.GenerationID {
		return true, nil
	}
	if state.GenerationID == "" {
		return false, nil
	}
	input := current.Copy()
	input.Manifest.ManifestVersion = catalogs.CurrentGenerationManifestVersion
	input.Manifest.GenerationID = state.GenerationID
	input.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{}
	derived, err := permission.PrepareGeneration(input, permission.GenerationConfig{AuthorityID: origin.config.AuthorityID, PolicyID: origin.config.PolicyID, Sequence: current.Manifest.AuthorityHead.Sequence})
	if err != nil {
		return false, err
	}
	return derived.Manifest.GenerationID == current.Manifest.GenerationID, nil
}

func (r *Runtime) commitPrepared(ctx context.Context, prepared preparedPublication, epoch uint64, evidence starmap.CandidateEvidence, source *sourceLayer) (starmap.CatalogState, error) {
	if prepared.generation == nil {
		return r.commitOrdinary(ctx, prepared.state, epoch, evidence, source)
	}
	if err := ctx.Err(); err != nil {
		return starmap.CatalogState{}, err
	}
	if err := r.lease.fence(epoch); err != nil {
		return starmap.CatalogState{}, err
	}
	if _, err := r.client.Activate(r.authorityPublicationContext(ctx), *prepared.generation); err != nil {
		return starmap.CatalogState{}, err
	}
	return r.client.CurrentCatalogState(), nil
}

func (r *Runtime) commit(ctx context.Context, state starmap.CatalogState, epoch uint64, evidence starmap.CandidateEvidence, source *sourceLayer) (starmap.CatalogState, error) {
	prepared, err := r.preparePublication(ctx, state, evidence, source)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	return r.commitPrepared(ctx, prepared, epoch, evidence, source)
}

func (r *Runtime) publishOriginStartup(ctx context.Context) error {
	r.mu.RLock()
	state, evidence, source := r.effective, r.layers.buildEvidence, r.layers.source
	r.mu.RUnlock()
	if r.lease.status() == leaseLost {
		return originError("origin startup requires the publication lease")
	}
	committed, err := r.commit(ctx, state, r.lease.epoch(), evidence, source)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.effective = committed
	r.mu.Unlock()
	return nil
}
