package catalogs

import (
	"fmt"
	"math"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	providerTokenScale           = 1_000_000
	providerTargetTokenPrice     = "token_price"
	providerTargetOperationPrice = "operation_price"
)

// ProviderNormalizationUnit is an allow-listed source pricing unit.
type ProviderNormalizationUnit string

const (
	// ProviderNormalizationUnitPerMillionTokens is currency per one million tokens.
	ProviderNormalizationUnitPerMillionTokens ProviderNormalizationUnit = "per_million_tokens"
	// ProviderNormalizationUnitPerToken is currency per individual token.
	ProviderNormalizationUnitPerToken ProviderNormalizationUnit = "per_token"
	// ProviderNormalizationUnitCentsPer100MillionTokens is cents per 100 million tokens.
	ProviderNormalizationUnitCentsPer100MillionTokens ProviderNormalizationUnit = "cents_per_100_million_tokens" //nolint:gosec // Public pricing unit label, not a credential.
	// ProviderNormalizationUnitMilliCurrencyPerMillionTokens is thousandths of the configured currency per one million tokens.
	ProviderNormalizationUnitMilliCurrencyPerMillionTokens ProviderNormalizationUnit = "milli_currency_per_million_tokens"
	// ProviderNormalizationUnitPerOperation is currency per operation.
	ProviderNormalizationUnitPerOperation ProviderNormalizationUnit = "per_operation"
)

// ProviderPricingTier identifies the canonical tier receiving one mapped price.
type ProviderPricingTier struct {
	Name string               `yaml:"name,omitempty" json:"name,omitempty"`
	Type ModelPricingTierType `yaml:"type" json:"type"`
	Size int64                `yaml:"size" json:"size"`
}

// NormalizeProviderTokenPrice converts one finite, non-negative provider value
// into the canonical per-token and per-million representation.
func NormalizeProviderTokenPrice(value float64, unit ProviderNormalizationUnit) (ModelTokenCost, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ModelTokenCost{}, providerNormalizationError("value", value, "must be finite")
	}
	if value < 0 {
		return ModelTokenCost{}, providerNormalizationError("value", value, "must not be negative")
	}

	var perToken, perMillion float64
	switch unit {
	case ProviderNormalizationUnitPerMillionTokens:
		perMillion = value
		perToken = value / providerTokenScale
	case ProviderNormalizationUnitPerToken:
		perToken = value
		perMillion = value * providerTokenScale
	case ProviderNormalizationUnitCentsPer100MillionTokens:
		perMillion = value / 10_000
		perToken = perMillion / providerTokenScale
	case ProviderNormalizationUnitMilliCurrencyPerMillionTokens:
		perMillion = value / 1_000
		perToken = perMillion / providerTokenScale
	default:
		return ModelTokenCost{}, providerNormalizationError("unit", unit, "is not a supported token pricing unit")
	}
	if math.IsNaN(perToken) || math.IsInf(perToken, 0) || math.IsNaN(perMillion) || math.IsInf(perMillion, 0) {
		return ModelTokenCost{}, providerNormalizationError("value", value, "overflows the canonical pricing representation")
	}
	return ModelTokenCost{PerToken: perToken, Per1M: perMillion}, nil
}

// ScaleProviderTokenPrice applies a finite, non-negative mode multiplier while
// preserving the canonical per-token/per-million equivalence.
func ScaleProviderTokenPrice(cost ModelTokenCost, scale float64) (ModelTokenCost, error) {
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale < 0 {
		return ModelTokenCost{}, providerNormalizationError("scale", scale, "must be finite and non-negative")
	}
	return NormalizeProviderTokenPrice(cost.Per1M*scale, ProviderNormalizationUnitPerMillionTokens)
}

// ValidateProviderFieldMappings validates bounded transformation configuration
// independently of any provider response or transport.
func ValidateProviderFieldMappings(mappings []FieldMapping) error {
	seenTargets := make(map[string]struct{}, len(mappings))
	for index, mapping := range mappings {
		field := fmt.Sprintf("provider.catalog.endpoint.field_mappings[%d]", index)
		if strings.TrimSpace(mapping.From) == "" {
			return providerNormalizationError(field+".from", mapping.From, "is required")
		}
		targetKind := providerMappingTargetKind(mapping.To)
		if targetKind == "" {
			return providerNormalizationError(field+".to", mapping.To, "is not an allow-listed canonical target")
		}
		if mapping.Mode != "" && !safeProviderPathSegment(mapping.Mode) {
			return providerNormalizationError(field+".mode", mapping.Mode, "must be a safe mode name")
		}
		if mapping.Tier != nil {
			if targetKind != providerTargetTokenPrice && targetKind != providerTargetOperationPrice {
				return providerNormalizationError(field+".tier", mapping.Tier, "is only valid for pricing targets")
			}
			if mapping.Tier.Type != ModelPricingTierTypeContext || mapping.Tier.Size <= 0 {
				return providerNormalizationError(field+".tier", mapping.Tier, "requires context type and a positive size")
			}
		}
		if len(mapping.Values) > 0 {
			if mapping.To != "lifecycle" {
				return providerNormalizationError(field+".values", mapping.Values, "is only valid for lifecycle mappings")
			}
			for source, destination := range mapping.Values {
				if strings.TrimSpace(source) == "" || !validMappedLifecycle(destination) {
					return providerNormalizationError(field+".values", mapping.Values, "requires non-empty source values and supported lifecycle destinations")
				}
			}
		}
		switch targetKind {
		case providerTargetTokenPrice:
			if !validPricingCurrency(mapping.Currency) {
				return providerNormalizationError(field+".currency", mapping.Currency, "must be a three-letter uppercase currency code")
			}
			if _, err := NormalizeProviderTokenPrice(0, mapping.Unit); err != nil {
				return providerNormalizationError(field+".unit", mapping.Unit, "is incompatible with a token pricing target")
			}
		case providerTargetOperationPrice:
			if !validPricingCurrency(mapping.Currency) {
				return providerNormalizationError(field+".currency", mapping.Currency, "must be a three-letter uppercase currency code")
			}
			if mapping.Unit != ProviderNormalizationUnitPerOperation {
				return providerNormalizationError(field+".unit", mapping.Unit, "must be per_operation for an operation pricing target")
			}
		default:
			if mapping.Unit != "" || mapping.Currency != "" || mapping.Mode != "" || mapping.Tier != nil {
				return providerNormalizationError(field, mapping, "non-pricing targets cannot declare pricing unit, currency, mode, or tier")
			}
		}
		key := fmt.Sprintf("%s|%s|%v", mapping.Mode, mapping.To, mapping.Tier)
		if _, found := seenTargets[key]; found {
			return providerNormalizationError(field+".to", mapping.To, "duplicates a canonical target in the same mode and tier")
		}
		seenTargets[key] = struct{}{}
	}
	return nil
}

func validMappedLifecycle(value string) bool {
	switch ModelStatus(value) {
	case ModelStatusActive, ModelStatusBeta, ModelStatusPreview, ModelStatusDeprecated, ModelStatusUnknown:
		return true
	default:
		return false
	}
}

func providerMappingTargetKind(target string) string {
	switch target {
	case "limits.context_window", "limits.input_tokens", "limits.output_tokens", "name", "description", "metadata.tags", "lifecycle",
		"features.tools", "features.reasoning", "features.structured_outputs":
		return "field"
	case "pricing.tokens.input", "pricing.tokens.output", "pricing.tokens.reasoning", "pricing.tokens.cache_read", "pricing.tokens.cache_write":
		return providerTargetTokenPrice
	case "pricing.operations.request", "pricing.operations.image_input", "pricing.operations.audio_input", "pricing.operations.video_input",
		"pricing.operations.image_gen", "pricing.operations.audio_gen", "pricing.operations.video_gen", "pricing.operations.web_search",
		"pricing.operations.function_call", "pricing.operations.tool_use":
		return providerTargetOperationPrice
	}
	parts := strings.Split(target, ".")
	if len(parts) == 3 && parts[0] == "extensions" && safeProviderPathSegment(parts[1]) && safeProviderPathSegment(parts[2]) {
		if sensitiveProviderField(parts[1]) || sensitiveProviderField(parts[2]) {
			return ""
		}
		return "field"
	}
	return ""
}

func sensitiveProviderField(value string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(value, "-", "_"))
	switch normalized {
	case "api_key", "apikey", "access_token", "auth_token", "authorization", "credential", "credentials", "password", "secret":
		return true
	default:
		return false
	}
}

func safeProviderPathSegment(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '_' || (index > 0 && character >= '0' && character <= '9') || (index > 0 && character == '-') {
			continue
		}
		return false
	}
	return true
}

func providerNormalizationError(field string, value any, message string) error {
	return &errors.ValidationError{Field: field, Value: value, Message: message}
}
