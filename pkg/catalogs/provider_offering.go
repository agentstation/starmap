package catalogs

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/goccy/go-yaml"

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
}

// OfferingAvailability describes whether an offering can currently be used.
type OfferingAvailability string

const (
	// OfferingAvailabilityUnknown means no source supplied current availability.
	OfferingAvailabilityUnknown OfferingAvailability = "unknown"
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
	// OfferingLifecycleUnknown means no source supplied a lifecycle state.
	OfferingLifecycleUnknown OfferingLifecycle = "unknown"
	// OfferingLifecycleActive means the offering is supported for production use.
	OfferingLifecycleActive OfferingLifecycle = "active"
	// OfferingLifecyclePreview means the offering is preview or beta quality.
	OfferingLifecyclePreview OfferingLifecycle = "preview"
	// OfferingLifecycleDeprecated means callers should migrate away from the offering.
	OfferingLifecycleDeprecated OfferingLifecycle = "deprecated"
	// OfferingLifecycleRetired means the provider no longer accepts new requests.
	OfferingLifecycleRetired OfferingLifecycle = "retired"
)

// ProviderOfferingEndpoint describes provider-specific inference endpoint behavior.
type ProviderOfferingEndpoint struct {
	Type EndpointType `json:"type,omitempty" yaml:"type,omitempty"`
	URL  string       `json:"url,omitempty" yaml:"url,omitempty"`
}

// OfferingRequestHeaders is a typed set of provider request header overrides.
type OfferingRequestHeaders map[string]string

// OfferingRequestBody is a typed set of exact JSON request-body values.
// RawMessage preserves booleans, numbers, strings, arrays, objects, and null
// without routing values through map[string]any.
type OfferingRequestBody map[string]json.RawMessage

// MarshalYAML preserves each exact JSON value as its equivalent native YAML
// scalar, sequence, mapping, or null instead of encoding RawMessage bytes as a
// sequence of integers.
func (b OfferingRequestBody) MarshalYAML() (any, error) {
	values := make(map[string]yaml.RawMessage, len(b))
	for field, value := range b {
		encoded, err := yaml.JSONToYAML(value)
		if err != nil {
			return nil, errors.WrapParse("json", "request.body."+field, err)
		}
		values[field] = yaml.RawMessage(encoded)
	}
	return values, nil
}

// UnmarshalYAML restores native YAML values as exact JSON values.
func (b *OfferingRequestBody) UnmarshalYAML(data []byte) error {
	encoded, err := yaml.YAMLToJSON(data)
	if err != nil {
		return errors.WrapParse("yaml", "request.body", err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &values); err != nil {
		return errors.WrapParse("json", "request.body", err)
	}
	*b = values
	return nil
}

// ProviderRequestOverrides contains provider-specific inference request changes.
type ProviderRequestOverrides struct {
	Headers OfferingRequestHeaders `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    OfferingRequestBody    `json:"body,omitempty" yaml:"body,omitempty"`
}

// ProviderOfferingMode describes one named service mode for an offering.
type ProviderOfferingMode struct {
	Pricing *ModelPricing            `json:"pricing,omitempty" yaml:"pricing,omitempty"`
	Request ProviderRequestOverrides `json:"request" yaml:"request,omitempty"`
}

// ProviderOffering is one provider's service contract for a model definition.
// Provider-specific price, limits, availability, regions, lifecycle, endpoint,
// modes, and request overrides live here rather than on the definition.
type ProviderOffering struct {
	ProviderID      ProviderID                      `json:"provider_id" yaml:"provider_id"`
	ProviderModelID ProviderModelID                 `json:"provider_model_id" yaml:"provider_model_id"`
	DefinitionID    ModelDefinitionID               `json:"definition_id" yaml:"definition_id"`
	Pricing         *ModelPricing                   `json:"pricing,omitempty" yaml:"pricing,omitempty"`
	Limits          *ModelLimits                    `json:"limits,omitempty" yaml:"limits,omitempty"`
	Availability    OfferingAvailability            `json:"availability" yaml:"availability"`
	Regions         []string                        `json:"regions,omitempty" yaml:"regions,omitempty"`
	Endpoint        ProviderOfferingEndpoint        `json:"endpoint" yaml:"endpoint,omitempty"`
	Lifecycle       OfferingLifecycle               `json:"lifecycle" yaml:"lifecycle"`
	Modes           map[string]ProviderOfferingMode `json:"modes,omitempty" yaml:"modes,omitempty"`
}

// Key returns the provider-scoped immutable offering identity.
func (o ProviderOffering) Key() OfferingKey {
	return OfferingKey{ProviderID: o.ProviderID, ProviderModelID: o.ProviderModelID}
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
			return offeringValidationError(required.field, required.value, "is required")
		}
	}
	if !validOfferingAvailability(o.Availability) {
		return offeringValidationError("availability", o.Availability, "must be unknown, available, restricted, or unavailable")
	}
	if !validOfferingLifecycle(o.Lifecycle) {
		return offeringValidationError("lifecycle", o.Lifecycle, "must be unknown, active, preview, deprecated, or retired")
	}
	seenRegions := make(map[string]struct{}, len(o.Regions))
	for index, region := range o.Regions {
		if strings.TrimSpace(region) == "" {
			return offeringValidationError(fmt.Sprintf("regions[%d]", index), region, "must not be empty")
		}
		if _, exists := seenRegions[region]; exists {
			return offeringValidationError(fmt.Sprintf("regions[%d]", index), region, "must be unique")
		}
		seenRegions[region] = struct{}{}
	}
	for modeName, mode := range o.Modes {
		if strings.TrimSpace(modeName) == "" {
			return offeringValidationError("modes", modeName, "mode name must not be empty")
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

func validOfferingAvailability(value OfferingAvailability) bool {
	switch value {
	case OfferingAvailabilityUnknown, OfferingAvailabilityAvailable, OfferingAvailabilityRestricted, OfferingAvailabilityUnavailable:
		return true
	default:
		return false
	}
}

func validOfferingLifecycle(value OfferingLifecycle) bool {
	switch value {
	case OfferingLifecycleUnknown, OfferingLifecycleActive, OfferingLifecyclePreview, OfferingLifecycleDeprecated, OfferingLifecycleRetired:
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
	copyOffering.Regions = append([]string(nil), offering.Regions...)
	if offering.Modes != nil {
		copyOffering.Modes = make(map[string]ProviderOfferingMode, len(offering.Modes))
		for name, mode := range offering.Modes {
			copyMode := mode
			copyMode.Pricing = deepCopyModelPricing(mode.Pricing)
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
