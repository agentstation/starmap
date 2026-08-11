package auth

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ReferenceBackend identifies one credential source primitive.
type ReferenceBackend string

const (
	referenceBackendEnvironment ReferenceBackend = "env"
	referenceBackendFile        ReferenceBackend = "file"
)

var (
	referenceBackendPattern      = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	credentialEnvironmentPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

// Reference identifies one operator-selected credential source without
// containing source authentication values.
type Reference struct {
	backend  ReferenceBackend
	resource string
	field    string
	version  string
}

// ParseReference parses backend:resource?version=VERSION#field syntax.
func ParseReference(value string) (Reference, error) {
	backendValue, remainder, found := strings.Cut(value, ":")
	backend := ReferenceBackend(backendValue)
	if !found || !referenceBackendPattern.MatchString(backendValue) {
		return Reference{}, referenceValidationError("backend", "must be a lowercase kebab-case ID")
	}
	resourceAndQuery, field, hasField := strings.Cut(remainder, "#")
	resource, rawQuery, hasQuery := strings.Cut(resourceAndQuery, "?")
	if resource == "" {
		return Reference{}, referenceValidationError("resource", "is required")
	}
	if hasField && field == "" {
		return Reference{}, referenceValidationError("field", "must be nonempty")
	}
	if strings.Contains(field, "#") {
		return Reference{}, referenceValidationError("field", "must not contain #")
	}
	version := ""
	if hasQuery {
		if rawQuery == "" {
			return Reference{}, referenceValidationError("query", "must be nonempty")
		}
		query, err := url.ParseQuery(rawQuery)
		if err != nil {
			return Reference{}, referenceValidationError("query", "is invalid")
		}
		for key := range query {
			if key != "version" {
				return Reference{}, referenceValidationError("query", "contains an unsupported parameter")
			}
		}
		versions := query["version"]
		if len(versions) != 1 || versions[0] == "" {
			return Reference{}, referenceValidationError("version", "must contain one nonempty value")
		}
		version = versions[0]
	}
	return Reference{backend: backend, resource: resource, field: field, version: version}, nil
}

func referenceValidationError(field, message string) error {
	return &errors.ValidationError{
		Field: "credential_reference." + field, Message: message,
	}
}

// CredentialFieldKey identifies one deployment-owned provider field policy.
type CredentialFieldKey struct {
	ProviderID catalogs.ProviderID
	FieldID    catalogs.ProviderCredentialFieldID
}

// ReferencePolicy selects an explicit source and an optional not-configured
// fallback to ambient discovery.
type ReferencePolicy struct {
	Reference       Reference
	FallbackAmbient bool
}

// ResolverOption configures credential resolution.
type ResolverOption func(*Resolver)

// WithReferencePolicies configures deployment-owned field references.
func WithReferencePolicies(policies map[CredentialFieldKey]ReferencePolicy) ResolverOption {
	return func(resolver *Resolver) {
		resolver.references = make(map[CredentialFieldKey]ReferencePolicy, len(policies))
		for key, policy := range policies {
			resolver.references[key] = policy
		}
	}
}
