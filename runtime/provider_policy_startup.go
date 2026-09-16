package runtime

import (
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
