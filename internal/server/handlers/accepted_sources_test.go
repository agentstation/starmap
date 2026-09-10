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

func TestReadinessSeparatesSourceActivityFromAcceptedInput(t *testing.T) {
	report := runtimeReadiness(status.Status{
		SourceConfiguration:        []sources.SourceActivity{{Source: sources.LocalCatalogID, Supported: true, Enabled: true, Eligibility: sources.EligibilityUnknown}},
		SourceActivities:           []sources.SourceActivity{{Source: sources.LocalCatalogID, Supported: true, Enabled: true, Eligibility: sources.EligibilityIneligible}},
		AcceptedAcquisitionSources: []sources.ID{sources.ModelsDevHTTPID},
	})
	configuration, ok := report["source_configuration"].([]sources.SourceActivity)
	if !ok || len(configuration) != 1 || configuration[0].Eligibility != sources.EligibilityUnknown || !configuration[0].Enabled {
		t.Fatalf("readiness configuration=%v", report)
	}

	activity, ok := report["source_activities"].([]sources.SourceActivity)
	if !ok || len(activity) != 1 || activity[0].Source != sources.LocalCatalogID || activity[0].Eligibility != sources.EligibilityIneligible {
		t.Fatalf("readiness activity=%v", report)
	}
	accepted := report["accepted_acquisition_sources"].([]sources.ID)
	if len(accepted) != 1 || accepted[0] != sources.ModelsDevHTTPID {
		t.Fatalf("readiness accepted input=%v", report)
	}
}
