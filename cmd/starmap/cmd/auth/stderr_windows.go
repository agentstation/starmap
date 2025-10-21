//go:build windows

package auth

// suppressStderr on Windows just runs the function normally.
// Stderr suppression is not implemented on Windows.
func suppressStderr(fn func() error) error {
	return fn()
}
