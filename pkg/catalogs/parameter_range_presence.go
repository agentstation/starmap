package catalogs

// RangeField identifies one bound or default in a numeric parameter range.
type RangeField string

const (
	// RangeMinimum identifies the minimum value.
	RangeMinimum RangeField = "min"
	// RangeMaximum identifies the maximum value.
	RangeMaximum RangeField = "max"
	// RangeDefault identifies the default value.
	RangeDefault RangeField = "default"
)

const (
	rangeMinimumMask uint8 = 1 << iota
	rangeMaximumMask
	rangeDefaultMask
)

// Value returns a range field and its observed presence.
// Legacy Go literals retain known-zero behavior. Use SetValue after decoding
// to replace a missing or unknown field with zero.
func (r *FloatRange) Value(field RangeField) (float64, ValuePresence) {
	value, mask := r.field(field)
	if value == nil {
		return 0, ValueMissing
	}
	if *value != 0 {
		return *value, ValueKnown
	}
	if r.unknownFields&mask != 0 {
		return 0, ValueUnknown
	}
	if r.absentFields&mask != 0 {
		return 0, ValueMissing
	}
	return 0, ValueKnown
}

// SetValue records a known field, including zero. Invalid fields return false.
func (r *FloatRange) SetValue(field RangeField, value float64) bool {
	target, mask := r.field(field)
	if target == nil {
		return false
	}
	*target = value
	r.absentFields &^= mask
	r.unknownFields &^= mask
	return true
}

// SetValueUnknown records an explicit unknown field. Invalid fields return false.
func (r *FloatRange) SetValueUnknown(field RangeField) bool {
	target, mask := r.field(field)
	if target == nil {
		return false
	}
	*target = 0
	r.absentFields &^= mask
	r.unknownFields |= mask
	return true
}

// UnsetValue removes a field claim. Invalid fields return false.
func (r *FloatRange) UnsetValue(field RangeField) bool {
	target, mask := r.field(field)
	if target == nil {
		return false
	}
	*target = 0
	r.absentFields |= mask
	r.unknownFields &^= mask
	return true
}

func (r *FloatRange) field(field RangeField) (*float64, uint8) {
	if r == nil {
		return nil, 0
	}
	switch field {
	case RangeMinimum:
		return &r.Min, rangeMinimumMask
	case RangeMaximum:
		return &r.Max, rangeMaximumMask
	case RangeDefault:
		return &r.Default, rangeDefaultMask
	default:
		return nil, 0
	}
}

// Value returns a range field and its observed presence.
// Legacy Go literals retain known-zero behavior. Use SetValue after decoding
// to replace a missing or unknown field with zero.
func (r *IntRange) Value(field RangeField) (int, ValuePresence) {
	value, mask := r.field(field)
	if value == nil {
		return 0, ValueMissing
	}
	if *value != 0 {
		return *value, ValueKnown
	}
	if r.unknownFields&mask != 0 {
		return 0, ValueUnknown
	}
	if r.absentFields&mask != 0 {
		return 0, ValueMissing
	}
	return 0, ValueKnown
}

// SetValue records a known field, including zero. Invalid fields return false.
func (r *IntRange) SetValue(field RangeField, value int) bool {
	target, mask := r.field(field)
	if target == nil {
		return false
	}
	*target = value
	r.absentFields &^= mask
	r.unknownFields &^= mask
	return true
}

// SetValueUnknown records an explicit unknown field. Invalid fields return false.
func (r *IntRange) SetValueUnknown(field RangeField) bool {
	target, mask := r.field(field)
	if target == nil {
		return false
	}
	*target = 0
	r.absentFields &^= mask
	r.unknownFields |= mask
	return true
}

// UnsetValue removes a field claim. Invalid fields return false.
func (r *IntRange) UnsetValue(field RangeField) bool {
	target, mask := r.field(field)
	if target == nil {
		return false
	}
	*target = 0
	r.absentFields |= mask
	r.unknownFields &^= mask
	return true
}

func (r *IntRange) field(field RangeField) (*int, uint8) {
	if r == nil {
		return nil, 0
	}
	switch field {
	case RangeMinimum:
		return &r.Min, rangeMinimumMask
	case RangeMaximum:
		return &r.Max, rangeMaximumMask
	case RangeDefault:
		return &r.Default, rangeDefaultMask
	default:
		return nil, 0
	}
}
