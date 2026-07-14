// Package acquisition resolves one provider catalog source into a request-scoped
// execution contract. Catalog configuration remains immutable and secret-free;
// runtime credential and binding values live only in Source.
package acquisition

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Environment is the request-scoped environment lookup boundary.
type Environment interface {
	LookupEnv(string) (string, bool)
}

type processEnvironment struct{}

func (processEnvironment) LookupEnv(name string) (string, bool) { return os.LookupEnv(name) }

// Resolver resolves authentication and non-secret source inputs before any
// connector or transport is constructed.
type Resolver struct {
	auth        *auth.Resolver
	environment Environment
}

// ResolverOption configures a Resolver.
type ResolverOption func(*Resolver)

// WithAuthResolver supplies the credential resolver used by source preflight.
func WithAuthResolver(resolver *auth.Resolver) ResolverOption {
	return func(target *Resolver) { target.auth = resolver }
}

// WithEnvironment supplies an isolated runtime environment.
func WithEnvironment(environment Environment) ResolverOption {
	return func(target *Resolver) { target.environment = environment }
}

// NewResolver constructs a source resolver.
func NewResolver(options ...ResolverOption) *Resolver {
	resolver := &Resolver{auth: auth.NewResolver(), environment: processEnvironment{}}
	for _, option := range options {
		option(resolver)
	}
	return resolver
}

// Source is the complete request-scoped input for one logical acquisition.
// Runtime values are deliberately private and have no JSON/YAML representation.
type Source struct {
	provider catalogs.Provider
	config   catalogs.ProviderSource
	auth     auth.ResolvedAuth
	endpoint string
	bindings map[string]string
	options  map[string]string
}

// Provider returns a deep copy of the secret-free provider configuration.
func (source Source) Provider() catalogs.Provider { return catalogs.DeepCopyProvider(source.provider) }

// Config returns a deep copy of the selected source configuration.
func (source Source) Config() catalogs.ProviderSource {
	provider := catalogs.DeepCopyProvider(catalogs.Provider{Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{source.config}}})
	return provider.Catalog.Sources[0]
}

// ProviderID returns the safe provider identity.
func (source Source) ProviderID() catalogs.ProviderID { return source.provider.ID }

// SourceID returns the safe logical source identity.
func (source Source) SourceID() string { return source.config.ID }

// EndpointURL returns the resolved configured endpoint for this request.
func (source Source) EndpointURL() string { return source.endpoint }

// Auth returns the opaque resolved authentication contract.
func (source Source) Auth() auth.ResolvedAuth { return source.auth }

// Binding returns a resolved typed source binding.
func (source Source) Binding(name string) (string, bool) {
	value, found := source.bindings[name]
	return value, found
}

// Option returns a resolved typed operational option.
func (source Source) Option(name string) (string, bool) {
	value, found := source.options[name]
	return value, found
}

// String returns safe diagnostic identity without runtime values.
func (source Source) String() string {
	return fmt.Sprintf("provider=%s source=%s auth=%s", source.ProviderID(), source.SourceID(), source.auth.Method())
}

// Resolve validates and resolves one exact configured logical source.
func (resolver *Resolver) Resolve(ctx context.Context, provider *catalogs.Provider, sourceID string) (Source, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if provider == nil {
		return Source{}, &errors.ValidationError{Field: "provider", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return Source{}, err
	}
	if err := provider.ValidateConfiguration(); err != nil {
		return Source{}, err
	}
	config, err := configuredSource(provider, sourceID)
	if err != nil {
		return Source{}, err
	}
	resolvedAuth, err := resolver.auth.Resolve(ctx, provider, config)
	if err != nil {
		return Source{}, err
	}
	bindings, err := resolver.resolveBindings(provider.ID, config)
	if err != nil {
		return Source{}, err
	}
	options, err := resolver.resolveOptions(provider.ID, config)
	if err != nil {
		return Source{}, err
	}
	endpoint, err := resolver.resolveEndpoint(provider.ID, config)
	if err != nil {
		return Source{}, err
	}
	return Source{
		provider: catalogs.DeepCopyProvider(*provider), config: config, auth: resolvedAuth,
		endpoint: endpoint, bindings: bindings, options: options,
	}, nil
}

func configuredSource(provider *catalogs.Provider, sourceID string) (catalogs.ProviderSource, error) {
	if provider.Catalog == nil {
		return catalogs.ProviderSource{}, &errors.ValidationError{Field: "provider.catalog", Message: "is required"}
	}
	for _, source := range provider.Catalog.Sources {
		if source.ID == sourceID {
			copied := catalogs.DeepCopyProvider(catalogs.Provider{Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{source}}})
			return copied.Catalog.Sources[0], nil
		}
	}
	return catalogs.ProviderSource{}, &errors.NotFoundError{Resource: "provider source", ID: string(provider.ID) + "/" + sourceID}
}

func (resolver *Resolver) resolveBindings(provider catalogs.ProviderID, source catalogs.ProviderSource) (map[string]string, error) {
	resolved := make(map[string]string, len(source.Scopes))
	names := sortedKeys(source.Scopes)
	for _, name := range names {
		binding := source.Scopes[name]
		if binding.Role == catalogs.ProviderBindingRoleIteration || binding.Role == catalogs.ProviderBindingRoleOutput {
			continue
		}
		value, found, err := resolver.bindingValue(binding.Source, binding.Name, binding.Value)
		if err != nil {
			return nil, bindingError(provider, source.ID, name, err)
		}
		if !found && binding.Fallback != "" {
			// SDK/profile fallbacks are resolved by the provider-native adapter from
			// the already selected cloud session, never by ambient catalog methods.
			continue
		}
		if !found {
			return nil, bindingError(provider, source.ID, name, errors.ErrNotFound)
		}
		resolved[name] = value
	}
	return resolved, nil
}

func (resolver *Resolver) resolveOptions(provider catalogs.ProviderID, source catalogs.ProviderSource) (map[string]string, error) {
	resolved := make(map[string]string, len(source.Options))
	names := sortedKeys(source.Options)
	for _, name := range names {
		option := source.Options[name]
		value, found, err := resolver.bindingValue(option.Source, option.Name, option.Value)
		if err != nil {
			return nil, bindingError(provider, source.ID, name, err)
		}
		if found {
			resolved[name] = value
		}
	}
	return resolved, nil
}

func (resolver *Resolver) bindingValue(source catalogs.ProviderBindingSource, names catalogs.ProviderEnvironmentNames, static string) (string, bool, error) {
	switch source {
	case catalogs.ProviderBindingSourceStatic:
		return static, true, nil
	case catalogs.ProviderBindingSourceEnv:
		var selectedName, selectedValue string
		for _, name := range names {
			value, found := resolver.environment.LookupEnv(name)
			if !found {
				continue
			}
			if selectedName == "" {
				selectedName, selectedValue = name, value
				continue
			}
			if value != selectedValue {
				return "", false, &errors.ConflictError{Resource: "source binding environment aliases", Expected: selectedName, Actual: name, Message: "simultaneously set aliases contain different values"}
			}
		}
		if selectedName == "" {
			return "", false, nil
		}
		if strings.TrimSpace(selectedValue) == "" {
			return "", false, &errors.ValidationError{Field: selectedName, Message: "is present but empty"}
		}
		return selectedValue, true, nil
	case catalogs.ProviderBindingSourceCloudProfile,
		catalogs.ProviderBindingSourceAPIResult,
		catalogs.ProviderBindingSourceGovernedSweep:
		return "", false, nil
	default:
		return "", false, &errors.ValidationError{Field: "binding.source", Value: source, Message: "is not supported"}
	}
}

func (resolver *Resolver) resolveEndpoint(provider catalogs.ProviderID, source catalogs.ProviderSource) (string, error) {
	endpoint := strings.TrimSpace(source.Endpoint.URL)
	if source.Endpoint.BaseURLEnv != "" {
		if base, found := resolver.environment.LookupEnv(source.Endpoint.BaseURLEnv); found {
			if strings.TrimSpace(base) == "" {
				return "", bindingError(provider, source.ID, "endpoint", &errors.ValidationError{Field: source.Endpoint.BaseURLEnv, Message: "is present but empty"})
			}
			endpoint = joinEndpointURL(base, source.Endpoint.Path)
		}
	}
	if endpoint == "" {
		return "", bindingError(provider, source.ID, "endpoint", errors.ErrNotFound)
	}
	return endpoint, nil
}

func joinEndpointURL(baseURL, endpointPath string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpointPath = strings.TrimLeft(strings.TrimSpace(endpointPath), "/")
	if endpointPath == "" {
		return baseURL
	}
	return baseURL + "/" + endpointPath
}

func bindingError(provider catalogs.ProviderID, sourceID, name string, err error) error {
	return &errors.ConfigError{Component: string(provider) + "/" + sourceID, Message: "source input " + name + " is unavailable", Err: err}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
