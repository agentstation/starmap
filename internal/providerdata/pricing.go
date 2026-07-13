// Package providerdata loads typed, non-secret provider facts from the embedded
// catalog. It keeps mutable commercial facts in catalog YAML rather than Go.
package providerdata

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Evidence identifies the non-duplicating authority for a provider fact file.
type Evidence struct {
	URL        string    `json:"url" yaml:"url"`
	Revision   string    `json:"revision" yaml:"revision"`
	ObservedAt time.Time `json:"observed_at" yaml:"observed_at"`
	Scope      string    `json:"scope" yaml:"scope"`
}

// PricingFacts contains canonical pricing and named provider modes for one
// exact provider model ID.
type PricingFacts struct {
	Pricing *catalogs.ModelPricing        `json:"pricing,omitempty" yaml:"pricing,omitempty"`
	Modes   map[string]catalogs.ModelMode `json:"modes,omitempty" yaml:"modes,omitempty"`
}

// PricingCatalog is a provider-scoped catalog of canonical commercial facts.
type PricingCatalog struct {
	ProviderID catalogs.ProviderID     `json:"provider_id" yaml:"provider_id"`
	Evidence   Evidence                `json:"evidence" yaml:"evidence"`
	Models     map[string]PricingFacts `json:"models" yaml:"models"`
}

// LoadPricingCatalog reads and validates one provider's embedded pricing facts.
func LoadPricingCatalog(providerID catalogs.ProviderID) (PricingCatalog, error) {
	path := fmt.Sprintf("catalog/providers/%s/pricing.yaml", providerID)
	payload, err := embedded.FS.ReadFile(path)
	if err != nil {
		return PricingCatalog{}, errors.WrapIO("read", path, err)
	}
	return ParsePricingCatalog(providerID, payload)
}

// ParsePricingCatalog decodes and validates one exact provider fact document.
func ParsePricingCatalog(providerID catalogs.ProviderID, payload []byte) (PricingCatalog, error) {
	var catalog PricingCatalog
	if err := yaml.Unmarshal(payload, &catalog); err != nil {
		return PricingCatalog{}, errors.WrapParse("yaml", "provider pricing catalog", err)
	}
	if catalog.ProviderID != providerID {
		return PricingCatalog{}, &errors.ValidationError{Field: "provider_id", Value: catalog.ProviderID, Message: "must match the requested provider"}
	}
	if err := validateEvidence(catalog.Evidence); err != nil {
		return PricingCatalog{}, err
	}
	if len(catalog.Models) == 0 {
		return PricingCatalog{}, &errors.ValidationError{Field: "models", Message: "must not be empty"}
	}
	for modelID, facts := range catalog.Models {
		if strings.TrimSpace(modelID) == "" {
			return PricingCatalog{}, &errors.ValidationError{Field: "models", Message: "model IDs must not be empty"}
		}
		if facts.Pricing == nil && len(facts.Modes) == 0 {
			return PricingCatalog{}, &errors.ValidationError{Field: "models." + modelID, Message: "must contain pricing or a named mode"}
		}
		if facts.Pricing != nil {
			if err := facts.Pricing.Validate(); err != nil {
				return PricingCatalog{}, errors.WrapResource("validate", "provider pricing", modelID, err)
			}
		}
		for modeName, mode := range facts.Modes {
			if strings.TrimSpace(modeName) == "" || mode.Pricing == nil {
				return PricingCatalog{}, &errors.ValidationError{Field: "models." + modelID + ".modes", Value: modeName, Message: "mode names and pricing are required"}
			}
			if err := mode.Pricing.Validate(); err != nil {
				return PricingCatalog{}, errors.WrapResource("validate", "provider mode pricing", modelID+"/"+modeName, err)
			}
		}
	}
	return catalog, nil
}

func validateEvidence(evidence Evidence) error {
	parsedURL, err := url.Parse(evidence.URL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return &errors.ValidationError{Field: "evidence.url", Value: evidence.URL, Message: "must be an absolute HTTPS URL"}
	}
	if strings.TrimSpace(evidence.Revision) == "" || evidence.ObservedAt.IsZero() || strings.TrimSpace(evidence.Scope) == "" {
		return &errors.ValidationError{Field: "evidence", Value: evidence, Message: "requires revision, observed_at, and scope"}
	}
	return nil
}
