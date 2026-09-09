package catalogs

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
)

// TokenCostUnit identifies one observed token price unit.
type TokenCostUnit string

const (
	// CostUnitPerToken identifies the amount for one token.
	CostUnitPerToken TokenCostUnit = "per_token"
	// CostUnitPerMillion identifies the amount for one million tokens.
	CostUnitPerMillion TokenCostUnit = "per_1m_tokens"
)

const (
	tokenCostTokenMask uint8 = 1 << iota
	tokenCostMillionMask
)

// Amount returns the amount and its observed presence without unit conversion.
// Go values constructed without presence setters retain legacy known-zero behavior.
// After decoding, callers use SetAmount to replace a missing or unknown amount with zero.
func (cost *ModelTokenCost) Amount(unit TokenCostUnit) (float64, ValuePresence) {
	value, mask := cost.unit(unit)
	if value == nil {
		return 0, ValueMissing
	}
	if *value != 0 {
		return *value, ValueKnown
	}
	if cost.unknownUnits&mask != 0 {
		return 0, ValueUnknown
	}
	if cost.absentUnits&mask != 0 || cost.legacyMissingUnits()&mask != 0 {
		return 0, ValueMissing
	}
	return 0, ValueKnown
}

// SetAmount records a known amount, including zero, and clears the alternate unit.
// Invalid units return false.
func (cost *ModelTokenCost) SetAmount(unit TokenCostUnit, amount float64) bool {
	value, mask := cost.unit(unit)
	if value == nil {
		return false
	}
	other := CostUnitPerMillion
	if unit == CostUnitPerMillion {
		other = CostUnitPerToken
	}
	cost.UnsetAmount(other)
	*value = amount
	cost.absentUnits &^= mask
	cost.unknownUnits &^= mask
	return true
}

// SetAmountUnknown records an explicit unknown amount. Invalid units return false.
func (cost *ModelTokenCost) SetAmountUnknown(unit TokenCostUnit) bool {
	value, mask := cost.unit(unit)
	if value == nil {
		return false
	}
	cost.absentUnits |= cost.legacyMissingUnits()
	*value = 0
	cost.absentUnits &^= mask
	cost.unknownUnits |= mask
	return true
}

// UnsetAmount removes an observed claim. Invalid units return false.
func (cost *ModelTokenCost) UnsetAmount(unit TokenCostUnit) bool {
	value, mask := cost.unit(unit)
	if value == nil {
		return false
	}
	cost.absentUnits |= cost.legacyMissingUnits()
	*value = 0
	cost.absentUnits |= mask
	cost.unknownUnits &^= mask
	return true
}

func (cost *ModelTokenCost) unit(unit TokenCostUnit) (*float64, uint8) {
	if cost == nil {
		return nil, 0
	}
	switch unit {
	case CostUnitPerToken:
		return &cost.PerToken, tokenCostTokenMask
	case CostUnitPerMillion:
		return &cost.Per1M, tokenCostMillionMask
	default:
		return nil, 0
	}
}

// MarshalJSON preserves unit presence, legacy zero placeholders, and field order.
func (cost ModelTokenCost) MarshalJSON() ([]byte, error) {
	type plain ModelTokenCost
	if cost.absentUnits|cost.unknownUnits == 0 {
		return json.Marshal(plain(cost))
	}
	result := []byte{'{'}
	for _, unit := range []TokenCostUnit{CostUnitPerToken, CostUnitPerMillion} {
		value, state := cost.Amount(unit)
		if state == ValueMissing {
			continue
		}
		if len(result) > 1 {
			result = append(result, ',')
		}
		result = append(result, '"')
		result = append(result, string(unit)...)
		result = append(result, '"', ':')
		if state == ValueUnknown {
			result = append(result, "null"...)
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		result = append(result, encoded...)
	}
	return append(result, '}'), nil
}

// UnmarshalJSON restores observed unit presence and clears reused state.
func (cost *ModelTokenCost) UnmarshalJSON(data []byte) error {
	type plain ModelTokenCost
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*cost = ModelTokenCost(decoded)
	for _, unit := range []TokenCostUnit{CostUnitPerToken, CostUnitPerMillion} {
		value, present := raw[string(unit)]
		if !present {
			cost.UnsetAmount(unit)
		} else if isJSONNull(value) {
			cost.SetAmountUnknown(unit)
		}
	}
	return nil
}

// MarshalYAML preserves token amount presence and legacy zero placeholders.
// A cost with no claimed units cannot use YAML, where an empty mapping means free.
func (cost ModelTokenCost) MarshalYAML() (any, error) {
	type plain ModelTokenCost
	if cost.absentUnits|cost.unknownUnits == 0 {
		return plain(cost), nil
	}
	if _, token := cost.Amount(CostUnitPerToken); token == ValueMissing {
		if _, million := cost.Amount(CostUnitPerMillion); million == ValueMissing {
			return nil, pricingValidationError("token_cost", cost, "cannot encode missing units as a legacy free YAML mapping")
		}
	}
	entries := make(yaml.MapSlice, 0, 2)
	for _, unit := range []TokenCostUnit{CostUnitPerToken, CostUnitPerMillion} {
		value, state := cost.Amount(unit)
		if state == ValueMissing {
			continue
		}
		key := string(unit)
		if unit == CostUnitPerMillion {
			key = "per_1m"
		}
		var encoded any = value
		if state == ValueUnknown {
			encoded = nil
		}
		entries = append(entries, yaml.MapItem{Key: key, Value: encoded})
	}
	return entries, nil
}

// UnmarshalYAML restores observed unit presence and clears reused state.
// An empty mapping retains the free-price meaning of the legacy YAML encoder.
func (cost *ModelTokenCost) UnmarshalYAML(unmarshal func(any) error) error {
	type plain ModelTokenCost
	var decoded plain
	if err := unmarshal(&decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*cost = ModelTokenCost(decoded)
	if raw != nil && len(raw) == 0 {
		return nil
	}
	for _, unit := range []TokenCostUnit{CostUnitPerToken, CostUnitPerMillion} {
		key := string(unit)
		if unit == CostUnitPerMillion {
			key = "per_1m"
		}
		value, present := raw[key]
		if !present {
			cost.UnsetAmount(unit)
		} else if value == nil {
			cost.SetAmountUnknown(unit)
		}
	}
	return nil
}

// legacyMissingUnits identifies zero placeholders beside a nonzero unit price.
func (cost *ModelTokenCost) legacyMissingUnits() uint8 {
	var missing uint8
	if cost.PerToken == 0 && cost.Per1M != 0 {
		missing |= tokenCostTokenMask
	}
	if cost.Per1M == 0 && cost.PerToken != 0 {
		missing |= tokenCostMillionMask
	}
	return missing &^ cost.absentUnits &^ cost.unknownUnits
}
