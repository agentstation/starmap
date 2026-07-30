package modelsdev

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// TestF009MalformedModelsDevSiblingIsQuarantined proves a malformed model does
// not discard a valid sibling and becomes observation degradation evidence.
func TestF009MalformedModelsDevSiblingIsQuarantined(t *testing.T) {
	payload := []byte(`{
		"provider": {
			"id": "provider",
			"name": "Provider",
			"models": {
				"valid": {
					"id": "valid",
					"name": "Valid",
					"limit": {"context": 8192, "input": 4096, "output": 4096}
				},
				"invalid": {
					"id": "invalid",
					"name": "Invalid",
					"limit": {"context": "schema-drift", "input": 4096, "output": 4096}
				}
			}
		}
	}`)

	api, err := parseAPIData(payload)
	if err != nil {
		t.Fatalf("parseAPIData: %v", err)
	}
	provider := (*api)["provider"]
	if len(provider.Models) != 1 || provider.Models["valid"].ID != "valid" {
		t.Fatalf("valid models = %#v, want only valid sibling", provider.Models)
	}
	if provider.RecordReport.Rejected != 1 || len(provider.RecordReport.Issues) != 1 {
		t.Fatalf("decode report = %#v, want one rejected record", provider.RecordReport)
	}

	builder := catalogs.NewEmpty()
	added, rejected, issues, err := processFetch(builder, api, nil)
	if err != nil {
		t.Fatalf("processFetch: %v", err)
	}
	if added != 1 || rejected != 1 {
		t.Fatalf("records = accepted %d rejected %d, want 1/1", added, rejected)
	}
	if len(issues) != 1 ||
		issues[0].Scope != sources.ObservationIssueScopeRecord ||
		issues[0].Code != sources.ObservationIssueCodeInvalidRecord {
		t.Fatalf("issues = %#v, want one invalid-record issue", issues)
	}
}
