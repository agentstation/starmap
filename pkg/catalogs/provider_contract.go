package catalogs

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// ValidateContract validates serializable catalog-acquisition and inference
// metadata. It does not inspect runtime credential values.
func (p Provider) ValidateContract() error {
	if err := p.validateIdentityAndCredentials(); err != nil {
		return err
	}
	if p.Catalog != nil {
		if !validEndpointType(p.Catalog.Endpoint.Type) {
			return providerContractError(
				"provider.catalog.endpoint.type",
				p.Catalog.Endpoint.Type,
				"is not supported",
			)
		}
		if err := validateCatalogProtocolOptions(p.Catalog.Endpoint); err != nil {
			return err
		}
		if p.Catalog.Endpoint.AuthorMapping != nil {
			if err := p.Catalog.Endpoint.AuthorMapping.Validate(); err != nil {
				return err
			}
		}
		if p.Credentials == nil || len(p.Credentials.CatalogAcquisition.Alternatives) == 0 {
			return providerContractError(
				"provider.credentials.catalog_acquisition.alternatives",
				nil,
				"must declare catalog-acquisition authentication",
			)
		}
		if err := validateEndpointBindingCoverage(
			"provider.credentials.catalog_acquisition",
			p.Catalog.Endpoint.URL,
			p.Credentials.CatalogAcquisition,
			p.Credentials.Profiles,
		); err != nil {
			return err
		}
	}
	if p.Inference == nil {
		return nil
	}
	if p.Credentials != nil {
		if err := validateEndpointBindingCoverage(
			"provider.credentials.inference",
			p.Inference.BaseURL,
			p.Credentials.Inference,
			p.Credentials.Profiles,
		); err != nil {
			return err
		}
	}
	seen := make(map[ProviderOperation]struct{}, len(p.Inference.Endpoints))
	for index, endpoint := range p.Inference.Endpoints {
		if !validProviderOperation(endpoint.Operation) {
			return providerContractError(
				fmt.Sprintf("provider.inference.endpoints[%d].operation", index),
				endpoint.Operation,
				providerOperationMessage(),
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
	if err := validateHealthAPI(p.Inference); err != nil {
		return err
	}
	return nil
}

// validateHealthAPI checks that a declared health API names a known wire
// convention and that a declared kind or component list has a URL to act on.
func validateHealthAPI(inference *ProviderInference) error {
	hasURL := inference.HealthAPIURL != nil && strings.TrimSpace(*inference.HealthAPIURL) != ""
	switch inference.HealthAPIKind {
	case "", HealthAPIKindStatuspage, HealthAPIKindHyperping, HealthAPIKindRSS, HealthAPIKindGoogleCloud:
	default:
		return providerContractError(
			"provider.inference.health_api_kind",
			inference.HealthAPIKind,
			"is not supported",
		)
	}
	if !hasURL {
		if inference.HealthAPIKind != "" {
			return providerContractError(
				"provider.inference.health_api_kind",
				inference.HealthAPIKind,
				"requires health_api_url",
			)
		}
		if len(inference.HealthComponents) > 0 {
			return providerContractError(
				"provider.inference.health_components",
				len(inference.HealthComponents),
				"require health_api_url",
			)
		}
	}
	return nil
}

func validateEndpointBindingCoverage(
	path string,
	template string,
	plane ProviderCredentialPlane,
	profiles []ProviderCredentialProfile,
) error {
	matches := inferenceEndpointVariable.FindAllString(template, -1)
	if len(matches) == 0 {
		return nil
	}
	required := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		required[strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")] = struct{}{}
	}
	indexed := make(map[ProviderCredentialProfileID]ProviderCredentialProfile, len(profiles))
	for _, profile := range profiles {
		indexed[profile.ID] = profile
	}
	for index, profileID := range plane.Alternatives {
		profile, exists := indexed[profileID]
		if !exists {
			continue // The credential schema reports the unknown profile first.
		}
		bound := make(map[string]struct{}, len(profile.EndpointBindings))
		for _, binding := range profile.EndpointBindings {
			bound[binding.Variable] = struct{}{}
		}
		for variable := range required {
			if _, exists := bound[variable]; !exists {
				return providerContractError(
					fmt.Sprintf("%s.alternatives[%d]", path, index),
					profileID,
					fmt.Sprintf("profile does not bind endpoint variable %q", variable),
				)
			}
		}
	}
	return nil
}

func (p Provider) validateIdentityAndCredentials() error {
	if strings.TrimSpace(string(p.ID)) == "" {
		return providerContractError("provider.id", p.ID, "is required")
	}
	if !validCredentialIdentifier(string(p.ID)) {
		return providerContractError("provider.id", p.ID, "must be a lowercase kebab-case ID")
	}
	for index, alias := range p.Aliases {
		if !validCredentialIdentifier(string(alias)) {
			return providerContractError(
				fmt.Sprintf("provider.aliases[%d]", index),
				alias,
				"must be a lowercase kebab-case ID",
			)
		}
	}
	if strings.TrimSpace(p.Name) == "" {
		return providerContractError("provider.name", p.Name, "is required")
	}
	if err := p.Credentials.validate(); err != nil {
		return err
	}
	return nil
}

func validateCatalogProtocolOptions(endpoint ProviderEndpoint) error {
	options := endpoint.ProtocolOptions
	switch endpoint.Type {
	case EndpointTypeOpenAI:
		if options.OpenAI == nil || options.Anthropic != nil {
			return providerContractError(
				"provider.catalog.endpoint.protocol_options",
				options,
				"openai endpoints require only openai protocol options",
			)
		}
		if options.OpenAI.TokenPriceUnit != ProviderTokenPriceUnitPerToken &&
			options.OpenAI.TokenPriceUnit != ProviderTokenPriceUnitPerMillion {
			return providerContractError(
				"provider.catalog.endpoint.protocol_options.openai.token_price_unit",
				options.OpenAI.TokenPriceUnit,
				"must be usd-per-token or usd-per-million-tokens",
			)
		}
	case EndpointTypeAnthropic:
		if options.Anthropic == nil || options.OpenAI != nil {
			return providerContractError(
				"provider.catalog.endpoint.protocol_options",
				options,
				"anthropic endpoints require only anthropic protocol options",
			)
		}
		if strings.TrimSpace(options.Anthropic.Version) == "" {
			return providerContractError(
				"provider.catalog.endpoint.protocol_options.anthropic.version",
				options.Anthropic.Version,
				"is required",
			)
		}
	case EndpointTypeOllama, EndpointTypeCohere, EndpointTypeVoyage:
		return providerContractError(
			"provider.catalog.endpoint.type",
			endpoint.Type,
			"has no compiled catalog-acquisition transport",
		)
	default:
		if options.OpenAI != nil || options.Anthropic != nil {
			return providerContractError(
				"provider.catalog.endpoint.protocol_options",
				options,
				"must match the endpoint type",
			)
		}
	}
	return nil
}

func validEndpointType(endpointType EndpointType) bool {
	switch endpointType {
	case EndpointTypeOpenAI,
		EndpointTypeAnthropic,
		EndpointTypeGoogle,
		EndpointTypeGoogleCloud,
		EndpointTypeOllama,
		EndpointTypeCohere,
		EndpointTypeVoyage:
		return true
	default:
		return false
	}
}

func providerContractError(field string, value any, message string) error {
	return &errors.ValidationError{Field: field, Value: value, Message: message}
}
