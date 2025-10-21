// Package emoji provides symbol constants for CLI output.
// These symbols create a consistent visual language across all command-line commands.
package emoji

// Symbol constants for CLI output provide a consistent visual language across commands.
// These symbols are used for status indicators, alerts, and user feedback in terminal output.
const (
	// Success symbols indicate positive outcomes or configured states.

	// Success represents successful completion of an operation.
	// Used for: completed operations, verified credentials, passing tests, validation.
	Success = "✓"

	// Error and warning symbols indicate problems or missing requirements.

	// Error represents failures or missing required configuration.
	// Used for: failed operations, missing API keys, validation errors.
	Error = "✗"

	// Stop represents critical stops, shutdowns, or blocking conditions.
	// Used for: graceful shutdowns, stop signals, blocking errors.
	Stop = "✗"

	// Warning represents warnings or non-critical issues.
	// Used for: deprecation notices, optional warnings.
	Warning = "!"

	// Status symbols for provider and configuration states.

	// Optional represents optional or skipped configuration.
	// Used for: optional API keys, skipped operations.
	Optional = "-"

	// Unsupported represents unsupported or unavailable features.
	// Used for: providers without client implementation, disabled features.
	Unsupported = "×"

	// Unknown represents unknown or indeterminate states.
	// Used for: unrecognized status, undefined behavior.
	Unknown = "?"

	// Information and progress symbols.

	// Info represents informational messages.
	// Used for: general information, tips, context.
	Info = "i"

	// Spinner can be used for in-progress operations (static).
	// Note: For animated spinners, use a dedicated spinner library.
	Spinner = "..."
)
