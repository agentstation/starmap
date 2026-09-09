package handlers

import (
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime/status"
	"testing"
)

func TestReadinessIncludesAcceptedAcquisitionSources(t *testing.T) {
	report := runtimeReadiness(status.Status{GenerationID: "accepted", AcceptedAcquisitionSources: []sources.ID{sources.LocalCatalogID}})
	accepted, ok := report["accepted_acquisition_sources"].([]sources.ID)
	if !ok || len(accepted) != 1 || accepted[0] != sources.LocalCatalogID || report["generation_id"] != "accepted" {
		t.Fatalf("readiness source report=%v", report)
	}
}
