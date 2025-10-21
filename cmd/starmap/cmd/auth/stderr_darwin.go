//go:build darwin

package auth

import (
	"os"
	"syscall"
)

// suppressStderr temporarily redirects stderr to /dev/null to suppress noisy SDK warnings.
//
//nolint:gosec // File descriptor manipulation is intentional for stderr suppression
func suppressStderr(fn func() error) error {
	// Save original stderr file descriptor
	origStderrFd, err := syscall.Dup(int(os.Stderr.Fd()))
	if err != nil {
		// If we can't duplicate stderr, just run the function normally
		return fn()
	}
	defer func() { _ = syscall.Close(origStderrFd) }()

	// Open /dev/null for writing
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		// If we can't open /dev/null, just run the function normally
		return fn()
	}
	defer func() { _ = devNull.Close() }()

	// Redirect stderr file descriptor to /dev/null
	if err := syscall.Dup2(int(devNull.Fd()), int(os.Stderr.Fd())); err != nil {
		// If redirection fails, just run the function normally
		return fn()
	}

	// Restore stderr file descriptor after function completes
	defer func() { _ = syscall.Dup2(origStderrFd, int(os.Stderr.Fd())) }()

	return fn()
}
