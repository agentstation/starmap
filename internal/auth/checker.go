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
// Resolver selection uses catalog authentication metadata. Endpoint protocols
// do not select credentials.
func (c *Checker) CheckProvider(provider *catalogs.Provider, supportedMap map[string]bool) *Status {
	if provider == nil {
		return &Status{State: StateInvalid, Summary: "Provider is required"}
	}
	// Check if provider is supported
	if !supportedMap[string(provider.ID)] {
		return &Status{
			State:   StateUnsupported,
			Summary: "No client implementation available",
		}
	}

	method := catalogs.ProviderCatalogAuthNone
	if provider.Catalog != nil {
		method = provider.Catalog.Auth.Method
		if method == "" {
			return &Status{
				State:   StateInvalid,
				Summary: "Catalog authentication method is required",
			}
		}
	}
	resolver, found := c.resolvers[method]
	if !found {
		return &Status{
			State:   StateUnsupported,
			Summary: fmt.Sprintf("No credential resolver for catalog auth method %s", method),
		}
	}
	return resolver.Check(provider)
}

// checkGoogleDefault checks Google default credentials using local evidence.
func checkGoogleDefault(_ *catalogs.Provider) *Status {
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
		State:   state,
		Summary: adc.FormatBrief(details),
		CredentialChain: &CredentialChainDetails{
			Method:  catalogs.ProviderCatalogAuthGoogleDefault,
			Source:  "application-default-credentials",
			Details: details,
		},
	}
}

func checkNoCredentials(_ *catalogs.Provider) *Status {
	return &Status{State: StateOptional, Summary: "No catalog credentials required"}
}

// checkAPIKey checks API key-based catalog authentication.
func checkAPIKey(provider *catalogs.Provider) *Status {
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
		if provider.IsCatalogAuthRequired() {
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
