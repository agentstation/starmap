package operations

import (
	"github.com/agentstation/starmap/pkg/sources"
	"testing"
)

func TestStatusCopyOwnsSourceActivity(t *testing.T) {
	original := Status{Detail: map[string]any{
		"accepted_sources":  sources.AcceptedSourceState{GenerationID: "prior", Sources: []sources.ID{sources.ModelsDevHTTPID}},
		"source_activities": []sources.SourceActivity{{Source: sources.ProvidersID}},
		"provider_attempts": []sources.ProviderAttempt{{ProviderID: "openai"}},
	}}
	copied := original.Copy()
	copied.Detail["accepted_sources"].(sources.AcceptedSourceState).Sources[0] = sources.LocalCatalogID
	if original.Detail["accepted_sources"].(sources.AcceptedSourceState).Sources[0] != sources.ModelsDevHTTPID {
		t.Fatal("status copy aliases accepted input")
	}
	copied.Detail["source_activities"].([]sources.SourceActivity)[0].Source = sources.LocalCatalogID
	copied.Detail["provider_attempts"].([]sources.ProviderAttempt)[0].ProviderID = "changed"
	if original.Detail["source_activities"].([]sources.SourceActivity)[0].Source != sources.ProvidersID || original.Detail["provider_attempts"].([]sources.ProviderAttempt)[0].ProviderID != "openai" {
		t.Fatal("status copy aliases retained activity")
	}
}
