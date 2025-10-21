//go:build !darwin

package auth

// suppressStderr on non-Darwin platforms just runs the function normally.
// Stderr suppression is only implemented on macOS (Darwin).
func suppressStderr(fn func() error) error {
	return fn()
}
