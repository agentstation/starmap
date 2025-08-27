package catalogs

// TokenPricing represents all token-based costs.
type TokenPricing struct {
	// Core tokens
	Input  *TokenCost `json:"input,omitempty" yaml:"input,omitempty"`   // Input/prompt tokens
	Output *TokenCost `json:"output,omitempty" yaml:"output,omitempty"` // Standard output tokens

	// Advanced token types
	Reasoning *TokenCost      `json:"reasoning,omitempty" yaml:"reasoning,omitempty"` // Internal reasoning tokens
	Cache     *TokenCacheCost `json:"cache,omitempty" yaml:"cache,omitempty"`         // Cache operations

	// Alternative flat cache structure (for backward compatibility)
	CacheRead  *TokenCost `json:"cache_read,omitempty" yaml:"cache_read,omitempty"`   // Cache read costs (flat structure)
	CacheWrite *TokenCost `json:"cache_write,omitempty" yaml:"cache_write,omitempty"` // Cache write costs (flat structure)
}

// MarshalYAML implements custom YAML marshaling for TokenPricing to use flat cache structure
func (t *TokenPricing) MarshalYAML() (interface{}, error) {
	result := make(map[string]interface{})

	if t.Input != nil {
		result["input"] = t.Input
	}

	if t.Output != nil {
		result["output"] = t.Output
	}

	if t.Reasoning != nil {
		result["reasoning"] = t.Reasoning
	}

	// Use flat structure for cache pricing in YAML output, prioritizing Cache.Read over CacheRead
	if t.Cache != nil && t.Cache.Read != nil {
		result["cache_read"] = t.Cache.Read
	} else if t.CacheRead != nil {
		result["cache_read"] = t.CacheRead
	}

	if t.Cache != nil && t.Cache.Write != nil {
		result["cache_write"] = t.Cache.Write
	} else if t.CacheWrite != nil {
		result["cache_write"] = t.CacheWrite
	}

	return result, nil
}

// TokenCacheCost represents cache-specific pricing.
type TokenCacheCost struct {
	Read  *TokenCost `json:"read,omitempty" yaml:"read,omitempty"`   // Cache read costs
	Write *TokenCost `json:"write,omitempty" yaml:"write,omitempty"` // Cache write costs
}

// TokenCost represents cost per token with flexible units.
type TokenCost struct {
	PerToken float64 `json:"per_token" yaml:"per_token"`  // Cost per individual token
	Per1M    float64 `json:"per_1m_tokens" yaml:"per_1m"` // Cost per 1M tokens
}

// MarshalYAML implements custom YAML marshaling for TokenCost to format decimals consistently
func (t *TokenCost) MarshalYAML() (interface{}, error) {
	result := make(map[string]interface{})

	if t.PerToken != 0 {
		result["per_token"] = t.PerToken
	}

	if t.Per1M != 0 {
		result["per_1m"] = t.Per1M
	}

	return result, nil
}
