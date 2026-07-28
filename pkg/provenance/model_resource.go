package provenance

import (
	"net/url"
	"strings"
)

const modelResourceSeparator = "/"

// ModelResourceID returns the durable provider-scoped identity used for model
// provenance. Provider and model identifiers are opaque and escaped
// independently so slashes, colons, and percent signs cannot make two
// identities collide with one another or with the provenance key format.
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
