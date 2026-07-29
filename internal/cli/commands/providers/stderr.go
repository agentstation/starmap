package providers

import (
	"io"
	"os"
)

// suppressStderr temporarily redirects stderr to discard to suppress noisy SDK warnings.
// This uses pure Go (no syscalls) and works cross-platform.
//
// The Google Cloud SDK writes deprecation warnings and diagnostic messages to stderr
// during credential detection, which pollutes CLI output. This function provides a
// clean way to suppress those warnings while preserving application output.
//
// Thread safety: This manipulates the global os.Stderr variable. The caller is
// responsible for ensuring this is called appropriately (e.g., wrapping concurrent
// operations in a single call rather than calling from multiple goroutines).
func suppressStderr(fn func() error) error {
	// Save original stderr so we can restore it
	original := os.Stderr

	// Create a pipe to capture stderr
	r, w, err := os.Pipe()
	if err != nil {
		// If we can't create a pipe, just run the function normally
		return fn()
	}

	// Replace stderr with the pipe writer
	os.Stderr = w

	// Discard all output written to the pipe in a background goroutine
	// Error is intentionally ignored - this is best-effort suppression
	go func() { _, _ = io.Copy(io.Discard, r) }()

	// Ensure we restore stderr when done
	defer func() {
		_ = w.Close() // Best-effort cleanup
		_ = r.Close() // Best-effort cleanup
		os.Stderr = original
	}()

	return fn()
}
