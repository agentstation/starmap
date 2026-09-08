package runtime

import (
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/sources"
)

// storedProviderPolicyRequired identifies provider facts with a recorded binding scope.
// Startup must apply current declarations before it serves those accepted bytes.
func storedProviderPolicyRequired(state starmap.CatalogState) bool {
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
