//go:build !darwin && !linux && !windows

package privatefiles

// ValidateAncestors applies on Linux, macOS, and Windows.
// Other platforms retain their existing access checks and require separate ancestor qualification.
func ValidateAncestors(_ string) error { return nil }
