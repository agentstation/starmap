package auth

import (
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func starportReferenceName(provider catalogs.ProviderID, field catalogs.ProviderCredentialFieldID) (string, error) {
	name, err := catalogs.DerivedCredentialEnvironmentName("STARPORT", provider, field)
	if err != nil {
		return "", err
	}
	return "STARPORT_CATALOG_" + strings.TrimPrefix(name, "STARPORT_") + "_REFERENCE", nil
}

func (r *Resolver) fieldReference(key CredentialFieldKey) (ReferencePolicy, bool, error) {
	if policy, exists := r.references[key]; exists {
		return policy, true, nil
	}
	if r.environmentPolicy.current() != EnvironmentPolicyStarportCurrent {
		return ReferencePolicy{}, false, nil
	}
	name, err := starportReferenceName(key.ProviderID, key.FieldID)
	if err != nil {
		return ReferencePolicy{}, false, err
	}
	value, present := r.lookup(name)
	fallbackValue, fallbackPresent := r.lookup(name + "_FALLBACK_AMBIENT")
	if !present {
		if fallbackPresent {
			return ReferencePolicy{}, false, &errors.ValidationError{Field: name, Message: "fallback requires an explicit reference"}
		}
		return ReferencePolicy{}, false, nil
	}
	ref, err := ParseReference(value)
	if err != nil {
		return ReferencePolicy{}, false, err
	}
	if fallbackPresent && fallbackValue != "true" && fallbackValue != "false" {
		return ReferencePolicy{}, false, &errors.ValidationError{Field: name + "_FALLBACK_AMBIENT", Message: "must be true or false"}
	}
	return ReferencePolicy{Reference: ref, FallbackAmbient: fallbackValue == "true"}, true, nil
}
