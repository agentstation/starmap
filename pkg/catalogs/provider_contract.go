package catalogs

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// ValidateContract validates serializable catalog-acquisition and inference
// metadata. It does not inspect runtime credential values.
func (p Provider) ValidateContract() error {
	if strings.TrimSpace(string(p.ID)) == "" {
		return providerContractError("provider.id", p.ID, "is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return providerContractError("provider.name", p.Name, "is required")
	}
	if p.Catalog != nil {
		if !validProviderCatalogAuthMethod(p.Catalog.Auth.Method) {
			return providerContractError(
				"provider.catalog.auth.method",
				p.Catalog.Auth.Method,
				"must be none, api-key, google-default, azure-default, or aws-default",
			)
		}
		if p.Catalog.Auth.Method == ProviderCatalogAuthNone && p.Catalog.Auth.Required {
			return providerContractError(
				"provider.catalog.auth.required",
				true,
				"cannot require the none authentication method",
			)
		}
		if p.Catalog.Auth.Method == ProviderCatalogAuthAPIKey && p.APIKey == nil {
			return providerContractError(
				"provider.api_key",
				nil,
				"is required for api-key catalog authentication",
			)
		}
		if !validEndpointType(p.Catalog.Endpoint.Type) {
			return providerContractError(
				"provider.catalog.endpoint.type",
				p.Catalog.Endpoint.Type,
				"is not supported",
			)
		}
		for index, scope := range p.Catalog.Auth.Scopes {
			if strings.TrimSpace(scope) == "" {
				return providerContractError(
					fmt.Sprintf("provider.catalog.auth.scopes[%d]", index),
					scope,
					"must not be empty",
				)
			}
		}
	}
	if p.Inference == nil {
		return nil
	}
	seen := make(map[ProviderOperation]struct{}, len(p.Inference.Endpoints))
	for index, endpoint := range p.Inference.Endpoints {
		if !validProviderOperation(endpoint.Operation) {
			return providerContractError(
				fmt.Sprintf("provider.inference.endpoints[%d].operation", index),
				endpoint.Operation,
				"must be chat-completions or embeddings",
			)
		}
		if !validEndpointType(endpoint.Type) {
			return providerContractError(
				fmt.Sprintf("provider.inference.endpoints[%d].type", index),
				endpoint.Type,
				"is not supported",
			)
		}
		if !strings.HasPrefix(endpoint.Path, "/") {
			return providerContractError(
				fmt.Sprintf("provider.inference.endpoints[%d].path", index),
				endpoint.Path,
				"must start with /",
			)
		}
		if endpoint.StreamPath != "" && !strings.HasPrefix(endpoint.StreamPath, "/") {
			return providerContractError(
				fmt.Sprintf("provider.inference.endpoints[%d].stream_path", index),
				endpoint.StreamPath,
				"must start with /",
			)
		}
		if _, exists := seen[endpoint.Operation]; exists {
			return providerContractError(
				fmt.Sprintf("provider.inference.endpoints[%d].operation", index),
				endpoint.Operation,
				"must be unique",
			)
		}
		for authorID, endpointType := range endpoint.ProtocolsByAuthor {
			if strings.TrimSpace(string(authorID)) == "" {
				return providerContractError(
					fmt.Sprintf("provider.inference.endpoints[%d].protocols_by_author", index),
					authorID,
					"author ID must not be empty",
				)
			}
			if !validEndpointType(endpointType) {
				return providerContractError(
					fmt.Sprintf("provider.inference.endpoints[%d].protocols_by_author.%s", index, authorID),
					endpointType,
					"is not supported",
				)
			}
		}
		for field, paths := range map[string]map[AuthorID]string{
			"paths_by_author":        endpoint.PathsByAuthor,
			"stream_paths_by_author": endpoint.StreamPathsByAuthor,
		} {
			for authorID, path := range paths {
				if strings.TrimSpace(string(authorID)) == "" {
					return providerContractError(
						fmt.Sprintf("provider.inference.endpoints[%d].%s", index, field),
						authorID,
						"author ID must not be empty",
					)
				}
				if !strings.HasPrefix(path, "/") {
					return providerContractError(
						fmt.Sprintf("provider.inference.endpoints[%d].%s.%s", index, field, authorID),
						path,
						"must start with /",
					)
				}
			}
		}
		seen[endpoint.Operation] = struct{}{}
	}
	return nil
}

func validProviderCatalogAuthMethod(method ProviderCatalogAuthMethod) bool {
	switch method {
	case ProviderCatalogAuthNone,
		ProviderCatalogAuthAPIKey,
		ProviderCatalogAuthGoogleDefault,
		ProviderCatalogAuthAzureDefault,
		ProviderCatalogAuthAWSDefault:
		return true
	default:
		return false
	}
}

func validEndpointType(endpointType EndpointType) bool {
	switch endpointType {
	case EndpointTypeOpenAI,
		EndpointTypeAnthropic,
		EndpointTypeGoogle,
		EndpointTypeGoogleCloud,
		EndpointTypeOllama:
		return true
	default:
		return false
	}
}

func providerContractError(field string, value any, message string) error {
	return &errors.ValidationError{Field: field, Value: value, Message: message}
}
