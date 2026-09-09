package catalogs

// SetQuantized records a known quantization claim, including false.
// Use this method to replace a previously decoded claim.
func (a *ModelArchitecture) SetQuantized(value bool) {
	a.Quantized = value
	a.quantizedPresence = ValueKnown
}

// SetQuantizedUnknown records an unknown quantization claim.
func (a *ModelArchitecture) SetQuantizedUnknown() {
	a.Quantized = false
	a.quantizedPresence = ValueUnknown
}

// UnsetQuantized removes the quantization claim.
func (a *ModelArchitecture) UnsetQuantized() {
	a.Quantized = false
	a.quantizedPresence = ValueMissing
}

// QuantizedValue returns the quantization claim and its presence.
// Nil receivers and false values without recorded presence return ValueMissing.
func (a *ModelArchitecture) QuantizedValue() (bool, ValuePresence) {
	if a == nil {
		return false, ValueMissing
	}
	if a.quantizedPresence != ValueMissing {
		return a.Quantized, a.quantizedPresence
	}
	if a.Quantized {
		return true, ValueKnown
	}
	return false, ValueMissing
}

// SetFineTuned records a known fine-tuning claim, including false.
// Use this method to replace a previously decoded claim.
func (a *ModelArchitecture) SetFineTuned(value bool) {
	a.FineTuned = value
	a.fineTunedPresence = ValueKnown
}

// SetFineTunedUnknown records an unknown fine-tuning claim.
func (a *ModelArchitecture) SetFineTunedUnknown() {
	a.FineTuned = false
	a.fineTunedPresence = ValueUnknown
}

// UnsetFineTuned removes the fine-tuning claim.
func (a *ModelArchitecture) UnsetFineTuned() {
	a.FineTuned = false
	a.fineTunedPresence = ValueMissing
}

// FineTunedValue returns the fine-tuning claim and its presence.
// Nil receivers and false values without recorded presence return ValueMissing.
func (a *ModelArchitecture) FineTunedValue() (bool, ValuePresence) {
	if a == nil {
		return false, ValueMissing
	}
	if a.fineTunedPresence != ValueMissing {
		return a.FineTuned, a.fineTunedPresence
	}
	if a.FineTuned {
		return true, ValueKnown
	}
	return false, ValueMissing
}

// Equal compares architecture facts and Boolean presence states.
func (a ModelArchitecture) Equal(other ModelArchitecture) bool {
	return equalPresenceJSON(a, other)
}
