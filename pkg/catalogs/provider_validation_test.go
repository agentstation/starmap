package catalogs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAllProviders(t *testing.T) {
	// Create a test catalog with various provider configurations
	cat, err := New()
	require.NoError(t, err)

	// Add test providers
	trueVal := true
	
	providers := []Provider{
		{
			ID:   "configured-provider",
			Name: "Configured Provider",
			APIKey: &ProviderAPIKey{
				Name:    "TEST_API_KEY",
				Pattern: ".*",
			},
			Catalog: &ProviderCatalog{
				APIKeyRequired: &trueVal,
			},
		},
		{
			ID:   "missing-provider",
			Name: "Missing Provider",
			APIKey: &ProviderAPIKey{
				Name:    "MISSING_API_KEY",
				Pattern: ".*",
			},
			Catalog: &ProviderCatalog{
				APIKeyRequired: &trueVal,
			},
		},
		{
			ID:   "optional-provider",
			Name: "Optional Provider",
			// No API key configuration - provider works without authentication
		},
		{
			ID:   "unsupported-provider",
			Name: "Unsupported Provider",
		},
	}

	for _, p := range providers {
		err := cat.SetProvider(p)
		require.NoError(t, err)
	}

	// Set environment variable for configured provider
	t.Setenv("TEST_API_KEY", "test-key-value")

	// Define supported providers (only the first three)
	supportedProviders := []ProviderID{
		"configured-provider",
		"missing-provider",
		"optional-provider",
	}

	// Validate all providers
	report, err := ValidateAllProviders(cat, supportedProviders)
	require.NoError(t, err)
	assert.NotNil(t, report)

	// Check configured providers (includes the one with API key set and the optional one)
	assert.Len(t, report.Configured, 2)
	configuredIDs := []string{
		string(report.Configured[0].Provider.ID),
		string(report.Configured[1].Provider.ID),
	}
	assert.Contains(t, configuredIDs, "configured-provider")
	assert.Contains(t, configuredIDs, "optional-provider")

	// Check missing providers
	assert.Len(t, report.Missing, 1)
	assert.Equal(t, "missing-provider", string(report.Missing[0].Provider.ID))

	// Check optional providers (none in this test - optional means no auth setup at all)
	assert.Len(t, report.Optional, 0)

	// Check unsupported providers
	assert.Len(t, report.Unsupported, 1)
	assert.Equal(t, "unsupported-provider", string(report.Unsupported[0].Provider.ID))
}

func TestProviderValidationReport_Print(t *testing.T) {
	// Create a sample report
	report := &ProviderValidationReport{
		Configured: []ProviderValidationEntry{
			{
				Provider: &Provider{
					ID:   "test-configured",
					Name: "Test Configured",
				},
			},
		},
		Missing: []ProviderValidationEntry{
			{
				Provider: &Provider{
					ID:   "test-missing",
					Name: "Test Missing",
					APIKey: &ProviderAPIKey{
						Name: "TEST_API_KEY",
					},
				},
			},
		},
		Optional: []ProviderValidationEntry{
			{
				Provider: &Provider{
					ID:   "test-optional",
					Name: "Test Optional",
				},
			},
		},
		Unsupported: []ProviderValidationEntry{
			{
				Provider: &Provider{
					ID:   "test-unsupported",
					Name: "Test Unsupported",
				},
			},
		},
	}

	// This just tests that Print doesn't panic
	// Output testing would require capturing stdout
	report.Print()
	
	// Also test the convenience function
	PrintProviderValidationReport(report)
}