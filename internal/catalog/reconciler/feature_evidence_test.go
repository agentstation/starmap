package reconciler

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestReasoningDenialClearsUnsupportedCapabilityEvidence(t *testing.T) {
	policies := authority.New()
	merger := newMerger(policies, NewAuthorityStrategy(policies), nil)
	denied := &catalogs.ModelFeatures{}
	denied.SetSupport(catalogs.ModelFeatureReasoning, false)
	prior := &catalogs.ModelFeatures{}
	prior.SetSupport(catalogs.ModelFeatureReasoning, true)
	prior.SetSupport(catalogs.ModelFeatureReasoningEffort, true)
	model, history := merger.model("provider", "model", map[sources.ID]*catalogs.Model{
		sources.ProvidersID:     {ID: "model", Features: denied},
		sources.ModelsDevHTTPID: {ID: "model", Features: prior},
	})
	if value, _ := model.Features.Support(catalogs.ModelFeatureReasoningEffort); value {
		t.Fatal("reasoning denial retained effort support")
	}
	entry := history["Features.reasoning_effort"].Current
	if entry.Value != false || entry.Source != "" || entry.ObservationID != "" {
		t.Fatalf("cleared capability retains source evidence: %+v", entry)
	}
}
