package runtime

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/sources"
)

// storedProviderPolicyRequired identifies local provider facts with a recorded binding scope.
// The exact compiled baseline carries publisher evidence independent of local declarations.
func storedProviderPolicyRequired(state, baseline starmap.CatalogState) bool {
	if baseline.Catalog != nil && baseline.GenerationID != "" && baseline.PayloadChecksum != "" &&
		state.GenerationID == baseline.GenerationID && state.PayloadChecksum == baseline.PayloadChecksum &&
		state.GeneratedAt.Equal(baseline.GeneratedAt) && state.AuthorityHead == baseline.AuthorityHead {
		return false
	}
	if state.Catalog == nil {
		return false
	}
	for _, entries := range state.Catalog.Provenance().Map() {
		for _, entry := range entries {
			if entry.Source == sources.ProvidersID && (entry.ProviderBindingID != "" || entry.ProviderBindingRevision != "") {
				return true
			}
		}
	}
	return false
}

// storedOriginMatchesBaseline checks whether the configured origin published the compiled baseline.
// Local provider acquisition still requires explicit bindings or retained input recovery.
func (r *Runtime) storedOriginMatchesBaseline(ctx context.Context, baseline starmap.CatalogState) (bool, error) {
	if r.config.origin == nil || baseline.Catalog == nil || baseline.GenerationID == "" {
		return false, nil
	}
	generation, err := r.client.CurrentGeneration(ctx)
	if err != nil {
		return false, err
	}
	if err := r.config.origin.validateCurrent(generation); err != nil {
		return false, err
	}
	return r.config.origin.matchesSource(generation, baseline)
}
