package reconciler

import (
	"fmt"
	"time"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func (merger *merger) applyProviderPolicy(
	target *catalogs.Provider,
	policy authority.Policy,
	providers map[sources.ID]*catalogs.Provider,
	history *map[string]provenance.Field,
) {
	switch policy.Path {
	case "Extensions":
		merger.mergeProviderExtensions(target, policy, providers, history)
	case "Aliases":
		merger.mergeProviderAliases(target, policy, providers, history)
	case "PrivacyPolicy":
		merger.mergeProviderPrivacy(target, policy, providers, history)
	case "RetentionPolicy":
		merger.mergeProviderRetention(target, policy, providers, history)
	case "GovernancePolicy":
		merger.mergeProviderGovernance(target, policy, providers, history)
	default:
		value, source := merger.providerField(policy, providers)
		if value == nil {
			return
		}
		merger.setProviderFieldValue(target, policy.Path, value)
		merger.recordProviderHistory(history, policy, source, value, "selected complete value by authority order")
	}
}

func (merger *merger) mergeProviderAliases(
	target *catalogs.Provider,
	policy authority.Policy,
	providers map[sources.ID]*catalogs.Provider,
	history *map[string]provenance.Field,
) {
	seen := make(map[catalogs.ProviderID]struct{})
	aliases := make([]catalogs.ProviderID, 0)
	var winner sources.ID
	for _, source := range policy.SourceOrder {
		provider := providers[source]
		if provider == nil || len(provider.Aliases) == 0 {
			continue
		}
		if winner == "" {
			winner = source
		}
		for _, alias := range provider.Aliases {
			if _, exists := seen[alias]; exists {
				continue
			}
			seen[alias] = struct{}{}
			aliases = append(aliases, alias)
		}
	}
	if len(aliases) == 0 {
		return
	}
	target.Aliases = aliases
	merger.recordProviderHistory(history, policy, winner, aliases, "merged non-duplicate aliases by authority order")
}

func (merger *merger) mergeProviderPrivacy(
	target *catalogs.Provider,
	policy authority.Policy,
	providers map[sources.ID]*catalogs.Provider,
	history *map[string]provenance.Field,
) {
	var winner sources.ID
	var merged *catalogs.ProviderPrivacyPolicy
	for _, source := range policy.SourceOrder {
		provider := providers[source]
		if provider == nil || provider.PrivacyPolicy == nil {
			continue
		}
		if winner == "" {
			winner = source
		}
		merged = fillProviderPrivacy(merged, provider.PrivacyPolicy)
	}
	if merged == nil {
		return
	}
	target.PrivacyPolicy = merged
	merger.recordProviderHistory(history, policy, winner, merged, "filled missing privacy fields by authority order")
}

func fillProviderPrivacy(
	target *catalogs.ProviderPrivacyPolicy,
	fallback *catalogs.ProviderPrivacyPolicy,
) *catalogs.ProviderPrivacyPolicy {
	if fallback == nil {
		return target
	}
	if target == nil {
		copied := *fallback
		copied.PrivacyPolicyURL = copyValuePtr(fallback.PrivacyPolicyURL)
		copied.TermsOfServiceURL = copyValuePtr(fallback.TermsOfServiceURL)
		copied.RetainsData = copyValuePtr(fallback.RetainsData)
		copied.TrainsOnData = copyValuePtr(fallback.TrainsOnData)
		return &copied
	}
	if target.PrivacyPolicyURL == nil {
		target.PrivacyPolicyURL = copyValuePtr(fallback.PrivacyPolicyURL)
	}
	if target.TermsOfServiceURL == nil {
		target.TermsOfServiceURL = copyValuePtr(fallback.TermsOfServiceURL)
	}
	if target.RetainsData == nil {
		target.RetainsData = copyValuePtr(fallback.RetainsData)
	}
	if target.TrainsOnData == nil {
		target.TrainsOnData = copyValuePtr(fallback.TrainsOnData)
	}
	return target
}

func (merger *merger) mergeProviderRetention(
	target *catalogs.Provider,
	policy authority.Policy,
	providers map[sources.ID]*catalogs.Provider,
	history *map[string]provenance.Field,
) {
	var winner sources.ID
	var merged *catalogs.ProviderRetentionPolicy
	for _, source := range policy.SourceOrder {
		provider := providers[source]
		if provider == nil || provider.RetentionPolicy == nil {
			continue
		}
		if winner == "" {
			winner = source
		}
		merged = fillProviderRetention(merged, provider.RetentionPolicy)
	}
	if merged == nil {
		return
	}
	target.RetentionPolicy = merged
	merger.recordProviderHistory(history, policy, winner, merged, "filled missing retention fields by authority order")
}

func fillProviderRetention(
	target *catalogs.ProviderRetentionPolicy,
	fallback *catalogs.ProviderRetentionPolicy,
) *catalogs.ProviderRetentionPolicy {
	if fallback == nil {
		return target
	}
	if target == nil {
		copied := *fallback
		copied.Duration = copyValuePtr(fallback.Duration)
		copied.Details = copyValuePtr(fallback.Details)
		return &copied
	}
	if target.Type == "" {
		target.Type = fallback.Type
	}
	if target.Duration == nil {
		target.Duration = copyValuePtr(fallback.Duration)
	}
	if target.Details == nil {
		target.Details = copyValuePtr(fallback.Details)
	}
	return target
}

func (merger *merger) mergeProviderGovernance(
	target *catalogs.Provider,
	policy authority.Policy,
	providers map[sources.ID]*catalogs.Provider,
	history *map[string]provenance.Field,
) {
	var winner sources.ID
	var merged *catalogs.ProviderGovernancePolicy
	for _, source := range policy.SourceOrder {
		provider := providers[source]
		if provider == nil || provider.GovernancePolicy == nil {
			continue
		}
		if winner == "" {
			winner = source
		}
		merged = fillProviderGovernance(merged, provider.GovernancePolicy)
	}
	if merged == nil {
		return
	}
	target.GovernancePolicy = merged
	merger.recordProviderHistory(history, policy, winner, merged, "filled missing governance fields by authority order")
}

func fillProviderGovernance(
	target *catalogs.ProviderGovernancePolicy,
	fallback *catalogs.ProviderGovernancePolicy,
) *catalogs.ProviderGovernancePolicy {
	if fallback == nil {
		return target
	}
	if target == nil {
		copied := *fallback
		copied.ModerationRequired = copyValuePtr(fallback.ModerationRequired)
		copied.Moderated = copyValuePtr(fallback.Moderated)
		copied.Moderator = copyValuePtr(fallback.Moderator)
		return &copied
	}
	if target.ModerationRequired == nil {
		target.ModerationRequired = copyValuePtr(fallback.ModerationRequired)
	}
	if target.Moderated == nil {
		target.Moderated = copyValuePtr(fallback.Moderated)
	}
	if target.Moderator == nil {
		target.Moderator = copyValuePtr(fallback.Moderator)
	}
	return target
}

func (merger *merger) recordProviderHistory(
	history *map[string]provenance.Field,
	policy authority.Policy,
	source sources.ID,
	value any,
	reason string,
) {
	if history == nil {
		return
	}
	path := policy.Evidence()
	(*history)[path] = provenance.Field{
		Current: provenance.Provenance{
			Source:     source,
			Field:      path,
			Value:      value,
			Timestamp:  time.Now(),
			Authority:  policy.Authority(source),
			Confidence: merger.calculateConfidence(value),
			Reason:     fmt.Sprintf("%s: %s", policy.Path, reason),
		},
	}
}
