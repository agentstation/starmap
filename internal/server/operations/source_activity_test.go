package operations

import (
	"github.com/agentstation/starmap/pkg/sources"
	"testing"
)

func TestStatusCopyOwnsSourceActivity(t *testing.T) {
	original := Status{Detail: map[string]any{
		"source_activities": []sources.SourceActivity{{Source: sources.ProvidersID}},
		"provider_attempts": []sources.ProviderAttempt{{ProviderID: "openai"}},
	}}
	copied := original.Copy()
	copied.Detail["source_activities"].([]sources.SourceActivity)[0].Source = sources.LocalCatalogID
	copied.Detail["provider_attempts"].([]sources.ProviderAttempt)[0].ProviderID = "changed"
	if original.Detail["source_activities"].([]sources.SourceActivity)[0].Source != sources.ProvidersID || original.Detail["provider_attempts"].([]sources.ProviderAttempt)[0].ProviderID != "openai" {
		t.Fatal("status copy aliases retained activity")
	}
}
