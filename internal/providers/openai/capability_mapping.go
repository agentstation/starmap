package openai

import (
	stderrors "errors"
	"net/url"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

var errContradictoryCapabilityClaims = stderrors.New("contradictory capability claims")

var capabilityMappingTargets = map[catalogs.ModelFeature]struct{}{
	catalogs.ModelFeatureTools:      {},
	catalogs.ModelFeatureToolCalls:  {},
	catalogs.ModelFeatureToolChoice: {},
	catalogs.ModelFeatureReasoning:  {},
}

func validateCapabilityMappings(provider *catalogs.Provider) error {
	if provider == nil || provider.Catalog == nil {
		return nil
	}
	targetCombinations := make(map[catalogs.ModelFeature]catalogs.ProviderCapabilityCombination)
	for index, mapping := range provider.Catalog.Endpoint.CapabilityMappings {
		if !validBooleanCapabilitySource(mapping.From) {
			return &errors.ValidationError{
				Field: "capability_mappings.from", Value: mapping.From,
				Message: "unsupported boolean source predicate",
			}
		}
		if len(mapping.To) == 0 {
			return &errors.ValidationError{
				Field: "capability_mappings.to", Value: index,
				Message: "must contain at least one canonical capability",
			}
		}
		if !validCapabilityEvidence(mapping.Evidence) {
			return &errors.ValidationError{
				Field: "capability_mappings.evidence", Value: index,
				Message: "must be an absolute HTTPS provider-contract URL",
			}
		}
		combination := normalizedCapabilityCombination(mapping.Combine)
		if !validCapabilityCombination(combination) {
			return &errors.ValidationError{
				Field: "capability_mappings.combine", Value: mapping.Combine,
				Message: "must be conflict, first-known, any, or all",
			}
		}
		seen := make(map[catalogs.ModelFeature]struct{}, len(mapping.To))
		for _, target := range mapping.To {
			if _, valid := capabilityMappingTargets[target]; !valid {
				return &errors.ValidationError{
					Field: "capability_mappings.to", Value: target,
					Message: "unsupported canonical capability target",
				}
			}
			if _, duplicate := seen[target]; duplicate {
				return &errors.ValidationError{
					Field: "capability_mappings.to", Value: target,
					Message: "duplicate target in one mapping",
				}
			}
			seen[target] = struct{}{}
			if existing, found := targetCombinations[target]; found && existing != combination {
				return &errors.ValidationError{
					Field: "capability_mappings.combine", Value: target,
					Message: "all mappings for one target must use the same combination rule",
				}
			}
			targetCombinations[target] = combination
		}
	}
	return nil
}

func validBooleanCapabilitySource(source string) bool {
	switch source {
	case "supports_tools", "supports_reasoning":
		return true
	default:
		return false
	}
}

func validCapabilityEvidence(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func normalizedCapabilityCombination(
	combination catalogs.ProviderCapabilityCombination,
) catalogs.ProviderCapabilityCombination {
	if combination == "" {
		return catalogs.ProviderCapabilityConflict
	}
	return combination
}

func validCapabilityCombination(combination catalogs.ProviderCapabilityCombination) bool {
	switch combination {
	case catalogs.ProviderCapabilityConflict,
		catalogs.ProviderCapabilityFirstKnown,
		catalogs.ProviderCapabilityAny,
		catalogs.ProviderCapabilityAll:
		return true
	default:
		return false
	}
}

type capabilityEvidenceValue struct {
	value bool
	known bool
}

func (c *Client) applyCapabilityMappings(model *catalogs.Model, apiModel Model) error {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()
	if provider == nil || provider.Catalog == nil || len(provider.Catalog.Endpoint.CapabilityMappings) == 0 {
		return nil
	}

	targetMappings := make(map[catalogs.ModelFeature][]catalogs.CapabilityMapping)
	targetOrder := make([]catalogs.ModelFeature, 0)
	for _, mapping := range provider.Catalog.Endpoint.CapabilityMappings {
		for _, target := range mapping.To {
			if _, found := targetMappings[target]; !found {
				targetOrder = append(targetOrder, target)
			}
			targetMappings[target] = append(targetMappings[target], mapping)
		}
	}
	for _, target := range targetOrder {
		mappings := targetMappings[target]
		value, known, err := combineCapabilityMappings(mappings, apiModel)
		if err != nil {
			return &errors.ValidationError{
				Field: "capability_mappings", Value: model.ID,
				Message: "contradictory upstream claims for " + string(target),
			}
		}
		if known {
			ensureModelFeatures(model).SetSupport(target, value)
		}
	}
	return nil
}

func combineCapabilityMappings(
	mappings []catalogs.CapabilityMapping,
	apiModel Model,
) (bool, bool, error) {
	combination := normalizedCapabilityCombination(mappings[0].Combine)
	values := make([]capabilityEvidenceValue, 0, len(mappings))
	for _, mapping := range mappings {
		value := capabilitySourceValue(mapping.From, apiModel)
		if value == nil {
			values = append(values, capabilityEvidenceValue{})
			continue
		}
		values = append(values, capabilityEvidenceValue{value: *value, known: true})
	}

	switch combination {
	case catalogs.ProviderCapabilityFirstKnown:
		for _, value := range values {
			if value.known {
				return value.value, true, nil
			}
		}
		return false, false, nil
	case catalogs.ProviderCapabilityAny:
		allKnown := true
		for _, value := range values {
			if value.known && value.value {
				return true, true, nil
			}
			allKnown = allKnown && value.known
		}
		return false, allKnown, nil
	case catalogs.ProviderCapabilityAll:
		allKnown := true
		for _, value := range values {
			if value.known && !value.value {
				return false, true, nil
			}
			allKnown = allKnown && value.known
		}
		return true, allKnown, nil
	default:
		var selected capabilityEvidenceValue
		for _, value := range values {
			if !value.known {
				continue
			}
			if selected.known && selected.value != value.value {
				return false, false, errContradictoryCapabilityClaims
			}
			selected = value
		}
		return selected.value, selected.known, nil
	}
}

func capabilitySourceValue(source string, apiModel Model) *bool {
	switch source {
	case "supports_tools":
		return apiModel.SupportsTools
	case "supports_reasoning":
		return apiModel.SupportsReasoning
	default:
		return nil
	}
}
