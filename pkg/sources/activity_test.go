package sources

import "testing"

func TestSourceActivityValidation(t *testing.T) {
	for _, test := range []struct {
		name     string
		activity SourceActivity
		valid    bool
	}{
		{"unknown", SourceActivity{Source: ProvidersID, Eligibility: EligibilityUnknown}, true},
		{"eligible", SourceActivity{Source: LocalCatalogID, Supported: true, Enabled: true, Eligibility: EligibilityEligible, Attempted: true}, true},
		{"credential-refusal", SourceActivity{Source: ProvidersID, Supported: true, Enabled: true, Eligibility: EligibilityIneligible, Attempted: true}, true},
		{"private-source", SourceActivity{Source: "private", Eligibility: EligibilityUnknown}, false},
		{"private-state", SourceActivity{Source: ProvidersID, Eligibility: "private"}, false},
		{"disabled-attempt", SourceActivity{Source: ProvidersID, Supported: true, Eligibility: EligibilityUnknown, Attempted: true}, false},
		{"unsupported-attempt", SourceActivity{Source: ProvidersID, Enabled: true, Eligibility: EligibilityUnknown, Attempted: true}, false},
		{"disabled-eligible", SourceActivity{Source: ProvidersID, Supported: true, Eligibility: EligibilityEligible}, false},
		{"baseline", SourceActivity{Source: EmbeddedCatalogID, Eligibility: EligibilityUnknown}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.activity.Valid(); got != test.valid {
				t.Fatalf("valid=%v, want %v", got, test.valid)
			}
		})
	}
}
