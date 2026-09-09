package handlers

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestUpdateDetailPreservesSourceActivity(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failed], func(t *testing.T) {
			activity := []sources.SourceActivity{{Source: sources.ProvidersID, Supported: true, Enabled: true, Eligibility: sources.EligibilityIneligible, Attempted: true}, {Source: "private-source", Eligibility: "private-reason"}}
			attempts := []sources.ProviderAttempt{{ProviderID: "openai", Outcome: sources.ProviderOutcomeSkippedNotConfigured, Reason: sources.ProviderReasonCredentialUnavailable}, {ProviderID: "openai", Outcome: "private-outcome"}}
			h := &Handlers{app: &testApplication{SyncFunc: func(context.Context, ...pkgsync.Option) (*pkgsync.Result, error) {
				if failed {
					return nil, &sources.ActivityError{Activities: activity, ProviderAttempts: attempts, Err: context.Canceled}
				}
				return &pkgsync.Result{SourceActivities: activity, ProviderAttempts: attempts}, nil
			}}}
			detail, err := h.runCatalogUpdate(t.Context(), nil)
			if (err != nil) != failed {
				t.Fatalf("error=%v", err)
			}
			actual, ok := detail["source_activities"].([]sources.SourceActivity)
			if !ok || len(actual) != 1 || actual[0] != activity[0] {
				t.Fatalf("source activity=%v", detail)
			}
			providers, ok := detail["provider_attempts"].([]sources.ProviderAttempt)
			if !ok || len(providers) != 1 || providers[0] != attempts[0] {
				t.Fatalf("provider attempts=%v", detail)
			}
			activity[0].Source = sources.LocalCatalogID
			attempts[0].ProviderID = "changed"
			if actual[0].Source != sources.ProvidersID || providers[0].ProviderID != "openai" {
				t.Fatal("detail aliases acquisition report")
			}
		})
	}
}
