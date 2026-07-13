package authority

import "testing"

func TestEnterpriseAttributeMatrixIsCompleteAndIsolated(t *testing.T) {
	rules := EnterpriseAttributeMatrix()
	want := map[string]EnterpriseAuthorityOwner{
		"definition.name_family_lineage":              EnterpriseAuthorityModelAuthor,
		"definition.context_modalities_capabilities":  EnterpriseAuthorityModelAuthor,
		"offering.identity_availability_lifecycle":    EnterpriseAuthorityProvider,
		"offering.invocation_deployment_capabilities": EnterpriseAuthorityProvider,
		"offering.pricing":                            EnterpriseAuthorityProvider,
		"offering.region_residency_tier_profile":      EnterpriseAuthorityCloud,
		"customer.deployment_alias_quota_access":      EnterpriseAuthorityCustomer,
		"routing.preference":                          EnterpriseAuthorityStarport,
	}
	for _, rule := range rules {
		owner, found := want[rule.Path]
		if !found {
			t.Fatalf("unexpected rule %q", rule.Path)
		}
		if owner != rule.Primary || rule.Merge == "" || rule.Empty == "" || rule.Rationale == "" {
			t.Fatalf("incomplete rule %#v", rule)
		}
		delete(want, rule.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing rules %#v", want)
	}

	rules[0].Fallbacks[0] = "mutated"
	if EnterpriseAttributeMatrix()[0].Fallbacks[0] == "mutated" {
		t.Fatal("matrix returned shared fallback state")
	}
}

func TestProviderPricingNeverFallsBackAheadOfLastKnownGood(t *testing.T) {
	for _, rule := range EnterpriseAttributeMatrix() {
		if rule.Path != "offering.pricing" {
			continue
		}
		if rule.Primary != EnterpriseAuthorityProvider || len(rule.Fallbacks) < 2 || rule.Fallbacks[0] != "last_known_good" || rule.Empty != "never_replace_with_stale_lower_authority" {
			t.Fatalf("pricing rule = %#v", rule)
		}
		return
	}
	t.Fatal("offering pricing rule missing")
}
