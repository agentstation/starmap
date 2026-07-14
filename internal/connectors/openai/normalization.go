package openai

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func applyMappedPricing(model *catalogs.Model, mapping catalogs.FieldMapping, sourceValue any) error {
	value, ok := mappedFloat(sourceValue)
	if !ok {
		return &errors.ValidationError{Field: mapping.From, Value: sourceValue, Message: "must contain a numeric pricing value"}
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return &errors.ValidationError{Field: mapping.From, Value: value, Message: "pricing must be finite and non-negative"}
	}
	if mapping.Scale != nil {
		value *= *mapping.Scale
	}

	pricing, tier, err := mappedPricingContainer(model, mapping)
	if err != nil {
		return err
	}
	if pricing.Currency != "" && pricing.Currency != mapping.Currency {
		return &errors.ValidationError{Field: mapping.To + ".currency", Value: mapping.Currency, Message: "conflicts with another mapped pricing currency"}
	}
	pricing.Currency = mapping.Currency
	if tier != nil {
		return setMappedPricingComponent(&tier.Tokens, &tier.Operations, mapping, value)
	}
	return setMappedPricingComponent(&pricing.Tokens, &pricing.Operations, mapping, value)
}

func mappedPricingContainer(model *catalogs.Model, mapping catalogs.FieldMapping) (*catalogs.ModelPricing, *catalogs.ModelPricingTier, error) {
	var pricing *catalogs.ModelPricing
	if mapping.Mode == "" {
		if model.Pricing == nil {
			model.Pricing = &catalogs.ModelPricing{}
		}
		pricing = model.Pricing
	} else {
		if model.Modes == nil {
			model.Modes = make(map[string]catalogs.ModelMode)
		}
		mode := model.Modes[mapping.Mode]
		if mode.Pricing == nil {
			mode.Pricing = &catalogs.ModelPricing{}
		}
		pricing = mode.Pricing
		model.Modes[mapping.Mode] = mode
	}
	if mapping.Tier == nil {
		return pricing, nil, nil
	}
	for index := range pricing.Tiers {
		tier := &pricing.Tiers[index]
		if tier.Type == mapping.Tier.Type && tier.Size == mapping.Tier.Size {
			if tier.Name != mapping.Tier.Name {
				return nil, nil, &errors.ValidationError{Field: mapping.To + ".tier.name", Value: mapping.Tier.Name, Message: "conflicts with another mapping for this tier"}
			}
			return pricing, tier, nil
		}
	}
	pricing.Tiers = append(pricing.Tiers, catalogs.ModelPricingTier{Name: mapping.Tier.Name, Type: mapping.Tier.Type, Size: mapping.Tier.Size})
	return pricing, &pricing.Tiers[len(pricing.Tiers)-1], nil
}

func setMappedPricingComponent(tokens **catalogs.ModelTokenPricing, operations **catalogs.ModelOperationPricing, mapping catalogs.FieldMapping, value float64) error {
	if strings.HasPrefix(mapping.To, "pricing.tokens.") {
		cost, err := catalogs.NormalizeProviderTokenPrice(value, mapping.Unit)
		if err != nil {
			return err
		}
		if *tokens == nil {
			*tokens = &catalogs.ModelTokenPricing{}
		}
		switch strings.TrimPrefix(mapping.To, "pricing.tokens.") {
		case "input":
			(*tokens).Input = &cost
		case "output":
			(*tokens).Output = &cost
		case featureReasoning:
			(*tokens).Reasoning = &cost
		case "cache_read":
			(*tokens).CacheRead = &cost
		case "cache_write":
			(*tokens).CacheWrite = &cost
		}
		return nil
	}
	if *operations == nil {
		*operations = &catalogs.ModelOperationPricing{}
	}
	price := value
	switch strings.TrimPrefix(mapping.To, "pricing.operations.") {
	case "request":
		(*operations).Request = &price
	case "image_input":
		(*operations).ImageInput = &price
	case "audio_input":
		(*operations).AudioInput = &price
	case "video_input":
		(*operations).VideoInput = &price
	case "image_gen":
		(*operations).ImageGen = &price
	case "audio_gen":
		(*operations).AudioGen = &price
	case "video_gen":
		(*operations).VideoGen = &price
	case "web_search":
		(*operations).WebSearch = &price
	case "function_call":
		(*operations).FunctionCall = &price
	case "tool_use":
		(*operations).ToolUse = &price
	default:
		return &errors.ValidationError{Field: mapping.To, Message: "unknown operation pricing target"}
	}
	return nil
}

func mappedFloat(value any) (float64, bool) {
	reflected := reflect.ValueOf(value)
	for reflected.IsValid() && (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) {
		if reflected.IsNil() {
			return 0, false
		}
		reflected = reflected.Elem()
	}
	if !reflected.IsValid() {
		return 0, false
	}
	switch reflected.Kind() {
	case reflect.Float32, reflect.Float64:
		return reflected.Convert(reflect.TypeOf(float64(0))).Float(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(reflected.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(reflected.Uint()), true
	default:
		if number, ok := value.(json.Number); ok {
			parsed, err := number.Float64()
			return parsed, err == nil
		}
		if text, ok := value.(string); ok {
			parsed, err := strconv.ParseFloat(text, 64)
			return parsed, err == nil
		}
		return 0, false
	}
}

func mappedBool(value any) (bool, bool) {
	reflected := reflect.ValueOf(value)
	for reflected.IsValid() && (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) {
		if reflected.IsNil() {
			return false, false
		}
		reflected = reflected.Elem()
	}
	if !reflected.IsValid() || reflected.Kind() != reflect.Bool {
		return false, false
	}
	return reflected.Bool(), true
}
