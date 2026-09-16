package publication

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func publicPublicationProfile(t *testing.T) Profile {
	t.Helper()
	raw, err := os.ReadFile("../../../.github/catalog-publication.yaml")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := ParseProfile(raw)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestPublicPublicationProfileUsesDeclaredPublicCredentials(t *testing.T) {
	profile := publicPublicationProfile(t)
	baseline, err := bootstrap.Generation()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.DecodeCatalogGeneration(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewProducer(profile, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, policy := range profile.Scopes {
		if policy.Scope.Source != sources.ProvidersID {
			if policy.Scope.Source != sources.ModelsDevHTTPID {
				t.Fatal("public profile includes an undeclared source transport")
			}
			continue
		}
		binding := policy.Scope.Binding
		if binding == nil || !binding.Public || binding.AccountID != "" || binding.ProjectID != "" || binding.MembershipAuthority != sources.ProviderMembershipScope {
			t.Fatal("public profile grants account or provider-wide authority")
		}
		provider, err := catalog.Provider(binding.ProviderID)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(provider.Credentials.CatalogAcquisition.Alternatives, binding.CredentialProfileID) {
			t.Fatalf("provider %s does not declare the selected acquisition credential profile", binding.ProviderID)
		}
	}
}

func TestPublicPublicationProfileSourceOutages(t *testing.T) {
	profile := publicPublicationProfile(t)
	start := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name     string
		age      time.Duration
		fresh    bool
		retained bool
		allowed  bool
	}{
		{name: "first-boot-without-required-source"},
		{name: "fresh-metadata-without-provider-keys", fresh: true, allowed: true},
		{name: "retained-at-age-boundary", retained: true, age: 24 * time.Hour, allowed: true},
		{name: "retained-past-age-boundary", retained: true, age: 24*time.Hour + time.Nanosecond, allowed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			run := Run{StartedAt: start, CompletedAt: start}
			for _, policy := range profile.Scopes {
				attempt := Attempt{Scope: policy.Scope, Outcome: MissingCredentials}
				if policy.Scope.Source == sources.ModelsDevHTTPID {
					attempt.Outcome = Failed
					if test.fresh {
						observation := admissionObservation(t, policy.Scope, start)
						attempt.Outcome, attempt.Observation = Succeeded, &observation
					}
					if test.retained {
						run.Retained = append(run.Retained, admissionObservation(t, policy.Scope, start.Add(-test.age)))
					}
				}
				run.Attempts = append(run.Attempts, attempt)
			}
			decision, err := Admit(profile, run)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Allowed != test.allowed || decision.FreshAcquisition != test.fresh {
				t.Fatalf("allowed=%t fresh=%t", decision.Allowed, decision.FreshAcquisition)
			}
			if len(decision.Removals) != 0 {
				t.Fatal("source outage removed catalog entries")
			}
			for _, result := range decision.Scopes {
				if result.Scope.Source == sources.ProvidersID && (result.Attempt != MissingCredentials || result.EvidenceKind != NoEvidence) {
					t.Fatal("provider outage invented source evidence")
				}
			}
		})
	}
}
