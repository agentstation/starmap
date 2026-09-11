package runtime

import (
	"bytes"
	"context"
	"encoding/json"
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
	if source == nil || source.Manifest == nil || source.Manifest.AuthorityHead != r.effective.AuthorityHead || source.Manifest.GenerationID != r.effective.GenerationID || source.Manifest.Payload.Checksum != r.effective.PayloadChecksum || !r.authorityClientMatches(r.effective) {
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

func (r *Runtime) authorityClientMatches(state starmap.CatalogState) bool {
	current := r.client.CurrentCatalogState()
	return state.GenerationID == current.GenerationID && state.PayloadChecksum == current.PayloadChecksum
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
	committed, err := r.commit(ctx, state, r.lease.epoch(), evidence, layer)
	if err != nil {
		return err
	}
	state = committed
	r.mu.Lock()
	r.effective = state
	r.activateAuthorityLocked(layer)
	r.mu.Unlock()
	return nil
}

// commitAuthority preserves the complete upstream generation in the serving store.
// Rebuilding a candidate from catalog facts would discard its authority manifest.
func (r *Runtime) commitAuthority(ctx context.Context, state starmap.CatalogState, source *sourceLayer, epoch uint64) (starmap.CatalogState, error) {
	if source == nil || source.Manifest == nil {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "catalog_authority.generation", Message: "requires the original source generation"}
	}
	manifest := source.Manifest
	if manifest.ManifestVersion != catalogs.AuthorityGenerationManifestVersion || manifest.GenerationID != state.GenerationID || manifest.Payload.Checksum != state.PayloadChecksum {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "catalog_authority.generation", Message: "must match the selected authoritative catalog"}
	}
	if err := r.lease.fence(epoch); err != nil {
		return starmap.CatalogState{}, err
	}
	if state.GenerationID == r.client.CurrentGenerationID() {
		retained, err := r.client.CurrentGeneration(ctx)
		if err != nil {
			return starmap.CatalogState{}, err
		}
		if !source.matchesGeneration(retained) {
			return starmap.CatalogState{}, &errors.ConflictError{Resource: "catalog authority generation", Message: "the serving store contains different content for the selected generation"}
		}
		return r.client.CurrentCatalogState(), nil
	}
	generation := catalogs.Generation{Manifest: manifest.Copy(), Payload: source.Payload}
	if _, err := r.client.Activate(ctx, generation); err != nil {
		return starmap.CatalogState{}, err
	}
	return r.client.CurrentCatalogState(), nil
}

func (s *sourceLayer) matchesGeneration(generation catalogs.Generation) bool {
	if s.Manifest == nil || !bytes.Equal(s.Payload, generation.Payload) {
		return false
	}
	selected, selectedErr := json.Marshal(s.Manifest)
	retained, retainedErr := json.Marshal(generation.Manifest)
	return selectedErr == nil && retainedErr == nil && bytes.Equal(selected, retained)
}
