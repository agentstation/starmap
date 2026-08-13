package catalogs

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/provenance"
)

func TestProvenanceFindModelScopesSharedIDsByProvider(t *testing.T) {
	t.Parallel()

	container := NewProvenance()
	container.Set(provenance.Map{
		"model:provider-a/shared:pricing": {{Value: "a"}},
		"model:provider-b/shared:pricing": {{Value: "b"}},
	})

	if got := container.FindModelField("provider-a", "shared", "pricing"); len(got) != 1 || got[0].Value != "a" {
		t.Fatalf("provider-a/shared pricing = %#v", got)
	}
	if got := container.FindModelField("provider-b", "shared", "pricing"); len(got) != 1 || got[0].Value != "b" {
		t.Fatalf("provider-b/shared pricing = %#v", got)
	}
	if got := container.FindByResource(evidence.ResourceTypeModel, "shared"); len(got) != 0 {
		t.Fatalf("bare shared model lookup combined scoped evidence: %#v", got)
	}
}
