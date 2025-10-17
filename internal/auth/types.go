// Package auth provides authentication checking for AI model providers.
package auth

// State represents the authentication state of a provider.
type State int

const (
	// StateConfigured means the provider has credentials configured.
	StateConfigured State = iota
	// StateMissing means required credentials are missing.
	StateMissing
	// StateInvalid means credentials are found but malformed or invalid.
	StateInvalid
	// StateOptional means the provider has optional or no auth requirements.
	StateOptional
	// StateUnsupported means the provider has no client implementation.
	StateUnsupported
)

// Status represents authentication status with type-safe details.
//
// GoogleCloud field contains *adc.Details (from internal/auth/adc package).
// It's defined as interface{} to avoid import cycles, but callers can safely
// type assert to *adc.Details when needed.
type Status struct {
	State       State
	Summary     string         // Brief one-line summary
	APIKey      *APIKeyDetails // For API key providers (nil if not applicable)
	GoogleCloud interface{}    // For Google Cloud providers (*adc.Details, nil if not applicable)
}

// APIKeyDetails contains API key authentication details.
type APIKeyDetails struct {
	EnvVar  string // Environment variable name
	IsSet   bool   // Whether the env var is set
	IsValid bool   // Whether the value matches required pattern
	Source  string // Where credentials come from (e.g., "env")
}

// Checker checks authentication status for providers.
type Checker struct {
	// Add fields as needed for caching, etc.
}

// NewChecker creates a new authentication checker.
func NewChecker() *Checker {
	return &Checker{}
}

// GCloudStatus represents Google Cloud authentication status.
// Deprecated: Use GoogleCloudDetails instead.
type GCloudStatus struct {
	Authenticated     bool
	Project           string
	Location          string
	HasVertexProvider bool
}
