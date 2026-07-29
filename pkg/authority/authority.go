// Package authority defines the executable field policy used by reconciliation.
package authority

import (
	"path/filepath"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogmeta"
)

// MergePolicy defines how accepted values compose after source selection.
type MergePolicy string

const (
	// MergeReplace selects one complete value without mixing subfields.
	MergeReplace MergePolicy = "replace"
	// MergeFillMissing accepts lower-authority values only for absent subfields.
	MergeFillMissing MergePolicy = "fill_missing"
	// MergeSetUnion combines unique members while preserving authority order.
	MergeSetUnion MergePolicy = "set_union"
	// MergeDeep combines a structured value according to its field semantics.
	MergeDeep MergePolicy = "deep_merge"
)

// EmptyPolicy defines whether a Go zero value carries source evidence.
type EmptyPolicy string

const (
	// EmptyAbsent treats nil and empty values as no claim and permits fallback.
	EmptyAbsent EmptyPolicy = "absent"
	// EmptyAuthoritative preserves explicit zero and false values as evidence.
	EmptyAuthoritative EmptyPolicy = "authoritative"
)

// Policy is the complete executable contract for one catalog field family.
//
// Path is both the reflected catalog path and the authority lookup pattern.
// EvidencePath is used only when persisted provenance has a stable external
// spelling that differs from Path.
type Policy struct {
	Resource     catalogmeta.ResourceType
	Path         string
	EvidencePath string
	SourceOrder  []catalogmeta.SourceID
	Merge        MergePolicy
	Empty        EmptyPolicy
	Rationale    string
}

// Evidence returns the stable provenance path for the policy.
func (p Policy) Evidence() string {
	if p.EvidencePath != "" {
		return p.EvidencePath
	}
	return p.Path
}

// Authority returns a normalized score for source within the policy order.
func (p Policy) Authority(source catalogmeta.SourceID) float64 {
	for index, candidate := range p.SourceOrder {
		if candidate == source {
			return float64(len(p.SourceOrder)-index) / float64(len(p.SourceOrder))
		}
	}
	return 0
}

// Reader is the narrow policy input consumed by reconciliation algorithms.
type Reader interface {
	Find(resource catalogmeta.ResourceType, path string) (Policy, bool)
	Policies(resource catalogmeta.ResourceType) []Policy
}

// Table is Starmap's immutable default authority policy.
type Table struct {
	byResource map[catalogmeta.ResourceType][]Policy
}

// New returns the concrete immutable default authority table.
func New() *Table {
	return &Table{byResource: indexPolicies(defaultPolicies())}
}

// Find returns the most specific policy covering path.
func (t *Table) Find(resource catalogmeta.ResourceType, path string) (Policy, bool) {
	if t == nil {
		return Policy{}, false
	}
	var (
		best      Policy
		bestWidth int
		found     bool
	)
	for _, policy := range t.byResource[resource] {
		if !MatchesPattern(path, policy.Path) {
			continue
		}
		if width := len(policy.Path); !found || width > bestWidth {
			best = clonePolicy(policy)
			bestWidth = width
			found = true
		}
	}
	return best, found
}

// Policies returns caller-owned policies for resource in execution order.
func (t *Table) Policies(resource catalogmeta.ResourceType) []Policy {
	if t == nil {
		return nil
	}
	policies := t.byResource[resource]
	result := make([]Policy, len(policies))
	for index, policy := range policies {
		result[index] = clonePolicy(policy)
	}
	return result
}

func indexPolicies(policies []Policy) map[catalogmeta.ResourceType][]Policy {
	indexed := make(map[catalogmeta.ResourceType][]Policy)
	for _, policy := range policies {
		indexed[policy.Resource] = append(indexed[policy.Resource], clonePolicy(policy))
	}
	return indexed
}

func clonePolicy(policy Policy) Policy {
	policy.SourceOrder = slices.Clone(policy.SourceOrder)
	return policy
}

// MatchesPattern reports whether a field path matches a policy pattern.
func MatchesPattern(fieldPath, pattern string) bool {
	if fieldPath == pattern {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(fieldPath) >= len(prefix) && fieldPath[:len(prefix)] == prefix
	}
	matched, err := filepath.Match(pattern, fieldPath)
	return err == nil && matched
}

func defaultPolicies() []Policy {
	providerFirst := []catalogmeta.SourceID{
		catalogmeta.ProvidersID,
		catalogmeta.ModelsDevHTTPID,
		catalogmeta.ModelsDevGitID,
		catalogmeta.LocalCatalogID,
		catalogmeta.EmbeddedCatalogID,
	}
	modelsDevFirst := []catalogmeta.SourceID{
		catalogmeta.ModelsDevHTTPID,
		catalogmeta.ModelsDevGitID,
		catalogmeta.ProvidersID,
		catalogmeta.LocalCatalogID,
		catalogmeta.EmbeddedCatalogID,
	}
	localFirst := []catalogmeta.SourceID{
		catalogmeta.LocalCatalogID,
		catalogmeta.ProvidersID,
		catalogmeta.ModelsDevHTTPID,
		catalogmeta.ModelsDevGitID,
		catalogmeta.EmbeddedCatalogID,
	}
	localThenModelsDev := []catalogmeta.SourceID{
		catalogmeta.LocalCatalogID,
		catalogmeta.ModelsDevHTTPID,
		catalogmeta.ModelsDevGitID,
		catalogmeta.ProvidersID,
		catalogmeta.EmbeddedCatalogID,
	}

	return []Policy{
		// Provider-scoped model facts.
		policy(catalogmeta.ResourceTypeModel, "Name", "", providerFirst, MergeReplace, EmptyAbsent, "The provider observation supplies the current provider-facing model name."),
		policy(catalogmeta.ResourceTypeModel, "Description", "", modelsDevFirst, MergeReplace, EmptyAuthoritative, "Current upstream descriptions lead; an explicitly present empty human value is distinct from an omitted fallback."),
		policy(catalogmeta.ResourceTypeModel, "Status", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed lifecycle data leads a manual fallback."),
		policy(catalogmeta.ResourceTypeModel, "Authors", "", modelsDevFirst, MergeSetUnion, EmptyAbsent, "Observed authorship leads and non-duplicate lower-authority authors may fill gaps."),
		policy(catalogmeta.ResourceTypeModel, "Lineage.Family", "lineage.family", modelsDevFirst, MergeReplace, EmptyAbsent, "Community model metadata leads provider and local fallback for canonical family."),
		policy(catalogmeta.ResourceTypeModel, "Lineage.Root", "lineage.root", providerFirst, MergeReplace, EmptyAbsent, "Provider lineage identifiers lead upstream and local fallback."),
		policy(catalogmeta.ResourceTypeModel, "Lineage.Parent", "lineage.parent", providerFirst, MergeReplace, EmptyAbsent, "Provider lineage identifiers lead upstream and local fallback."),
		policy(catalogmeta.ResourceTypeModel, "Limits", "limits", providerFirst, MergeFillMissing, EmptyAbsent, "Current provider limits lead; upstream and human data fill dimensions the provider omits."),
		policy(catalogmeta.ResourceTypeModel, "Metadata", "metadata", modelsDevFirst, MergeFillMissing, EmptyAbsent, "Definition metadata comes from upstream observation with lower-authority gap filling."),
		policy(catalogmeta.ResourceTypeModel, "Features", "", providerFirst, MergeDeep, EmptyAuthoritative, "A present provider capability record is authoritative; modalities accumulate documented support."),
		policy(catalogmeta.ResourceTypeModel, "Attachments", "", providerFirst, MergeReplace, EmptyAbsent, "Provider capability evidence leads upstream and human fallback."),
		policy(catalogmeta.ResourceTypeModel, "Generation", "", providerFirst, MergeReplace, EmptyAbsent, "Provider generation controls lead upstream and human fallback."),
		policy(catalogmeta.ResourceTypeModel, "Reasoning", "", providerFirst, MergeReplace, EmptyAbsent, "Provider reasoning controls lead upstream and human fallback."),
		policy(catalogmeta.ResourceTypeModel, "ReasoningTokens", "", providerFirst, MergeReplace, EmptyAbsent, "Provider reasoning-token controls lead upstream and human fallback."),
		policy(catalogmeta.ResourceTypeModel, "Verbosity", "", providerFirst, MergeReplace, EmptyAbsent, "Provider verbosity controls lead upstream and human fallback."),
		policy(catalogmeta.ResourceTypeModel, "Tools", "", providerFirst, MergeReplace, EmptyAbsent, "Provider tool controls lead upstream and human fallback."),
		policy(catalogmeta.ResourceTypeModel, "Delivery", "", providerFirst, MergeReplace, EmptyAbsent, "Provider delivery controls lead upstream and human fallback."),
		policy(catalogmeta.ResourceTypeModel, "Modes", "modes", providerFirst, MergeDeep, EmptyAbsent, "Provider service modes lead upstream and human fallback."),
		policy(catalogmeta.ResourceTypeModel, "Pricing", "pricing", providerFirst, MergeReplace, EmptyAbsent, "A semantically valid provider price wins atomically for its offering."),
		policy(catalogmeta.ResourceTypeModel, "Extensions", "extensions", localThenModelsDev, MergeDeep, EmptyAbsent, "Namespaced source extensions merge fieldwise without replacing canonical facts."),
		policy(catalogmeta.ResourceTypeModel, "CreatedAt", "", providerFirst, MergeReplace, EmptyAbsent, "Creation time follows the highest-authority observation that supplies it."),
		policy(catalogmeta.ResourceTypeModel, "UpdatedAt", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Update time follows the highest-authority observation that supplies it."),

		// Provider discovery facts and operator configuration.
		policy(catalogmeta.ResourceTypeProvider, "Name", "", providerFirst, MergeReplace, EmptyAbsent, "Observed provider identity leads upstream and human fallback."),
		policy(catalogmeta.ResourceTypeProvider, "Headquarters", "", providerFirst, MergeReplace, EmptyAbsent, "Observed organization metadata leads upstream and human fallback."),
		policy(catalogmeta.ResourceTypeProvider, "IconURL", "", providerFirst, MergeReplace, EmptyAbsent, "Observed provider branding leads upstream and human fallback."),
		policy(catalogmeta.ResourceTypeProvider, "StatusPageURL", "", providerFirst, MergeReplace, EmptyAbsent, "Observed provider status metadata leads upstream and human fallback."),
		policy(catalogmeta.ResourceTypeProvider, "Aliases", "", localThenModelsDev, MergeSetUnion, EmptyAbsent, "Human aliases lead and discovered aliases may add non-duplicate identifiers."),
		policy(catalogmeta.ResourceTypeProvider, "APIKey", "", localFirst, MergeReplace, EmptyAbsent, "Credential parameter configuration is operator-owned."),
		policy(catalogmeta.ResourceTypeProvider, "EnvVars", "", localFirst, MergeReplace, EmptyAbsent, "Environment configuration is operator-owned."),
		policy(catalogmeta.ResourceTypeProvider, "Catalog", "", localFirst, MergeReplace, EmptyAbsent, "Acquisition endpoint configuration is operator-owned."),
		policy(catalogmeta.ResourceTypeProvider, "ChatCompletions", "", localFirst, MergeReplace, EmptyAbsent, "Inference endpoint configuration is operator-owned."),
		policy(catalogmeta.ResourceTypeProvider, "PrivacyPolicy", "", modelsDevFirst, MergeFillMissing, EmptyAbsent, "Observed policy data leads a human fallback."),
		policy(catalogmeta.ResourceTypeProvider, "RetentionPolicy", "", modelsDevFirst, MergeFillMissing, EmptyAbsent, "Observed retention data leads a human fallback."),
		policy(catalogmeta.ResourceTypeProvider, "GovernancePolicy", "", modelsDevFirst, MergeFillMissing, EmptyAbsent, "Observed governance data leads a human fallback."),
		policy(catalogmeta.ResourceTypeProvider, "Models", "", providerFirst, MergeReplace, EmptyAbsent, "The live provider observation supplies its current model set."),
		policy(catalogmeta.ResourceTypeProvider, "Extensions", "extensions", localThenModelsDev, MergeDeep, EmptyAbsent, "Namespaced source extensions merge fieldwise without replacing canonical facts."),

		// Author records are currently sourced as catalog metadata, but the same
		// table remains the only policy if reconciliation requests a winner.
		policy(catalogmeta.ResourceTypeAuthor, "Name", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed author identity leads a human fallback."),
		policy(catalogmeta.ResourceTypeAuthor, "Aliases", "", localThenModelsDev, MergeSetUnion, EmptyAbsent, "Human aliases lead discovered non-duplicate aliases."),
		policy(catalogmeta.ResourceTypeAuthor, "Description", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed author metadata leads a human fallback."),
		policy(catalogmeta.ResourceTypeAuthor, "Headquarters", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed author metadata leads a human fallback."),
		policy(catalogmeta.ResourceTypeAuthor, "IconURL", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed author metadata leads a human fallback."),
		policy(catalogmeta.ResourceTypeAuthor, "Website", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed author metadata leads a human fallback."),
		policy(catalogmeta.ResourceTypeAuthor, "HuggingFace", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed author metadata leads a human fallback."),
		policy(catalogmeta.ResourceTypeAuthor, "GitHub", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed author metadata leads a human fallback."),
		policy(catalogmeta.ResourceTypeAuthor, "Twitter", "", modelsDevFirst, MergeReplace, EmptyAbsent, "Observed author metadata leads a human fallback."),
		policy(catalogmeta.ResourceTypeAuthor, "Catalog", "", localFirst, MergeReplace, EmptyAbsent, "Author attribution configuration is operator-owned."),
	}
}

func policy(
	resource catalogmeta.ResourceType,
	path string,
	evidencePath string,
	order []catalogmeta.SourceID,
	merge MergePolicy,
	empty EmptyPolicy,
	rationale string,
) Policy {
	return Policy{
		Resource:     resource,
		Path:         path,
		EvidencePath: evidencePath,
		SourceOrder:  slices.Clone(order),
		Merge:        merge,
		Empty:        empty,
		Rationale:    rationale,
	}
}
