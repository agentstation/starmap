package catalogs

import (
	"strings"
	"testing"
	"time"
)

func TestProvidersFormatYAML(t *testing.T) {
	// Create test providers similar to the expected format
	providerSlice := []Provider{
		{
			ID:           "anthropic",
			Name:         "Anthropic",
			Headquarters: stringPtr("San Francisco, CA, USA"),
			IconURL:      stringPtr("https://www.anthropic.com/favicon.ico"),
			Credentials:  testAPIKeyCredentials("ANTHROPIC_API_KEY", "x-api-key", ProviderCredentialSchemeDirect),
			Catalog: &ProviderCatalog{
				Docs: stringPtr("https://docs.anthropic.com/en/docs/about-claude/models/overview"),
				Endpoint: ProviderEndpoint{
					Type: EndpointTypeAnthropic,
					URL:  "https://api.anthropic.com/v1/models",
					ProtocolOptions: ProviderCatalogProtocolOptions{Anthropic: &ProviderAnthropicCatalogProtocolOptions{
						Version: "2023-06-01",
					}},
				},
			},
			StatusPageURL: stringPtr("https://status.anthropic.com"),
			Inference: &ProviderInference{
				BaseURL: "https://api.anthropic.com",
				Endpoints: []ProviderInferenceEndpoint{{
					Operation: ProviderOperationChatCompletions,
					Type:      EndpointTypeAnthropic,
					Path:      "/v1/messages",
				}},
			},
			PrivacyPolicy: &ProviderPrivacyPolicy{
				PrivacyPolicyURL:  stringPtr("https://www.anthropic.com/privacy"),
				TermsOfServiceURL: stringPtr("https://www.anthropic.com/terms"),
				RetainsData:       boolPtr(true),
				TrainsOnData:      boolPtr(false),
			},
			RetentionPolicy: &ProviderRetentionPolicy{
				Type:     ProviderRetentionTypeFixed,
				Duration: durationPtr(720 * time.Hour), // 30 days
				Details:  stringPtr("API inputs and outputs are automatically deleted within 30 days unless required for policy enforcement or legal compliance"),
			},
			GovernancePolicy: &ProviderGovernancePolicy{
				ModerationRequired: boolPtr(false),
				Moderated:          boolPtr(true),
				Moderator:          stringPtr("anthropic"),
			},
			Extensions: SourceExtensions{
				"models.dev": {
					Fields: map[string]any{
						"npm": "@ai-sdk/anthropic",
					},
				},
			},
		},
		{
			ID:           "cerebras",
			Name:         "Cerebras",
			Headquarters: stringPtr("Sunnyvale, CA, USA"),
			IconURL:      stringPtr("https://cerebras.ai/favicon.ico"),
			Credentials:  testAPIKeyCredentials("CEREBRAS_API_KEY", "Authorization", ProviderCredentialSchemeBearer),
			Catalog: &ProviderCatalog{
				Docs: stringPtr("https://inference-docs.cerebras.ai/models/overview"),
				Endpoint: ProviderEndpoint{
					Type: EndpointTypeOpenAI,
					URL:  "https://api.cerebras.ai/v1/models",
					ProtocolOptions: ProviderCatalogProtocolOptions{OpenAI: &ProviderOpenAICatalogProtocolOptions{
						TokenPriceUnit: ProviderTokenPriceUnitPerMillion,
					}},
				},
				Authors: []AuthorID{"alibaba", "meta", "openai"},
			},
			RetentionPolicy: &ProviderRetentionPolicy{
				Type:     ProviderRetentionTypeNone,
				Duration: durationPtr(0), // immediate
				Details:  stringPtr("API inputs and outputs are not retained for training, inference and chatbot services. Data is processed for immediate response generation and then discarded."),
			},
		},
	}

	// Create a Providers collection and add our test providers
	providers := NewProviders()
	for _, provider := range providerSlice {
		providerCopy := provider // Create a copy since Add expects a pointer
		providers.Add(&providerCopy)
	}

	// Generate YAML using the Providers.FormatYAML() method
	yamlString := providers.FormatYAML()
	t.Logf("Generated YAML:\n%s", yamlString)

	// Test specific formatting requirements
	expectedElements := []string{
		"# Anthropic",
		"- id: anthropic",
		"name: Anthropic",
		"headquarters: San Francisco, CA, USA",
		"icon_url: https://www.anthropic.com/favicon.ico",
		"credentials:",
		"- ANTHROPIC_API_KEY",
		"name: x-api-key",
		"primitive: api-key",
		"duration: 720h0m0s #30 days", // Inline comment for duration
		"extensions:",
		"models.dev:",
		"npm: \"@ai-sdk/anthropic\"",

		"# Cerebras",
		"- id: cerebras",
		"name: Cerebras",
		"authors:",
		"- alibaba",
		"- meta",
		"- openai",
		"duration: 0s", // Zero duration
	}

	for _, element := range expectedElements {
		if !strings.Contains(yamlString, element) {
			t.Errorf("YAML should contain: %s", element)
		}
	}

	// Test that providers are separated by blank lines
	lines := strings.Split(yamlString, "\n")
	foundAnthropicHeader := false
	foundCerebrasHeader := false
	foundBlankLineBeforeCerebras := false

	for i, line := range lines {
		if line == "# Anthropic" {
			foundAnthropicHeader = true
		}
		if line == "# Cerebras" {
			foundCerebrasHeader = true
			// Check if there's a blank line before this header
			if i > 0 && lines[i-1] == "" {
				foundBlankLineBeforeCerebras = true
			}
		}
	}

	if !foundAnthropicHeader {
		t.Error("Should have '# Anthropic' header comment")
	}
	if !foundCerebrasHeader {
		t.Error("Should have '# Cerebras' header comment")
	}
	if !foundBlankLineBeforeCerebras {
		t.Error("Should have blank line before '# Cerebras' header")
	}
}

// Helper functions for creating pointers.
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func testAPIKeyCredentials(
	environment string,
	header string,
	scheme ProviderCredentialScheme,
) *ProviderCredentials {
	return &ProviderCredentials{
		Fields: []ProviderCredentialField{{
			ID: "api-key", Kind: ProviderCredentialFieldSecret, Required: true,
			Environment: []string{environment},
		}},
		Profiles: []ProviderCredentialProfile{{
			ID: "api-key", Primitive: ProviderAuthenticationAPIKey,
			Fields: []ProviderCredentialFieldID{"api-key"},
			Placements: []ProviderCredentialPlacement{{
				Field: "api-key", Kind: ProviderCredentialPlacementHeader,
				Name: header, Scheme: scheme,
			}},
		}},
		CatalogAcquisition: ProviderCredentialPlane{
			Required: true, Alternatives: []ProviderCredentialProfileID{"api-key"},
		},
		Inference: ProviderCredentialPlane{
			Required: true, Alternatives: []ProviderCredentialProfileID{"api-key"},
		},
	}
}
