package sources

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestProviderAttemptBindingIdentityValidation(t *testing.T) {
	for _, test := range []struct {
		name, id, revision string
		valid              bool
	}{
		{"legacy", "", "", true}, {"bound", "account-catalog", "1", true},
		{"missing revision", "account-catalog", "", false}, {"missing identity", "", "1", false},
		{"control", "account\nsecret", "1", false}, {"whitespace", "account-catalog", " 1", false},
		{"oversize", strings.Repeat("x", MaxProviderBindingFieldBytes+1), "1", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			attempt := ProviderAttempt{ProviderID: catalogs.ProviderID("provider"), BindingID: test.id, BindingRevision: test.revision, Outcome: ProviderOutcomeSucceeded}
			err := attempt.Validate()
			if (err == nil) != test.valid {
				t.Fatalf("validation error: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "account\nsecret") {
				t.Fatal("validation exposed a rejected value")
			}
		})
	}
}

func TestLegacyProviderAttemptOmitsBindingFields(t *testing.T) {
	raw, err := json.Marshal(ProviderAttempt{ProviderID: "provider", Outcome: ProviderOutcomeSucceeded})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"BindingID", "BindingRevision"} {
		if _, present := decoded[key]; present {
			t.Fatal("legacy attempt gained an empty binding field")
		}
	}
}
