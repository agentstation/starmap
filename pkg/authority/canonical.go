package authority

import (
	"slices"

	"github.com/agentstation/starmap/pkg/sources"
)

// MergePolicy defines how accepted values compose after authority selection.
type MergePolicy string

const (
	// MergeIdentity requires one stable value and rejects disagreement.
	MergeIdentity MergePolicy = "identity"
	// MergeReplace selects one complete value without mixing subfields.
	MergeReplace MergePolicy = "replace"
	// MergeFillMissing accepts lower-authority values only for absent subfields.
	MergeFillMissing MergePolicy = "fill_missing"
	// MergeSetUnion combines unique members while preserving authority order.
	MergeSetUnion MergePolicy = "set_union"
	// MergeDeep combines named maps/records while resolving each leaf by authority.
	MergeDeep MergePolicy = "deep_merge"
)

// EmptyPolicy defines whether a Go zero/empty value carries source evidence.
type EmptyPolicy string

const (
	// EmptyReject makes an empty identity or required field invalid.
	EmptyReject EmptyPolicy = "reject"
	// EmptyAbsent treats nil/empty as no claim and permits fallback.
	EmptyAbsent EmptyPolicy = "absent"
	// EmptyAuthoritative preserves explicit zero and false values as evidence.
	EmptyAuthoritative EmptyPolicy = "authoritative"
)

// AttributePolicy is the complete merge contract for one canonical field family.
type AttributePolicy struct {
	Resource       sources.ResourceType
	Path           string
	AuthorityOrder []sources.ID
	Merge          MergePolicy
	Empty          EmptyPolicy
	Rationale      string
}

// CanonicalPolicies returns a caller-owned policy inventory for a canonical resource.
func CanonicalPolicies(resource sources.ResourceType) []AttributePolicy {
	var policies []AttributePolicy
	switch resource {
	case sources.ResourceTypeModelDefinition:
		policies = definitionPolicies()
	case sources.ResourceTypeProviderOffering:
		policies = offeringPolicies()
	default:
		return nil
	}
	result := make([]AttributePolicy, len(policies))
	for index, policy := range policies {
		result[index] = policy
		result[index].AuthorityOrder = slices.Clone(policy.AuthorityOrder)
	}
	return result
}

// FindCanonicalPolicy returns the policy whose pattern covers path.
func FindCanonicalPolicy(resource sources.ResourceType, path string) (AttributePolicy, bool) {
	for _, policy := range CanonicalPolicies(resource) {
		if MatchesPattern(path, policy.Path) {
			return policy, true
		}
	}
	return AttributePolicy{}, false
}

func definitionPolicies() []AttributePolicy {
	curated := []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID, sources.AmazonBedrockID, sources.MicrosoftFoundryID, sources.ProvidersID}
	observedCapability := []sources.ID{sources.ProvidersID, sources.AmazonBedrockID, sources.MicrosoftFoundryID, sources.ModelsDevHTTPID, sources.ModelsDevGitID, sources.LocalCatalogID}
	return []AttributePolicy{
		{sources.ResourceTypeModelDefinition, "ID", curated, MergeIdentity, EmptyReject, "Canonical definition identity is curated and provider-independent."},
		{sources.ResourceTypeModelDefinition, authorityPathName, curated, MergeReplace, EmptyReject, "A stable curated display name avoids provider branding drift."},
		{sources.ResourceTypeModelDefinition, "AuthorIDs", curated, MergeSetUnion, EmptyAbsent, "Curated authorship leads; discovered authors may add non-duplicate evidence."},
		{sources.ResourceTypeModelDefinition, authorityPathDescription, curated, MergeReplace, EmptyAbsent, "Human-reviewed copy leads community and provider descriptions."},
		{sources.ResourceTypeModelDefinition, "Metadata*", curated, MergeFillMissing, EmptyAbsent, "Release, knowledge, and discovery metadata are definition facts led by curated/community evidence."},
		{sources.ResourceTypeModelDefinition, "Lineage*", curated, MergeFillMissing, EmptyAbsent, "Family and derivation are canonical identity facts, not offering labels."},
		{sources.ResourceTypeModelDefinition, "Weights.Open", curated, MergeReplace, EmptyAuthoritative, "Open=false is an explicit provider-independent weight fact."},
		{sources.ResourceTypeModelDefinition, "Weights.Architecture*", curated, MergeFillMissing, EmptyAbsent, "Architecture is provider-independent and lower sources fill only absent details."},
		{sources.ResourceTypeModelDefinition, "Capabilities.Features*", observedCapability, MergeFillMissing, EmptyAuthoritative, "Explicit provider feature evidence leads; known false must not become absence."},
		{sources.ResourceTypeModelDefinition, "Capabilities*", observedCapability, MergeFillMissing, EmptyAbsent, "Optional intrinsic capability structures use provider evidence first and lower sources only for absence."},
		{sources.ResourceTypeModelDefinition, "CreatedAt", curated, MergeReplace, EmptyAbsent, "Creation time follows the highest-authority definition record."},
		{sources.ResourceTypeModelDefinition, "UpdatedAt", curated, MergeReplace, EmptyAbsent, "Update time follows the winning definition record rather than ingestion time."},
	}
}

func offeringPolicies() []AttributePolicy {
	providerFirst := []sources.ID{sources.ProvidersID, sources.AmazonBedrockID, sources.MicrosoftFoundryID, sources.OCIGenerativeAIID, sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID}
	curatedIdentity := []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID, sources.AmazonBedrockID, sources.MicrosoftFoundryID, sources.ProvidersID}
	return []AttributePolicy{
		{sources.ResourceTypeProviderOffering, "ProviderID", providerFirst, MergeIdentity, EmptyReject, "Offering identity is scoped to the provider that serves it."},
		{sources.ResourceTypeProviderOffering, "ProviderModelID", providerFirst, MergeIdentity, EmptyReject, "The provider model ID is the exact opaque inference identifier."},
		{sources.ResourceTypeProviderOffering, "DeploymentID", providerFirst, MergeIdentity, EmptyAbsent, "A contextual deployment ID distinguishes multiple deployments of one provider model."},
		{sources.ResourceTypeProviderOffering, "DefinitionID", curatedIdentity, MergeIdentity, EmptyReject, "Definition resolution is curated independently of provider naming."},
		{sources.ResourceTypeProviderOffering, authorityPathAliases, providerFirst, MergeSetUnion, EmptyAbsent, "Provider-observed deployment and routing aliases are additive contextual facts."},
		{sources.ResourceTypeProviderOffering, "Pricing*", providerFirst, MergeReplace, EmptyAbsent, "A semantically valid provider price is atomic and leads offering-specific fallbacks."},
		{sources.ResourceTypeProviderOffering, "Limits*", providerFirst, MergeFillMissing, EmptyAbsent, "Provider limits lead; community data fills only absent dimensions."},
		{sources.ResourceTypeProviderOffering, "Availability", providerFirst, MergeReplace, EmptyReject, "The live provider observation is authoritative for current service availability."},
		{sources.ResourceTypeProviderOffering, "Access*", providerFirst, MergeReplace, EmptyReject, "Provider invocation evidence leads and routability fails closed."},
		{sources.ResourceTypeProviderOffering, "Regions", providerFirst, MergeSetUnion, EmptyAbsent, "Provider regions lead; lower sources may add documented non-duplicate regions."},
		{sources.ResourceTypeProviderOffering, "Deployment*", providerFirst, MergeReplace, EmptyReject, "Deployment type and service tier are provider-specific facts."},
		{sources.ResourceTypeProviderOffering, "InferenceProfile*", providerFirst, MergeReplace, EmptyAbsent, "Cloud providers own cross-region profile identity and destinations."},
		{sources.ResourceTypeProviderOffering, "AggregatorUpstream*", providerFirst, MergeReplace, EmptyAbsent, "Aggregators retain underlying provider offering identity when known."},
		{sources.ResourceTypeProviderOffering, "Endpoint*", providerFirst, MergeFillMissing, EmptyAbsent, "Provider behavior leads while curated configuration may supply absent connection details."},
		{sources.ResourceTypeProviderOffering, "Lifecycle", providerFirst, MergeReplace, EmptyReject, "Provider lifecycle is offering-specific and current provider evidence leads."},
		{sources.ResourceTypeProviderOffering, "Modes*", providerFirst, MergeDeep, EmptyAbsent, "Named provider modes merge by name while price, limits, and request leaves retain provider authority."},
	}
}
