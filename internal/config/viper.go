package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/spf13/viper"
)

// GetString is a helper to get string values from Viper.
// It checks both OS environment variables and Viper configuration.
func GetString(key string) string {
	// Check OS env directly first
	osValue := os.Getenv(key)
	viperValue := viper.GetString(key)

	// If Viper doesn't have it but OS does, return OS value
	if viperValue == "" && osValue != "" {
		return osValue
	}
	return viperValue
}

// GetAPIKey retrieves and validates an API key for a provider using Viper.
// It returns the API key if found and valid, or an error if required but missing.
// This version checks both Viper configuration and environment variables.
//
// Deprecated: Consider using provider.GetAPIKey() for simpler environment-only access.
// This function is kept for backward compatibility and advanced Viper integration.
func GetAPIKey(provider *catalogs.Provider) (string, error) {
	if provider.APIKey == nil {
		return "", nil
	}

	// Get the API key from Viper (handles env vars and config files)
	apiKey := GetString(provider.APIKey.Name)
	if apiKey == "" {
		// Check if API key is required
		if provider.IsAPIKeyRequired() {
			return "", fmt.Errorf("environment variable %s not set", provider.APIKey.Name)
		}
		return "", nil
	}

	// Validate against pattern if specified
	if provider.APIKey.Pattern != "" && provider.APIKey.Pattern != ".*" {
		matched, err := regexp.MatchString(provider.APIKey.Pattern, apiKey)
		if err != nil {
			return "", fmt.Errorf("invalid pattern %s: %w", provider.APIKey.Pattern, err)
		}
		if !matched {
			return "", fmt.Errorf("API key does not match required pattern for provider %s", provider.ID)
		}
	}

	return apiKey, nil
}

// CheckAPIKeyConfigured checks if an API key is configured for a provider without validating it.
// This is useful for status checks where we just want to know if something is set.
//
// Deprecated: Use provider.HasAPIKey() instead. This function is kept for backward compatibility.
func CheckAPIKeyConfigured(provider *catalogs.Provider) bool {
	return provider.HasAPIKey()
}
