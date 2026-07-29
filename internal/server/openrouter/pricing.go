package openrouter

import (
	"math"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func projectPricing(pricing *catalogs.ModelPricing) *Pricing {
	if pricing == nil || pricing.Currency != catalogs.ModelPricingCurrencyUSD {
		return nil
	}
	result := &Pricing{}
	if pricing.Tokens != nil {
		result.Prompt = tokenPrice(pricing.Tokens.Input)
		result.Completion = tokenPrice(pricing.Tokens.Output)
		result.InternalReasoning = tokenPrice(pricing.Tokens.Reasoning)
		result.InputCacheRead = tokenPrice(pricing.Tokens.CacheRead)
		result.InputCacheWrite = tokenPrice(pricing.Tokens.CacheWrite)
	}
	if pricing.Operations != nil {
		result.Request = operationPrice(pricing.Operations.Request)
		result.Image = operationPrice(pricing.Operations.ImageInput)
		result.WebSearch = operationPrice(pricing.Operations.WebSearch)
	}
	if *result == (Pricing{}) {
		return nil
	}
	return result
}

func tokenPrice(cost *catalogs.ModelTokenCost) *string {
	if cost == nil {
		return nil
	}
	value := cost.PerToken
	if value == 0 {
		value = cost.Per1M / 1_000_000
	}
	return priceString(value)
}

func operationPrice(value *float64) *string {
	if value == nil {
		return nil
	}
	return priceString(*value)
}

func priceString(value float64) *string {
	formatted := strconv.FormatFloat(value, 'f', -1, 64)
	return &formatted
}

func preferredOffering(
	offerings []catalogs.ProviderOffering,
) *catalogs.ProviderOffering {
	if len(offerings) == 0 {
		return nil
	}
	best := offerings[0]
	for _, candidate := range offerings[1:] {
		if compareOfferingPreference(candidate, best) < 0 {
			best = candidate
		}
	}
	return &best
}

func compareOfferingPreference(
	left, right catalogs.ProviderOffering,
) int {
	leftScore, leftPriced := offeringPriceScore(left.Pricing)
	rightScore, rightPriced := offeringPriceScore(right.Pricing)
	if leftPriced != rightPriced {
		if leftPriced {
			return -1
		}
		return 1
	}
	if leftScore < rightScore {
		return -1
	}
	if leftScore > rightScore {
		return 1
	}
	if compared := strings.Compare(string(left.ProviderID), string(right.ProviderID)); compared != 0 {
		return compared
	}
	return strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID))
}

func offeringPriceScore(pricing *catalogs.ModelPricing) (float64, bool) {
	if pricing == nil || pricing.Currency != catalogs.ModelPricingCurrencyUSD {
		return math.Inf(1), false
	}
	if pricing.Tokens != nil {
		for _, cost := range []*catalogs.ModelTokenCost{
			pricing.Tokens.Input,
			pricing.Tokens.Output,
		} {
			if cost != nil {
				if cost.PerToken != 0 {
					return cost.PerToken, true
				}
				return cost.Per1M / 1_000_000, true
			}
		}
	}
	if pricing.Operations != nil && pricing.Operations.Request != nil {
		return *pricing.Operations.Request, true
	}
	return math.Inf(1), false
}
