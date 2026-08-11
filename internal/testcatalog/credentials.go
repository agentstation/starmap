// Package testcatalog provides canonical catalog fixtures for cross-package tests.
package testcatalog

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// UnauthenticatedCredentials returns an optional, no-authentication contract.
func UnauthenticatedCredentials() *catalogs.ProviderCredentials {
	return &catalogs.ProviderCredentials{
		Profiles: []catalogs.ProviderCredentialProfile{{
			ID: "unauthenticated", Primitive: catalogs.ProviderAuthenticationNone,
		}},
		CatalogAcquisition: catalogs.ProviderCredentialPlane{
			Alternatives: []catalogs.ProviderCredentialProfileID{"unauthenticated"},
		},
		Inference: catalogs.ProviderCredentialPlane{
			Alternatives: []catalogs.ProviderCredentialProfileID{"unauthenticated"},
		},
	}
}

// APIKeyCredentials returns a required API-key contract shared by both planes.
func APIKeyCredentials(
	environment string,
	header string,
	scheme catalogs.ProviderCredentialScheme,
) *catalogs.ProviderCredentials {
	return &catalogs.ProviderCredentials{
		Fields: []catalogs.ProviderCredentialField{{
			ID: "api-key", Kind: catalogs.ProviderCredentialFieldSecret,
			Required: true, Environment: []string{environment},
		}},
		Profiles: []catalogs.ProviderCredentialProfile{{
			ID: "api-key", Primitive: catalogs.ProviderAuthenticationAPIKey,
			Fields: []catalogs.ProviderCredentialFieldID{"api-key"},
			Placements: []catalogs.ProviderCredentialPlacement{{
				Field: "api-key", Kind: catalogs.ProviderCredentialPlacementHeader,
				Name: header, Scheme: scheme,
			}},
		}},
		CatalogAcquisition: catalogs.ProviderCredentialPlane{
			Required: true, Alternatives: []catalogs.ProviderCredentialProfileID{"api-key"},
		},
		Inference: catalogs.ProviderCredentialPlane{
			Required: true, Alternatives: []catalogs.ProviderCredentialProfileID{"api-key"},
		},
	}
}

// QueryAPIKeyCredentials returns a required query-parameter API-key contract.
func QueryAPIKeyCredentials(
	environment string,
	parameter string,
) *catalogs.ProviderCredentials {
	credentials := APIKeyCredentials(
		environment, "Authorization", catalogs.ProviderCredentialSchemeDirect,
	)
	credentials.Profiles[0].Placements[0] = catalogs.ProviderCredentialPlacement{
		Field: "api-key", Kind: catalogs.ProviderCredentialPlacementQuery,
		Name: parameter, Scheme: catalogs.ProviderCredentialSchemeDirect,
		EvidenceURL: "https://ai.google.dev/api/rest/v1beta/models/list",
	}
	return credentials
}

// APIKeyMaterial returns resolved material for an API-key fixture contract.
func APIKeyMaterial(
	credentials *catalogs.ProviderCredentials,
	value string,
) sources.ProviderCredentialMaterial {
	if credentials == nil || len(credentials.Profiles) == 0 {
		return sources.ProviderCredentialMaterial{}
	}
	return sources.NewProviderCredentialMaterial(
		credentials.Profiles[0],
		map[catalogs.ProviderCredentialFieldID]string{"api-key": value},
		sources.ProviderCredentialMetadata{Version: "test"},
	)
}

// OpenAIProtocolOptions returns a valid OpenAI catalog protocol contract.
func OpenAIProtocolOptions() catalogs.ProviderCatalogProtocolOptions {
	return catalogs.ProviderCatalogProtocolOptions{
		OpenAI: &catalogs.ProviderOpenAICatalogProtocolOptions{
			TokenPriceUnit: catalogs.ProviderTokenPriceUnitPerMillion,
		},
	}
}

// AnthropicProtocolOptions returns a valid Anthropic catalog protocol contract.
func AnthropicProtocolOptions() catalogs.ProviderCatalogProtocolOptions {
	return catalogs.ProviderCatalogProtocolOptions{
		Anthropic: &catalogs.ProviderAnthropicCatalogProtocolOptions{Version: "2023-06-01"},
	}
}
