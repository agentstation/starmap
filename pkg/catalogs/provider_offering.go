package catalogs

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// ModelDefinitionID identifies one provider-independent model definition.
type ModelDefinitionID string

// ProviderModelID is the exact opaque model identifier accepted by a provider.
type ProviderModelID string

// OfferingKey is the globally unique identity of a provider model offering.
type OfferingKey struct {
	ProviderID      ProviderID      `json:"provider_id" yaml:"provider_id"`
	ProviderModelID ProviderModelID `json:"provider_model_id" yaml:"provider_model_id"`
	DeploymentID    string          `json:"deployment_id,omitempty" yaml:"deployment_id,omitempty"`
}

// OfferingAvailability describes whether an offering can currently be used.
type OfferingAvailability string

const (
	// OfferingAvailabilityAvailable means the offering is generally available.
	OfferingAvailabilityAvailable OfferingAvailability = "available"
	// OfferingAvailabilityRestricted means access depends on region, account, or allowlisting.
	OfferingAvailabilityRestricted OfferingAvailability = "restricted"
	// OfferingAvailabilityUnavailable means the provider does not currently serve the offering.
	OfferingAvailabilityUnavailable OfferingAvailability = "unavailable"
)

// OfferingLifecycle describes the provider-specific lifecycle of an offering.
type OfferingLifecycle string

const (
	// OfferingLifecycleActive means the offering is supported for production use.
	OfferingLifecycleActive OfferingLifecycle = "active"
	// OfferingLifecyclePreview means the offering is preview or beta quality.
	OfferingLifecyclePreview OfferingLifecycle = "preview"
	// OfferingLifecycleDeprecated means callers should migrate away from the offering.
	OfferingLifecycleDeprecated OfferingLifecycle = "deprecated"
	// OfferingLifecycleRetired means the provider no longer accepts new requests.
	OfferingLifecycleRetired OfferingLifecycle = "retired"
)

// OfferingAccessChannel identifies how a consumer reaches an offering.
type OfferingAccessChannel string

const (
	// OfferingAccessChannelServerToServer is a supported machine invocation contract.
	OfferingAccessChannelServerToServer OfferingAccessChannel = "server_to_server"
	// OfferingAccessChannelApplication is access inside a provider application only.
	OfferingAccessChannelApplication OfferingAccessChannel = "application_only"
)

// OfferingRoutability states whether Starport may select an offering.
type OfferingRoutability string

const (
	// OfferingRoutabilityRoutable is eligible for provider routing.
	OfferingRoutabilityRoutable OfferingRoutability = "routable"
	// OfferingRoutabilityDiscoverable exposes catalog facts without an invocation contract.
	OfferingRoutabilityDiscoverable OfferingRoutability = "discoverable_only"
)

// InvocationAPI identifies one provider-supported invocation contract.
type InvocationAPI string

const (
	// InvocationAPIChatCompletions is the OpenAI chat-completions contract.
	InvocationAPIChatCompletions InvocationAPI = "chat_completions"
	// InvocationAPICompletions is the OpenAI text-completions contract still used by base models.
	InvocationAPICompletions InvocationAPI = "completions"
	// InvocationAPIResponses is the OpenAI responses contract.
	InvocationAPIResponses InvocationAPI = "responses"
	// InvocationAPIMessages is the Anthropic messages contract.
	InvocationAPIMessages InvocationAPI = "messages"
	// InvocationAPIEmbeddings is an embeddings contract.
	InvocationAPIEmbeddings InvocationAPI = "embeddings"
	// InvocationAPIImageGeneration is an image-generation contract.
	InvocationAPIImageGeneration InvocationAPI = "image_generation"
	// InvocationAPIAudio is an audio invocation contract.
	InvocationAPIAudio InvocationAPI = "audio"
	// InvocationAPIRerank is a reranking contract.
	InvocationAPIRerank InvocationAPI = "rerank"
	// InvocationAPIBedrockConverse is the native Amazon Bedrock Converse contract.
	InvocationAPIBedrockConverse InvocationAPI = "bedrock_converse"
	// InvocationAPIBedrockInvokeModel is the native Amazon Bedrock InvokeModel contract.
	InvocationAPIBedrockInvokeModel InvocationAPI = "bedrock_invoke_model"
	// InvocationAPISnowflakeComplete is the Snowflake Cortex REST and SQL completion contract.
	InvocationAPISnowflakeComplete InvocationAPI = "snowflake_complete"
	// InvocationAPIWatsonxGenerate is IBM watsonx.ai text generation.
	InvocationAPIWatsonxGenerate InvocationAPI = "watsonx_generate"
	// InvocationAPIOCIInference is the native OCI Generative AI inference contract.
	InvocationAPIOCIInference InvocationAPI = "oci_inference"
)

// OfferingAccess is the routing-safe access contract for an offering.
type OfferingAccess struct {
	Channel     OfferingAccessChannel `json:"channel" yaml:"channel"`
	Routability OfferingRoutability   `json:"routability" yaml:"routability"`
	APIs        []InvocationAPI       `json:"apis,omitempty" yaml:"apis,omitempty"`
}

// GeographicBoundaryKind classifies a residency or sovereignty boundary.
type GeographicBoundaryKind string

const (
	// GeographicBoundaryGlobal permits provider-documented global processing.
	GeographicBoundaryGlobal GeographicBoundaryKind = "global"
	// GeographicBoundaryGeography is a multi-country named geography.
	GeographicBoundaryGeography GeographicBoundaryKind = "geography"
	// GeographicBoundaryCountry limits processing to one or more countries.
	GeographicBoundaryCountry GeographicBoundaryKind = "country"
	// GeographicBoundarySovereign is a sovereign cloud or realm boundary.
	GeographicBoundarySovereign GeographicBoundaryKind = "sovereign"
)

// GeographicBoundary is one provider-declared data-residency boundary.
type GeographicBoundary struct {
	ID        string                 `json:"id" yaml:"id"`
	Kind      GeographicBoundaryKind `json:"kind" yaml:"kind"`
	Countries []string               `json:"countries,omitempty" yaml:"countries,omitempty"`
}

// CloudRegion is one provider-scoped deployment region.
type CloudRegion struct {
	ID          string              `json:"id" yaml:"id"`
	Realm       string              `json:"realm,omitempty" yaml:"realm,omitempty"`
	Residency   *GeographicBoundary `json:"residency,omitempty" yaml:"residency,omitempty"`
	Destination bool                `json:"destination,omitempty" yaml:"destination,omitempty"`
}

// ProviderDeployment identifies a provider deployment type and service tier.
type ProviderDeployment struct {
	Type string `json:"type" yaml:"type"`
	Tier string `json:"tier,omitempty" yaml:"tier,omitempty"`
}

// CrossRegionInferenceProfile describes provider-controlled regional routing.
type CrossRegionInferenceProfile struct {
	ID                 string   `json:"id" yaml:"id"`
	Scope              string   `json:"scope" yaml:"scope"`
	SourceRegions      []string `json:"source_regions,omitempty" yaml:"source_regions,omitempty"`
	DestinationRegions []string `json:"destination_regions" yaml:"destination_regions"`
}

// AggregatorUpstream identifies the underlying provider offering when known.
type AggregatorUpstream struct {
	ProviderID      ProviderID      `json:"provider_id" yaml:"provider_id"`
	ProviderModelID ProviderModelID `json:"provider_model_id" yaml:"provider_model_id"`
}

// ProviderOfferingEndpoint describes provider-specific inference endpoint behavior.
type ProviderOfferingEndpoint struct {
	Type    EndpointType `json:"type,omitempty" yaml:"type,omitempty"`
	BaseURL string       `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	Path    string       `json:"path,omitempty" yaml:"path,omitempty"`
}

// OfferingRequestHeaders is a typed set of provider request header overrides.
type OfferingRequestHeaders map[string]string

// OfferingRequestBody is a typed set of exact JSON request-body values.
// RawMessage preserves booleans, numbers, strings, arrays, objects, and null
// without routing values through map[string]any.
type OfferingRequestBody map[string]json.RawMessage

// ProviderRequestOverrides contains provider-specific inference request changes.
type ProviderRequestOverrides struct {
	Headers OfferingRequestHeaders `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    OfferingRequestBody    `json:"body,omitempty" yaml:"body,omitempty"`
}

// ProviderOfferingMode describes one named service mode for an offering.
type ProviderOfferingMode struct {
	Pricing    *ModelPricing            `json:"pricing,omitempty" yaml:"pricing,omitempty"`
	Limits     *ModelLimits             `json:"limits,omitempty" yaml:"limits,omitempty"`
	Deployment *ProviderDeployment      `json:"deployment,omitempty" yaml:"deployment,omitempty"`
	Request    ProviderRequestOverrides `json:"request" yaml:"request,omitempty"`
}

// ProviderOffering is one provider's service contract for a model definition.
// Provider-specific price, limits, availability, regions, lifecycle, endpoint,
// modes, and request overrides live here rather than on the definition.
type ProviderOffering struct {
	ProviderID         ProviderID                      `json:"provider_id" yaml:"provider_id"`
	ProviderModelID    ProviderModelID                 `json:"provider_model_id" yaml:"provider_model_id"`
	DeploymentID       string                          `json:"deployment_id,omitempty" yaml:"deployment_id,omitempty"`
	DefinitionID       ModelDefinitionID               `json:"definition_id" yaml:"definition_id"`
	Aliases            []string                        `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Pricing            *ModelPricing                   `json:"pricing,omitempty" yaml:"pricing,omitempty"`
	Limits             *ModelLimits                    `json:"limits,omitempty" yaml:"limits,omitempty"`
	Availability       OfferingAvailability            `json:"availability" yaml:"availability"`
	Access             OfferingAccess                  `json:"access" yaml:"access"`
	Regions            []CloudRegion                   `json:"regions,omitempty" yaml:"regions,omitempty"`
	Deployment         ProviderDeployment              `json:"deployment" yaml:"deployment"`
	InferenceProfile   *CrossRegionInferenceProfile    `json:"inference_profile,omitempty" yaml:"inference_profile,omitempty"`
	AggregatorUpstream *AggregatorUpstream             `json:"aggregator_upstream,omitempty" yaml:"aggregator_upstream,omitempty"`
	Endpoint           ProviderOfferingEndpoint        `json:"endpoint" yaml:"endpoint,omitempty"`
	Lifecycle          OfferingLifecycle               `json:"lifecycle" yaml:"lifecycle"`
	Modes              map[string]ProviderOfferingMode `json:"modes,omitempty" yaml:"modes,omitempty"`
}

// Key returns the provider-scoped immutable offering identity.
func (o ProviderOffering) Key() OfferingKey {
	return OfferingKey{ProviderID: o.ProviderID, ProviderModelID: o.ProviderModelID, DeploymentID: o.DeploymentID}
}

// Validate verifies required identity and provider-specific fields.
func (o ProviderOffering) Validate() error {
	for _, required := range []struct {
		field string
		value string
	}{
		{field: "provider_id", value: string(o.ProviderID)},
		{field: "provider_model_id", value: string(o.ProviderModelID)},
		{field: "definition_id", value: string(o.DefinitionID)},
	} {
		if strings.TrimSpace(required.value) == "" {
			return offeringValidationError(required.field, required.value, validationMessageIsRequired)
		}
	}
	if !validOfferingAvailability(o.Availability) {
		return offeringValidationError("availability", o.Availability, "must be available, restricted, or unavailable")
	}
	if !validOfferingLifecycle(o.Lifecycle) {
		return offeringValidationError(catalogFieldLifecycle, o.Lifecycle, "must be active, preview, deprecated, or retired")
	}
	if err := validateOfferingAccess(o.Access, o.Endpoint); err != nil {
		return err
	}
	seenRegions := make(map[string]struct{}, len(o.Regions))
	for index, region := range o.Regions {
		if strings.TrimSpace(region.ID) == "" {
			return offeringValidationError(fmt.Sprintf("regions[%d]", index), region, validationMessageMustNotBeEmpty)
		}
		if _, exists := seenRegions[region.ID]; exists {
			return offeringValidationError(fmt.Sprintf("regions[%d]", index), region, validationMessageMustBeUnique)
		}
		seenRegions[region.ID] = struct{}{}
		if region.Residency != nil {
			if err := validateGeographicBoundary(*region.Residency, fmt.Sprintf("regions[%d].residency", index)); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(o.Deployment.Type) == "" {
		return offeringValidationError("deployment.type", o.Deployment.Type, validationMessageIsRequired)
	}
	seenAliases := make(map[string]struct{}, len(o.Aliases))
	for index, alias := range o.Aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			return offeringValidationError(fmt.Sprintf("aliases[%d]", index), alias, validationMessageMustNotBeEmpty)
		}
		if _, found := seenAliases[alias]; found {
			return offeringValidationError(fmt.Sprintf("aliases[%d]", index), alias, validationMessageMustBeUnique)
		}
		seenAliases[alias] = struct{}{}
	}
	if o.InferenceProfile != nil {
		if err := validateInferenceProfile(*o.InferenceProfile); err != nil {
			return err
		}
	}
	if o.AggregatorUpstream != nil {
		if strings.TrimSpace(string(o.AggregatorUpstream.ProviderID)) == "" || strings.TrimSpace(string(o.AggregatorUpstream.ProviderModelID)) == "" {
			return offeringValidationError("aggregator_upstream", o.AggregatorUpstream, "requires provider_id and provider_model_id")
		}
	}
	for modeName, mode := range o.Modes {
		if strings.TrimSpace(modeName) == "" {
			return offeringValidationError("modes", modeName, "mode name must not be empty")
		}
		if mode.Deployment != nil && strings.TrimSpace(mode.Deployment.Type) == "" {
			return offeringValidationError("modes."+modeName+".deployment.type", mode.Deployment.Type, validationMessageMustNotBeEmpty)
		}
		for header := range mode.Request.Headers {
			if strings.TrimSpace(header) == "" {
				return offeringValidationError("modes."+modeName+".request.headers", header, "header name must not be empty")
			}
		}
		for field, value := range mode.Request.Body {
			if strings.TrimSpace(field) == "" {
				return offeringValidationError("modes."+modeName+".request.body", field, "field name must not be empty")
			}
			if !json.Valid(value) {
				return offeringValidationError("modes."+modeName+".request.body."+field, string(value), "must contain one valid JSON value")
			}
		}
	}
	return nil
}

// IsRoutable reports whether Starport may select this offering.
func (o ProviderOffering) IsRoutable() bool {
	return o.Access.Channel == OfferingAccessChannelServerToServer &&
		o.Access.Routability == OfferingRoutabilityRoutable && len(o.Access.APIs) > 0
}

func validateOfferingAccess(access OfferingAccess, endpoint ProviderOfferingEndpoint) error {
	switch access.Channel {
	case OfferingAccessChannelServerToServer:
		if access.Routability == OfferingRoutabilityRoutable {
			if len(access.APIs) == 0 {
				return offeringValidationError("access.apis", access.APIs, "routable offerings require an invocation API")
			}
			if endpoint.Type == "" {
				return offeringValidationError("endpoint.type", endpoint.Type, "routable offerings require an endpoint contract")
			}
		} else if access.Routability != OfferingRoutabilityDiscoverable {
			return offeringValidationError("access.routability", access.Routability, validationMessageIsNotSupported)
		}
	case OfferingAccessChannelApplication:
		if access.Routability != OfferingRoutabilityDiscoverable || len(access.APIs) != 0 {
			return offeringValidationError("access", access, "application-only offerings must be discoverable-only with no invocation APIs")
		}
	default:
		return offeringValidationError("access.channel", access.Channel, validationMessageIsNotSupported)
	}
	seen := make(map[InvocationAPI]struct{}, len(access.APIs))
	for index, api := range access.APIs {
		if strings.TrimSpace(string(api)) == "" {
			return offeringValidationError(fmt.Sprintf("access.apis[%d]", index), api, validationMessageMustNotBeEmpty)
		}
		if _, found := seen[api]; found {
			return offeringValidationError(fmt.Sprintf("access.apis[%d]", index), api, validationMessageMustBeUnique)
		}
		seen[api] = struct{}{}
	}
	return nil
}

func validateGeographicBoundary(boundary GeographicBoundary, field string) error {
	if strings.TrimSpace(boundary.ID) == "" {
		return offeringValidationError(field+".id", boundary.ID, validationMessageIsRequired)
	}
	switch boundary.Kind {
	case GeographicBoundaryGlobal, GeographicBoundaryGeography, GeographicBoundaryCountry, GeographicBoundarySovereign:
	default:
		return offeringValidationError(field+".kind", boundary.Kind, validationMessageIsNotSupported)
	}
	return nil
}

func validateInferenceProfile(profile CrossRegionInferenceProfile) error {
	if strings.TrimSpace(profile.ID) == "" || strings.TrimSpace(profile.Scope) == "" {
		return offeringValidationError("inference_profile", profile, "requires id and scope")
	}
	if len(profile.DestinationRegions) == 0 {
		return offeringValidationError("inference_profile.destination_regions", profile.DestinationRegions, "requires at least one destination region")
	}
	for index, region := range append(append([]string(nil), profile.SourceRegions...), profile.DestinationRegions...) {
		if strings.TrimSpace(region) == "" {
			return offeringValidationError(fmt.Sprintf("inference_profile.regions[%d]", index), region, validationMessageMustNotBeEmpty)
		}
	}
	return nil
}

func validOfferingAvailability(value OfferingAvailability) bool {
	switch value {
	case OfferingAvailabilityAvailable, OfferingAvailabilityRestricted, OfferingAvailabilityUnavailable:
		return true
	default:
		return false
	}
}

func validOfferingLifecycle(value OfferingLifecycle) bool {
	switch value {
	case OfferingLifecycleActive, OfferingLifecyclePreview, OfferingLifecycleDeprecated, OfferingLifecycleRetired:
		return true
	default:
		return false
	}
}

func offeringValidationError(field string, value any, message string) error {
	return &errors.ValidationError{Field: field, Value: value, Message: message}
}

func copyProviderOffering(offering ProviderOffering) ProviderOffering {
	copyOffering := offering
	copyOffering.Pricing = deepCopyModelPricing(offering.Pricing)
	if offering.Limits != nil {
		limits := *offering.Limits
		copyOffering.Limits = &limits
	}
	copyOffering.Access.APIs = append([]InvocationAPI(nil), offering.Access.APIs...)
	copyOffering.Aliases = append([]string(nil), offering.Aliases...)
	copyOffering.Regions = append([]CloudRegion(nil), offering.Regions...)
	for index := range copyOffering.Regions {
		if offering.Regions[index].Residency != nil {
			residency := *offering.Regions[index].Residency
			residency.Countries = append([]string(nil), residency.Countries...)
			copyOffering.Regions[index].Residency = &residency
		}
	}
	if offering.InferenceProfile != nil {
		profile := *offering.InferenceProfile
		profile.SourceRegions = append([]string(nil), profile.SourceRegions...)
		profile.DestinationRegions = append([]string(nil), profile.DestinationRegions...)
		copyOffering.InferenceProfile = &profile
	}
	if offering.AggregatorUpstream != nil {
		upstream := *offering.AggregatorUpstream
		copyOffering.AggregatorUpstream = &upstream
	}
	if offering.Modes != nil {
		copyOffering.Modes = make(map[string]ProviderOfferingMode, len(offering.Modes))
		for name, mode := range offering.Modes {
			copyMode := mode
			copyMode.Pricing = deepCopyModelPricing(mode.Pricing)
			if mode.Deployment != nil {
				deployment := *mode.Deployment
				copyMode.Deployment = &deployment
			}
			if mode.Limits != nil {
				limits := *mode.Limits
				copyMode.Limits = &limits
			}
			copyMode.Request.Headers = make(OfferingRequestHeaders, len(mode.Request.Headers))
			maps.Copy(copyMode.Request.Headers, mode.Request.Headers)
			copyMode.Request.Body = make(OfferingRequestBody, len(mode.Request.Body))
			for field, value := range mode.Request.Body {
				copyMode.Request.Body[field] = append(json.RawMessage(nil), value...)
			}
			copyOffering.Modes[name] = copyMode
		}
	}
	return copyOffering
}
