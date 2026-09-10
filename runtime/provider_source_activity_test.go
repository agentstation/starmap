package runtime

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderSourceActivityEligibility(t *testing.T) {
	missing := sources.ProviderAttempt{ProviderID: "openai", Outcome: sources.ProviderOutcomeSkippedNotConfigured, Reason: sources.ProviderReasonCredentialUnavailable}
	requested := sources.ProviderAttempt{ProviderID: "openai", Outcome: sources.ProviderOutcomeFailed, Reason: sources.ProviderReasonCredentialRejected, Requested: true}
	for _, test := range []struct {
		name        string
		result      AcquisitionResult
		attempted   bool
		err         error
		eligibility sources.Eligibility
	}{
		{"empty-selection", AcquisitionResult{}, false, nil, sources.EligibilityIneligible},
		{"canceled-preflight", AcquisitionResult{}, false, context.Canceled, sources.EligibilityUnknown},
		{"missing", AcquisitionResult{Eligible: 1, Attempts: []sources.ProviderAttempt{missing}}, true, nil, sources.EligibilityIneligible},
		{"unreported-target", AcquisitionResult{Eligible: 2, Attempts: []sources.ProviderAttempt{missing}}, true, nil, sources.EligibilityUnknown},
		{"request-rejected", AcquisitionResult{Eligible: 1, Attempts: []sources.ProviderAttempt{requested}}, true, nil, sources.EligibilityEligible},
		{"mixed", AcquisitionResult{Eligible: 2, Attempts: []sources.ProviderAttempt{missing, requested}}, true, nil, sources.EligibilityEligible},
		{"opaque-layer", AcquisitionResult{Layers: []ProviderLayer{{}}}, true, nil, sources.EligibilityUnknown},
		{"invalid-attempt", AcquisitionResult{Attempts: []sources.ProviderAttempt{{ProviderID: "openai", Outcome: "private"}}}, true, nil, sources.EligibilityUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := providerSourceActivity(test.result, test.attempted, test.err)
			if !row.Valid() || row.Attempted != test.attempted || row.Eligibility != test.eligibility {
				t.Fatalf("activity=%+v, want eligibility %s", row, test.eligibility)
			}
		})
	}
}

func TestProviderActivityCountsOnlyCollectorCalls(t *testing.T) {
	for _, test := range []struct {
		name      string
		policy    *providerBindingPolicy
		attempted bool
		fails     bool
	}{
		{"unscoped-call", nil, true, false},
		{"empty-bindings", &providerBindingPolicy{}, false, false},
		{"unsupported-binding-role", &providerBindingPolicy{bindings: map[string]sources.ProviderAcquisitionBinding{"one": {ID: "one", ProviderID: "openai"}}}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			observer := &stubAcquirer{}
			connected := &Runtime{config: options{acquirer: observer, providerBindings: test.policy}}
			_, attempted, err := connected.acquireSelectedProviders(t.Context(), AcquisitionRequest{})
			if attempted != test.attempted || (err != nil) != test.fails {
				t.Fatalf("attempted=%v, error=%v", attempted, err)
			}
			if (observer.callCount() != 0) != attempted {
				t.Fatal("attempt report differs from collector calls")
			}
		})
	}
}
