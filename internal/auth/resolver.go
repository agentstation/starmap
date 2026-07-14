package auth

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	authMethodCloudChain      = "cloud_chain"
	validationMessageRequired = "is required"
)

// Environment supplies request-scoped environment values to credential resolution.
type Environment interface {
	LookupEnv(name string) (string, bool)
}

type processEnvironment struct{}

func (processEnvironment) LookupEnv(name string) (string, bool) { return os.LookupEnv(name) }

// CloudChainSession is an opaque official-SDK session selected for one provider.
// Implementations must not expose credentials through String or error output.
type CloudChainSession interface {
	ProviderID() catalogs.ProviderID
}

// CloudChainAdapter delegates credential discovery and lifecycle to an official SDK.
// ErrNotFound means the chain is absent; any other error means it is present but invalid.
type CloudChainAdapter interface {
	Resolve(context.Context) (CloudChainSession, error)
}

// CloudChainRegistration binds exactly one provider to one official-SDK adapter.
type CloudChainRegistration struct {
	Provider catalogs.ProviderID
	Adapter  CloudChainAdapter
}

// CloudChainRegistry is an immutable provider-to-adapter registry.
type CloudChainRegistry struct {
	adapters map[catalogs.ProviderID]CloudChainAdapter
}

// NewCloudChainRegistry constructs an exact registry and rejects duplicate providers.
func NewCloudChainRegistry(registrations ...CloudChainRegistration) (*CloudChainRegistry, error) {
	registry := &CloudChainRegistry{adapters: make(map[catalogs.ProviderID]CloudChainAdapter, len(registrations))}
	for _, registration := range registrations {
		if registration.Provider == "" || registration.Adapter == nil {
			return nil, &errors.ValidationError{Field: "cloud_chain.registration", Message: "provider and adapter are required"}
		}
		if _, found := registry.adapters[registration.Provider]; found {
			return nil, &errors.ValidationError{Field: "cloud_chain.provider", Value: registration.Provider, Message: "must have exactly one adapter"}
		}
		registry.adapters[registration.Provider] = registration.Adapter
	}
	return registry, nil
}

func (registry *CloudChainRegistry) adapter(provider catalogs.ProviderID) (CloudChainAdapter, bool) {
	if registry == nil {
		return nil, false
	}
	adapter, found := registry.adapters[provider]
	return adapter, found
}

// ValidateProvider verifies every provider-inferred cloud-chain reference before transport creation.
func (registry *CloudChainRegistry) ValidateProvider(provider *catalogs.Provider) error {
	if provider == nil {
		return &errors.ValidationError{Field: "provider", Message: validationMessageRequired}
	}
	if provider.Catalog == nil {
		return &errors.ValidationError{Field: "provider.catalog", Message: validationMessageRequired}
	}
	if _, declared := provider.Credentials[authMethodCloudChain]; declared {
		return &errors.ValidationError{Field: "provider.credentials.cloud_chain", Message: "cloud_chain is provider-inferred and has no credential block"}
	}
	for index, source := range provider.Catalog.Sources {
		if err := registry.validatePolicy(provider.ID, fmt.Sprintf("provider.catalog.sources[%d].auth", index), source.Auth, provider.Credentials); err != nil {
			return err
		}
	}
	if provider.Invocation != nil {
		for index, route := range provider.Invocation.Routes {
			if err := registry.validatePolicy(provider.ID, fmt.Sprintf("provider.invocation.routes[%d].auth", index), route.Auth, provider.Credentials); err != nil {
				return err
			}
		}
	}
	return nil
}

func (registry *CloudChainRegistry) validatePolicy(
	provider catalogs.ProviderID,
	field string,
	policy catalogs.ProviderAuthPolicy,
	credentials map[catalogs.ProviderCredentialID]catalogs.ProviderCredential,
) error {
	if policy.Mode == catalogs.ProviderAuthModeOptional {
		_, hasKey := credentials[catalogs.ProviderCredentialID(catalogs.ProviderCredentialKindAPIKey)]
		_, hasChain := registry.adapter(provider)
		if !hasKey && !hasChain {
			return &errors.ValidationError{Field: field, Message: "optional requires conventional api_key metadata or one registered cloud chain"}
		}
		return nil
	}
	for _, method := range policy.Methods {
		switch method {
		case "aws_default", "azure_default", "google_adc", "oci_default":
			return &errors.ValidationError{Field: field, Value: method, Message: "vendor cloud-chain names are not part of the exact schema"}
		case authMethodCloudChain:
			if _, found := registry.adapter(provider); !found {
				return &errors.ValidationError{Field: field, Value: method, Message: "provider has no registered cloud-chain adapter"}
			}
		}
	}
	return nil
}

// Resolver resolves source-local auth without mutating provider configuration.
type Resolver struct {
	environment Environment
	cloud       *CloudChainRegistry
}

// ResolverOption configures a Resolver.
type ResolverOption func(*Resolver)

// WithEnvironment supplies an explicit environment, primarily for isolated callers and tests.
func WithEnvironment(environment Environment) ResolverOption {
	return func(resolver *Resolver) { resolver.environment = environment }
}

// WithCloudChainRegistry supplies the provider-inferred official-SDK registry.
func WithCloudChainRegistry(registry *CloudChainRegistry) ResolverOption {
	return func(resolver *Resolver) { resolver.cloud = registry }
}

// NewResolver constructs a request-scoped credential resolver.
func NewResolver(options ...ResolverOption) *Resolver {
	resolver := &Resolver{environment: processEnvironment{}}
	for _, option := range options {
		option(resolver)
	}
	return resolver
}

// ResolvedAuth is a request-scoped authentication result. Secret values are private.
type ResolvedAuth struct {
	provider   catalogs.ProviderID
	source     string
	method     catalogs.ProviderCredentialID
	credential *resolvedAPIKey
	compound   *resolvedCompound
	cloud      CloudChainSession
}

type resolvedAPIKey struct {
	value     string
	transport catalogs.ProviderCredentialTransport
}

type resolvedCompound struct {
	inputs map[string]string
}

// Method returns the selected safe method identifier. Empty means unauthenticated.
func (auth ResolvedAuth) Method() catalogs.ProviderCredentialID { return auth.method }

// Anonymous reports whether this source will execute without authentication.
func (auth ResolvedAuth) Anonymous() bool { return auth.method == "" }

// CloudSession returns the opaque SDK session when cloud_chain was selected.
func (auth ResolvedAuth) CloudSession() CloudChainSession { return auth.cloud }

// APIKey returns the selected request-scoped key to the connector that owns
// the wire protocol. It is never included in diagnostics or serialization.
func (auth ResolvedAuth) APIKey() (string, bool) {
	if auth.credential == nil {
		return "", false
	}
	return auth.credential.value, true
}

// Input returns one named compound credential input to its owning adapter.
// The value is request-scoped and must not be logged or persisted.
func (auth ResolvedAuth) Input(name string) (string, bool) {
	if auth.compound == nil {
		return "", false
	}
	value, found := auth.compound.inputs[name]
	return value, found
}

// String returns secret-safe diagnostic identity.
func (auth ResolvedAuth) String() string {
	method := string(auth.method)
	if method == "" {
		method = "none"
	}
	return fmt.Sprintf("provider=%s source=%s auth=%s", auth.provider, auth.source, method)
}

// Apply applies resolved HTTP API-key authentication. Cloud sessions are consumed by SDK adapters.
func (auth ResolvedAuth) Apply(request *http.Request) error {
	if request == nil {
		return &errors.ValidationError{Field: "request", Message: validationMessageRequired}
	}
	if auth.credential == nil {
		return nil
	}
	transport := auth.credential.transport
	if transport.QueryParam != "" {
		query := request.URL.Query()
		query.Set(transport.QueryParam, auth.credential.value)
		request.URL.RawQuery = query.Encode()
		return nil
	}
	value := auth.credential.value
	switch transport.Scheme {
	case catalogs.ProviderCredentialSchemeBearer:
		value = "Bearer " + value
	case catalogs.ProviderCredentialSchemeBasic:
		value = "Basic " + value
	case catalogs.ProviderCredentialSchemeDirect:
	default:
		return &errors.ValidationError{Field: "resolved_auth.transport.scheme", Value: transport.Scheme, Message: "is not supported"}
	}
	request.Header.Set(transport.Header, value)
	return nil
}

// Resolve selects authentication once for one logical source.
func (resolver *Resolver) Resolve(ctx context.Context, provider *catalogs.Provider, source catalogs.ProviderSource) (ResolvedAuth, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if provider == nil {
		return ResolvedAuth{}, &errors.ValidationError{Field: "provider", Message: validationMessageRequired}
	}
	if err := ctx.Err(); err != nil {
		return ResolvedAuth{}, err
	}
	result := ResolvedAuth{provider: provider.ID, source: source.ID}
	if source.Auth.Mode == catalogs.ProviderAuthModeNone {
		return result, nil
	}
	if source.Auth.Mode == catalogs.ProviderAuthModeOptional {
		if _, declared := provider.Credentials[catalogs.ProviderCredentialID(catalogs.ProviderCredentialKindAPIKey)]; declared {
			resolved, found, err := resolver.resolveCredential(provider, source.ID, catalogs.ProviderCredentialID(catalogs.ProviderCredentialKindAPIKey))
			if err != nil {
				return ResolvedAuth{}, err
			}
			if found {
				return resolved, nil
			}
		}
		resolved, found, err := resolver.resolveCloudChain(ctx, provider.ID, source.ID)
		if err != nil {
			return ResolvedAuth{}, err
		}
		if found {
			return resolved, nil
		}
		return result, nil
	}
	for _, method := range source.Auth.Methods {
		var (
			resolved ResolvedAuth
			found    bool
			err      error
		)
		if method == authMethodCloudChain {
			resolved, found, err = resolver.resolveCloudChain(ctx, provider.ID, source.ID)
		} else {
			resolved, found, err = resolver.resolveCredential(provider, source.ID, method)
		}
		if err != nil {
			return ResolvedAuth{}, err
		}
		if found {
			return resolved, nil
		}
	}
	return ResolvedAuth{}, &errors.AuthenticationError{
		Provider: string(provider.ID), Method: authPolicyName(source.Auth), Message: "required authentication is unavailable", Err: errors.ErrAPIKeyRequired,
	}
}

func (resolver *Resolver) resolveCredential(provider *catalogs.Provider, sourceID string, method catalogs.ProviderCredentialID) (ResolvedAuth, bool, error) {
	credential, found := provider.Credentials[method]
	if !found {
		return ResolvedAuth{}, false, nil
	}
	normalized, err := credential.Normalized(method)
	if err != nil {
		return ResolvedAuth{}, false, err
	}
	if normalized.Kind == catalogs.ProviderCredentialKindCompound {
		return resolver.resolveCompound(provider.ID, sourceID, method, normalized)
	}
	value, envName, found, err := resolver.environmentValue(normalized.Env)
	if err != nil {
		return ResolvedAuth{}, false, &errors.AuthenticationError{
			Provider: string(provider.ID), Method: string(method), Message: err.Error(), Err: errors.ErrAPIKeyInvalid,
		}
	}
	if !found {
		return ResolvedAuth{}, false, nil
	}
	if strings.TrimSpace(value) == "" {
		return ResolvedAuth{}, false, &errors.AuthenticationError{
			Provider: string(provider.ID), Method: string(method), Message: fmt.Sprintf("%s is present but empty", envName), Err: errors.ErrAPIKeyInvalid,
		}
	}
	return ResolvedAuth{
		provider: provider.ID, source: sourceID, method: method,
		credential: &resolvedAPIKey{value: value, transport: normalized.Transport},
	}, true, nil
}

func (resolver *Resolver) resolveCompound(
	provider catalogs.ProviderID,
	sourceID string,
	method catalogs.ProviderCredentialID,
	credential catalogs.ProviderCredential,
) (ResolvedAuth, bool, error) {
	inputIDs := make([]string, 0, len(credential.Inputs))
	for inputID := range credential.Inputs {
		inputIDs = append(inputIDs, inputID)
	}
	sort.Strings(inputIDs)
	values := make(map[string]string, len(inputIDs))
	missing := make([]string, 0)
	for _, inputID := range inputIDs {
		value, envName, found, err := resolver.environmentValue(credential.Inputs[inputID].Env)
		if err != nil {
			return ResolvedAuth{}, false, &errors.AuthenticationError{Provider: string(provider), Method: string(method), Message: err.Error(), Err: errors.ErrAPIKeyInvalid}
		}
		if !found {
			missing = append(missing, inputID)
			continue
		}
		if strings.TrimSpace(value) == "" {
			return ResolvedAuth{}, false, &errors.AuthenticationError{Provider: string(provider), Method: string(method), Message: fmt.Sprintf("%s is present but empty", envName), Err: errors.ErrAPIKeyInvalid}
		}
		values[inputID] = value
	}
	if len(values) == 0 {
		return ResolvedAuth{}, false, nil
	}
	if len(missing) != 0 {
		return ResolvedAuth{}, false, &errors.AuthenticationError{
			Provider: string(provider), Method: string(method),
			Message: "compound credential is partially configured; missing inputs: " + strings.Join(missing, ","), Err: errors.ErrAPIKeyInvalid,
		}
	}
	return ResolvedAuth{provider: provider, source: sourceID, method: method, compound: &resolvedCompound{inputs: values}}, true, nil
}

func (resolver *Resolver) environmentValue(names catalogs.ProviderEnvironmentNames) (value, selected string, found bool, err error) {
	for _, name := range names {
		candidate, exists := resolver.environment.LookupEnv(name)
		if !exists {
			continue
		}
		if !found {
			value, selected, found = candidate, name, true
			continue
		}
		if candidate != value {
			return "", "", false, &errors.ConflictError{
				Resource: "credential environment aliases", Expected: selected, Actual: name,
				Message: "simultaneously set aliases contain different values",
			}
		}
	}
	return value, selected, found, nil
}

func (resolver *Resolver) resolveCloudChain(ctx context.Context, provider catalogs.ProviderID, source string) (ResolvedAuth, bool, error) {
	adapter, found := resolver.cloud.adapter(provider)
	if !found {
		return ResolvedAuth{}, false, nil
	}
	session, err := adapter.Resolve(ctx)
	if err != nil {
		if stderrors.Is(err, errors.ErrNotFound) {
			return ResolvedAuth{}, false, nil
		}
		return ResolvedAuth{}, false, &errors.AuthenticationError{
			Provider: string(provider), Method: authMethodCloudChain, Message: "registered cloud chain is invalid", Err: err,
		}
	}
	if session == nil || session.ProviderID() != provider {
		return ResolvedAuth{}, false, &errors.AuthenticationError{
			Provider: string(provider), Method: authMethodCloudChain, Message: "registered cloud chain returned an invalid provider session", Err: errors.ErrAPIKeyInvalid,
		}
	}
	return ResolvedAuth{provider: provider, source: source, method: authMethodCloudChain, cloud: session}, true, nil
}

func authPolicyName(policy catalogs.ProviderAuthPolicy) string {
	if policy.Mode != "" {
		return string(policy.Mode)
	}
	methods := make([]string, len(policy.Methods))
	for index, method := range policy.Methods {
		methods[index] = string(method)
	}
	return strings.Join(methods, ",")
}
