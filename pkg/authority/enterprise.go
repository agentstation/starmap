package authority

// EnterpriseAuthorityOwner is the primary owner of an enterprise catalog fact.
type EnterpriseAuthorityOwner string

const (
	// EnterpriseAuthorityModelAuthor owns canonical model-definition facts.
	EnterpriseAuthorityModelAuthor EnterpriseAuthorityOwner = "model_author"
	// EnterpriseAuthorityProvider owns inference-provider offering facts.
	EnterpriseAuthorityProvider EnterpriseAuthorityOwner = "inference_provider"
	// EnterpriseAuthorityCloud owns cloud geography and deployment facts.
	EnterpriseAuthorityCloud EnterpriseAuthorityOwner = "cloud_provider"
	// EnterpriseAuthorityCustomer owns credential-scoped customer facts.
	EnterpriseAuthorityCustomer EnterpriseAuthorityOwner = "customer_api"
	// EnterpriseAuthorityStarport owns routing policy rather than catalog facts.
	EnterpriseAuthorityStarport EnterpriseAuthorityOwner = "starport_policy"
)

// EnterpriseAttributeRule is one executable attribute-authority decision.
type EnterpriseAttributeRule struct {
	Path      string
	Primary   EnterpriseAuthorityOwner
	Fallbacks []string
	Merge     string
	Empty     string
	Rationale string
}

// EnterpriseAttributeMatrix returns caller-owned enterprise authority rules.
func EnterpriseAttributeMatrix() []EnterpriseAttributeRule {
	rules := []EnterpriseAttributeRule{
		{Path: "definition.name_family_lineage", Primary: EnterpriseAuthorityModelAuthor, Fallbacks: []string{"curated", "models.dev"}, Merge: "atomic", Empty: "retain_last_known_good", Rationale: "Canonical identity belongs to the model author."},
		{Path: "definition.context_modalities_capabilities", Primary: EnterpriseAuthorityModelAuthor, Fallbacks: []string{"provider_evidence", "models.dev"}, Merge: "field", Empty: "retain_last_known_good", Rationale: "Intrinsic behavior is provider independent."},
		{Path: "offering.identity_availability_lifecycle", Primary: EnterpriseAuthorityProvider, Fallbacks: []string{"last_known_good"}, Merge: "atomic", Empty: "partial_is_not_deletion", Rationale: "The serving provider owns its current offering."},
		{Path: "offering.invocation_deployment_capabilities", Primary: EnterpriseAuthorityProvider, Fallbacks: []string{"last_known_good", "models.dev"}, Merge: "field", Empty: "retain_last_known_good", Rationale: "Invocation and deployment behavior are provider facts."},
		{Path: "offering.pricing", Primary: EnterpriseAuthorityProvider, Fallbacks: []string{"last_known_good", "models.dev", "reviewed_curated"}, Merge: "atomic", Empty: "never_replace_with_stale_lower_authority", Rationale: "A provider price applies only to its offering."},
		{Path: "offering.region_residency_tier_profile", Primary: EnterpriseAuthorityCloud, Fallbacks: []string{"last_known_good"}, Merge: "atomic_per_region", Empty: "no_cross_region_inference", Rationale: "Clouds own regional availability and residency."},
		{Path: "customer.deployment_alias_quota_access", Primary: EnterpriseAuthorityCustomer, Fallbacks: nil, Merge: "replace_customer_scope", Empty: "private_only", Rationale: "Customer APIs are authoritative only for that customer."},
		{Path: "routing.preference", Primary: EnterpriseAuthorityStarport, Fallbacks: nil, Merge: "policy", Empty: "not_catalog_data", Rationale: "Routing preference is not source evidence."},
	}
	result := make([]EnterpriseAttributeRule, len(rules))
	for index, rule := range rules {
		result[index] = rule
		result[index].Fallbacks = append([]string(nil), rule.Fallbacks...)
	}
	return result
}
