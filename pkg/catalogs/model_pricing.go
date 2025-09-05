package catalogs

const (
	// ModelPricingCurrencyUSD is the US Dollar currency constant.
	ModelPricingCurrencyUSD ModelPricingCurrency = "USD" // US Dollar
	// ModelPricingCurrencyEUR is the Euro currency constant.
	ModelPricingCurrencyEUR ModelPricingCurrency = "EUR" // Euro
	// ModelPricingCurrencyJPY is the Japanese Yen currency constant.
	ModelPricingCurrencyJPY ModelPricingCurrency = "JPY" // Japanese Yen
	// ModelPricingCurrencyGBP is the British Pound Sterling currency constant.
	ModelPricingCurrencyGBP ModelPricingCurrency = "GBP" // British Pound Sterling
	// ModelPricingCurrencyAUD is the Australian Dollar currency constant.
	ModelPricingCurrencyAUD ModelPricingCurrency = "AUD" // Australian Dollar
	// ModelPricingCurrencyCAD is the Canadian Dollar currency constant.
	ModelPricingCurrencyCAD ModelPricingCurrency = "CAD" // Canadian Dollar
	// ModelPricingCurrencyCNY is the Chinese Yuan currency constant.
	ModelPricingCurrencyCNY ModelPricingCurrency = "CNY" // Chinese Yuan
	// ModelPricingCurrencyNZD is the New Zealand Dollar currency constant.
	ModelPricingCurrencyNZD ModelPricingCurrency = "NZD" // New Zealand Dollar
)

// ModelPricingCurrency represents a currency code for model pricing.
type ModelPricingCurrency string

// String returns the string representation of a ModelPricingCurrency.
func (m ModelPricingCurrency) String() string {
	return string(m)
}

// Symbol returns the symbol for a given currency.
func (m ModelPricingCurrency) Symbol() string {
	switch m {
	case ModelPricingCurrencyUSD:
		return "$"
	case ModelPricingCurrencyEUR:
		return "€"
	case ModelPricingCurrencyJPY:
		return "¥"
	case ModelPricingCurrencyGBP:
		return "£"
	case ModelPricingCurrencyAUD:
		return "$"
	case ModelPricingCurrencyCAD:
		return "$"
	case ModelPricingCurrencyCNY:
		return "¥"
	case ModelPricingCurrencyNZD:
		return "$"
	default:
		// Default to USD symbol for unknown or empty currencies
		if m == "" || m == ModelPricingCurrency("") {
			return "$"
		}
		// For any unrecognized currency, default to USD
		return "$"
	}
}

// ModelPricing represents the pricing structure for a model.
type ModelPricing struct {
	// Token-based costs
	Tokens *ModelTokenPricing `json:"tokens,omitempty" yaml:"tokens,omitempty"`

	// Fixed costs per operation
	Operations *ModelOperationPricing `json:"operations,omitempty" yaml:"operations,omitempty"`

	// Metadata
	Currency ModelPricingCurrency `json:"currency" yaml:"currency"` // "USD", "EUR", etc.
}

// ModelTokenPricing represents all token-based costs.
type ModelTokenPricing struct {
	// Core tokens
	Input  *ModelTokenCost `json:"input,omitempty" yaml:"input,omitempty"`   // Input/prompt tokens
	Output *ModelTokenCost `json:"output,omitempty" yaml:"output,omitempty"` // Standard output tokens

	// Advanced token types
	Reasoning *ModelTokenCost         `json:"reasoning,omitempty" yaml:"reasoning,omitempty"` // Internal reasoning tokens
	Cache     *ModelTokenCachePricing `json:"cache,omitempty" yaml:"cache,omitempty"`         // Cache operations

	// Alternative flat cache structure (for backward compatibility)
	CacheRead  *ModelTokenCost `json:"cache_read,omitempty" yaml:"cache_read,omitempty"`   // Cache read costs (flat structure)
	CacheWrite *ModelTokenCost `json:"cache_write,omitempty" yaml:"cache_write,omitempty"` // Cache write costs (flat structure)
}

// MarshalYAML implements custom YAML marshaling for TokenPricing to use flat cache structure.
func (t *ModelTokenPricing) MarshalYAML() (any, error) {
	result := make(map[string]any)

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

// ModelTokenCachePricing represents cache-specific pricing.
type ModelTokenCachePricing struct {
	Read  *ModelTokenCost `json:"read,omitempty" yaml:"read,omitempty"`   // Cache read costs
	Write *ModelTokenCost `json:"write,omitempty" yaml:"write,omitempty"` // Cache write costs
}

// ModelTokenCost represents cost per token with flexible units.
type ModelTokenCost struct {
	PerToken float64 `json:"per_token" yaml:"per_token"`  // Cost per individual token
	Per1M    float64 `json:"per_1m_tokens" yaml:"per_1m"` // Cost per 1M tokens
}

// MarshalYAML implements custom YAML marshaling for TokenCost to format decimals consistently.
func (t *ModelTokenCost) MarshalYAML() (any, error) {
	result := make(map[string]float64)

	if t.PerToken != 0 {
		result["per_token"] = t.PerToken
	}

	if t.Per1M != 0 {
		result["per_1m"] = t.Per1M
	}

	return result, nil
}

// ModelOperationPricing represents fixed costs for operations.
type ModelOperationPricing struct {
	// Core operations
	Request *float64 `json:"request,omitempty" yaml:"request,omitempty"` // Cost per API request

	// Media operations
	ImageInput *float64 `json:"image_input,omitempty" yaml:"image_input,omitempty"` // Cost per image processed
	AudioInput *float64 `json:"audio_input,omitempty" yaml:"audio_input,omitempty"` // Cost per audio input
	VideoInput *float64 `json:"video_input,omitempty" yaml:"video_input,omitempty"` // Cost per video input

	// Generation operations
	ImageGen *float64 `json:"image_gen,omitempty" yaml:"image_gen,omitempty"` // Cost per image generated
	AudioGen *float64 `json:"audio_gen,omitempty" yaml:"audio_gen,omitempty"` // Cost per audio generated
	VideoGen *float64 `json:"video_gen,omitempty" yaml:"video_gen,omitempty"` // Cost per video generated

	// Service operations
	WebSearch    *float64 `json:"web_search,omitempty" yaml:"web_search,omitempty"`       // Cost per web search
	FunctionCall *float64 `json:"function_call,omitempty" yaml:"function_call,omitempty"` // Cost per function call
	ToolUse      *float64 `json:"tool_use,omitempty" yaml:"tool_use,omitempty"`           // Cost per tool usage
}
