package auth

import (
	"fmt"
	"os"
	"regexp"

	"github.com/agentstation/starmap/internal/auth/adc"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// CheckProvider checks authentication status for a provider.
// Performs local checks only - no network calls are made.
//
// Returns a Status with type-safe details:
//   - Status.GoogleCloud for Google Cloud providers
//   - Status.APIKey for API key providers
func (c *Checker) CheckProvider(provider *catalogs.Provider, supportedMap map[string]bool) *Status {
	// Check if provider is supported
	if !supportedMap[string(provider.ID)] {
		return &Status{
			State:   StateUnsupported,
			Summary: "No client implementation available",
		}
	}

	// Google Cloud providers (Vertex AI, etc.)
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return c.checkGoogleCloud()
	}

	// API key providers
	return c.checkAPIKey(provider)
}

// checkGoogleCloud checks Google Cloud authentication using ADC.
func (c *Checker) checkGoogleCloud() *Status {
	details := adc.BuildDetails()

	// Map adc.State to auth.State
	var state State
	switch details.State {
	case adc.StateConfigured:
		state = StateConfigured
	case adc.StateMissing:
		state = StateMissing
	case adc.StateInvalid:
		state = StateInvalid
	default:
		state = StateInvalid
	}

	return &Status{
		State:       state,
		Summary:     adc.FormatBrief(details),
		GoogleCloud: details,
	}
}

// checkAPIKey checks API key-based authentication.
func (c *Checker) checkAPIKey(provider *catalogs.Provider) *Status {
	// No API key configured
	if provider.APIKey == nil {
		return &Status{
			State:   StateOptional,
			Summary: "No API key required",
		}
	}

	envValue := os.Getenv(provider.APIKey.Name)

	// API key not set
	if envValue == "" {
		if provider.Catalog != nil && provider.Catalog.Endpoint.AuthRequired {
			return &Status{
				State:   StateMissing,
				Summary: fmt.Sprintf("Set %s environment variable", provider.APIKey.Name),
				APIKey: &APIKeyDetails{
					EnvVar: provider.APIKey.Name,
					IsSet:  false,
				},
			}
		}
		return &Status{
			State:   StateOptional,
			Summary: fmt.Sprintf("Optional: %s not set", provider.APIKey.Name),
		}
	}

	// Validate pattern if specified
	isValid := true
	if provider.APIKey.Pattern != "" && provider.APIKey.Pattern != ".*" {
		matched, err := regexp.MatchString(provider.APIKey.Pattern, envValue)
		if err != nil || !matched {
			isValid = false
		}
	}

	if !isValid {
		return &Status{
			State:   StateInvalid,
			Summary: "API key does not match required pattern",
			APIKey: &APIKeyDetails{
				EnvVar:  provider.APIKey.Name,
				IsSet:   true,
				IsValid: false,
				Source:  "env",
			},
		}
	}

	return &Status{
		State:   StateConfigured,
		Summary: fmt.Sprintf("API key configured (%s)", provider.APIKey.Name),
		APIKey: &APIKeyDetails{
			EnvVar:  provider.APIKey.Name,
			IsSet:   true,
			IsValid: true,
			Source:  "env",
		},
	}
}
