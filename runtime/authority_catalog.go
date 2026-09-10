package runtime

import (
	"context"
	"slices"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// buildAuthorityCatalog preserves the authority's complete permitted catalog.
// Retained local observations cannot expand it when the source policy changes.
func (l *layerSet) buildAuthorityCatalog(baseline starmap.CatalogState) (starmap.CatalogState, error) {
	state, err := l.selectedBaseline(baseline)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	l.buildEvidence = starmap.CandidateEvidence{}
	l.acceptedSources = nil
	if l.source != nil && l.source.Manifest != nil {
		l.buildEvidence.SourceObservations = slices.Clone(l.source.Manifest.SourceObservations)
		l.buildEvidence.ReviewCandidates = slices.Clone(l.source.Manifest.ReviewCandidates)
	}
	l.sequence++
	state.Sequence = baseline.Sequence + l.sequence
	return state, nil
}

func (r *Runtime) validateAuthorityPublication(source *sourceLayer, localChanges int, removal *removalUpdate) error {
	if !r.requiresAuthority() {
		return nil
	}
	if source == nil || source.Manifest == nil || localChanges != 0 || removal != nil {
		return &errors.ConflictError{Resource: "catalog authority", Message: "only the configured authority can replace its permitted catalog"}
	}
	if source.Manifest.ManifestVersion != catalogs.AuthorityGenerationManifestVersion {
		return &errors.ValidationError{Field: "catalog_authority.manifest", Message: "requires an authority manifest"}
	}
	r.mu.RLock()
	p := r.permissions
	r.mu.RUnlock()
	_, err := p.activate(source.Manifest.AuthorityHead)
	return err
}

// activateAuthorityLocked joins catalog activation and permission state under mu.
// A withdrawal learned during publication leaves the new metadata unavailable for admission.
func (r *Runtime) activateAuthorityLocked(source *sourceLayer) {
	if !r.requiresAuthority() {
		return
	}
	if source == nil || source.Manifest == nil || r.effective.PayloadChecksum != r.client.CurrentCatalogState().PayloadChecksum {
		r.permissions.enforced = catalogs.CatalogAuthorityHead{}
		return
	}
	next, err := r.permissions.activate(source.Manifest.AuthorityHead)
	if err != nil {
		r.permissions.enforced = catalogs.CatalogAuthorityHead{}
		return
	}
	r.permissions = next
}

// publishAuthorityStartup aligns the serving client with the retained authority catalog.
// A lease follower never replaces shared state and stays unready when its client differs.
func (r *Runtime) publishAuthorityStartup(ctx context.Context) error {
	r.mu.RLock()
	layer, state, evidence := r.layers.source, r.effective, r.layers.buildEvidence
	r.mu.RUnlock()
	if layer == nil || r.lease.status() == leaseLost {
		return nil
	}
	current := r.client.CurrentCatalogState()
	if current.GenerationID != state.GenerationID || current.PayloadChecksum != state.PayloadChecksum {
		committed, err := r.commit(ctx, state, r.lease.epoch(), evidence)
		if err != nil {
			return err
		}
		state = committed
	}
	r.mu.Lock()
	r.effective = state
	r.activateAuthorityLocked(layer)
	r.mu.Unlock()
	return nil
}
