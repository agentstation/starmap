package operations

import (
	"github.com/agentstation/starmap/pkg/sources"
	"testing"
)

func TestStatusCopyOwnsSourceFailures(t *testing.T) {
	original := Status{Detail: map[string]any{"source_failures": []sources.SourceFailure{{Source: sources.ModelsDevGitID, Reason: sources.ProviderReasonDependencyUnavailable}}}}
	copied := original.Copy()
	copied.Detail["source_failures"].([]sources.SourceFailure)[0].Source = sources.LocalCatalogID
	if original.Detail["source_failures"].([]sources.SourceFailure)[0].Source != sources.ModelsDevGitID {
		t.Fatal("status copy exposed retained source failures")
	}
}
