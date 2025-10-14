package ptr

// To creates a pointer to the given value.
// This is a generic utility function that works with any type.
func To[T any](v T) *T {
	return &v
}

// String creates a pointer to the given string value.
func String(s string) *string {
	return &s
}

// Bool creates a pointer to the given bool value.
func Bool(b bool) *bool {
	return &b
}

// Int creates a pointer to the given int value.
func Int(i int) *int {
	return &i
}

// Int64 creates a pointer to the given int64 value.
func Int64(i int64) *int64 {
	return &i
}

// Float64 creates a pointer to the given float64 value.
func Float64(f float64) *float64 {
	return &f
}
