package provenance

import (
	"net/url"
	"strings"
)

const modelResourceSeparator = "/"

// ModelResourceID returns the durable provider-scoped identity for model
// provenance. It escapes opaque provider and model identifiers independently.
// This prevents slashes, colons, or percent signs from causing key collisions.
func ModelResourceID(providerID, modelID string) string {
	return url.QueryEscape(providerID) + modelResourceSeparator + url.QueryEscape(modelID)
}

// ParseModelResourceID decodes an identity produced by ModelResourceID.
func ParseModelResourceID(resourceID string) (providerID, modelID string, ok bool) {
	encodedProvider, encodedModel, found := strings.Cut(resourceID, modelResourceSeparator)
	if !found || encodedProvider == "" || encodedModel == "" {
		return "", "", false
	}

	providerID, err := url.QueryUnescape(encodedProvider)
	if err != nil || providerID == "" {
		return "", "", false
	}
	modelID, err = url.QueryUnescape(encodedModel)
	if err != nil || modelID == "" {
		return "", "", false
	}
	if ModelResourceID(providerID, modelID) != resourceID {
		return "", "", false
	}
	return providerID, modelID, true
}
