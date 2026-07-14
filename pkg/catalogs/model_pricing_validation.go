package catalogs

import (
	"fmt"
	"math"

	"github.com/agentstation/starmap/pkg/errors"
)

const tokenPriceScale = 1_000_000

// Validate verifies that pricing is structurally complete and financially safe
// to use as an authoritative provider-offering observation.
func (p *ModelPricing) Validate() error {
	if p == nil {
		return pricingValidationError("pricing", nil, validationMessageIsRequired)
	}
	if !validPricingCurrency(p.Currency) {
		return pricingValidationError("currency", p.Currency, "must be a three-letter uppercase currency code")
	}
	if p.EffectiveFrom != nil && p.EffectiveFrom.IsZero() {
		return pricingValidationError("effective_from", p.EffectiveFrom, "must be a non-zero timestamp when present")
	}
	if p.EffectiveUntil != nil && p.EffectiveUntil.IsZero() {
		return pricingValidationError("effective_until", p.EffectiveUntil, "must be a non-zero timestamp when present")
	}
	if p.EffectiveFrom != nil && p.EffectiveUntil != nil && !p.EffectiveUntil.After(*p.EffectiveFrom) {
		return pricingValidationError("effective_until", p.EffectiveUntil, "must be after effective_from")
	}

	basePrices, err := validatePricingComponents("pricing", p.Tokens, p.Operations)
	if err != nil {
		return err
	}
	totalPrices := basePrices
	seenTiers := make(map[string]struct{}, len(p.Tiers))
	for index, tier := range p.Tiers {
		path := fmt.Sprintf("tiers[%d]", index)
		if tier.Type != ModelPricingTierTypeContext {
			return pricingValidationError(path+".type", tier.Type, "must be context")
		}
		if tier.Size <= 0 {
			return pricingValidationError(path+".size", tier.Size, "must be greater than zero")
		}
		key := fmt.Sprintf("%s:%d", tier.Type, tier.Size)
		if _, exists := seenTiers[key]; exists {
			return pricingValidationError(path, key, "must have a unique type and size")
		}
		seenTiers[key] = struct{}{}
		prices, validateErr := validatePricingComponents(path, tier.Tokens, tier.Operations)
		if validateErr != nil {
			return validateErr
		}
		if prices == 0 {
			return pricingValidationError(path, tier, "must contain at least one price")
		}
		totalPrices += prices
	}
	if totalPrices == 0 {
		return pricingValidationError("pricing", p, "must contain at least one price")
	}
	return nil
}

func validPricingCurrency(currency ModelPricingCurrency) bool {
	value := string(currency)
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validatePricingComponents(path string, tokens *ModelTokenPricing, operations *ModelOperationPricing) (int, error) {
	count := 0
	if tokens != nil {
		costs := []struct {
			name string
			cost *ModelTokenCost
		}{
			{"input", tokens.Input},
			{"output", tokens.Output},
			{"reasoning", tokens.Reasoning},
			{"cache_read", tokens.CacheRead},
			{"cache_write", tokens.CacheWrite},
		}
		if tokens.Cache != nil {
			costs = append(costs,
				struct {
					name string
					cost *ModelTokenCost
				}{"cache.read", tokens.Cache.Read},
				struct {
					name string
					cost *ModelTokenCost
				}{"cache.write", tokens.Cache.Write},
			)
		}
		for _, item := range costs {
			if item.cost == nil {
				continue
			}
			if err := validateTokenCost(path+".tokens."+item.name, item.cost); err != nil {
				return 0, err
			}
			count++
		}
	}

	if operations != nil {
		prices := []struct {
			name  string
			price *float64
		}{
			{"request", operations.Request},
			{"image_input", operations.ImageInput},
			{"audio_input", operations.AudioInput},
			{"video_input", operations.VideoInput},
			{"image_gen", operations.ImageGen},
			{"audio_gen", operations.AudioGen},
			{"video_gen", operations.VideoGen},
			{"web_search", operations.WebSearch},
			{"function_call", operations.FunctionCall},
			{"tool_use", operations.ToolUse},
		}
		for _, item := range prices {
			if item.price == nil {
				continue
			}
			if err := validatePrice(path+".operations."+item.name, *item.price); err != nil {
				return 0, err
			}
			count++
		}
	}
	return count, nil
}

func validateTokenCost(path string, cost *ModelTokenCost) error {
	if err := validatePrice(path+".per_token", cost.PerToken); err != nil {
		return err
	}
	if err := validatePrice(path+".per_1m", cost.Per1M); err != nil {
		return err
	}
	if cost.PerToken != 0 && cost.Per1M != 0 {
		expected := cost.PerToken * tokenPriceScale
		tolerance := math.Max(math.Abs(expected), math.Abs(cost.Per1M)) * 1e-9
		if math.Abs(expected-cost.Per1M) > tolerance {
			return pricingValidationError(path, *cost, "per_token and per_1m must represent the same price")
		}
	}
	return nil
}

func validatePrice(path string, price float64) error {
	if math.IsNaN(price) || math.IsInf(price, 0) {
		return pricingValidationError(path, price, "must be finite")
	}
	if price < 0 {
		return pricingValidationError(path, price, "must not be negative")
	}
	return nil
}

func pricingValidationError(field string, value any, message string) error {
	return &errors.ValidationError{Field: field, Value: value, Message: message}
}
