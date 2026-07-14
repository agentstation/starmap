package catalogs

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// Provider represents a provider configuration.
type Provider struct {
	// Core identification and integration
	ID           ProviderID   `json:"id" yaml:"id"`                                         // Unique provider identifier
	Aliases      []ProviderID `json:"aliases,omitempty" yaml:"aliases,omitempty"`           // Alternative IDs this provider is known by (e.g., in models.dev)
	Name         string       `json:"name" yaml:"name"`                                     // Display name (must not be empty)
	Headquarters *string      `json:"headquarters,omitempty" yaml:"headquarters,omitempty"` // Company headquarters location
	IconURL      *string      `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`         // Provider icon/logo URL

	// Exact-current provider acquisition and invocation configuration.
	Credentials map[ProviderCredentialID]ProviderCredential `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	Invocation  *ProviderInvocation                         `json:"invocation,omitempty" yaml:"invocation,omitempty"`
	Advisories  []ProviderEnvironmentAdvisory               `json:"environment_advisories,omitempty" yaml:"environment_advisories,omitempty"`

	// Models
	Catalog *ProviderCatalog  `json:"catalog,omitempty" yaml:"catalog,omitempty"` // Models catalog configuration
	Models  map[string]*Model `json:"-" yaml:"-"`                                 // Available models indexed by model ID - not serialized to YAML

	// Status & Health
	StatusPageURL *string `json:"status_page_url,omitempty" yaml:"status_page_url,omitempty"` // Link to service status page

	// Privacy, Retention, and Governance Policies
	PrivacyPolicy    *ProviderPrivacyPolicy    `json:"privacy_policy,omitempty" yaml:"privacy_policy,omitempty"`       // Data collection and usage practices
	RetentionPolicy  *ProviderRetentionPolicy  `json:"retention_policy,omitempty" yaml:"retention_policy,omitempty"`   // Data retention and deletion practices
	GovernancePolicy *ProviderGovernancePolicy `json:"governance_policy,omitempty" yaml:"governance_policy,omitempty"` // Oversight and moderation practices

	// Extensions - controlled source-specific fields that are not canonical schema
	Extensions SourceExtensions `json:"extensions,omitempty" yaml:"extensions,omitempty"`
}

// ProviderCredentialID identifies one named provider authentication method.
type ProviderCredentialID string

// ProviderCredentialKind identifies how a named credential is applied.
type ProviderCredentialKind string

const (
	// ProviderCredentialKindAPIKey is an API key or token placed on transport.
	ProviderCredentialKindAPIKey ProviderCredentialKind = "api_key"
	// ProviderCredentialKindCompound is one named method with multiple required inputs.
	ProviderCredentialKindCompound ProviderCredentialKind = "compound"
)

// ProviderCredential configures one secret-bearing input without containing its value.
type ProviderCredential struct {
	Kind        ProviderCredentialKind             `json:"kind,omitempty" yaml:"kind,omitempty"`
	Env         ProviderEnvironmentNames           `json:"env,omitempty" yaml:"env,omitempty"`
	Inputs      map[string]ProviderCredentialInput `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Description string                             `json:"description,omitempty" yaml:"description,omitempty"`
	Transport   ProviderCredentialTransport        `json:"transport,omitempty" yaml:"transport,omitempty"`
}

// ProviderCredentialInput declares one required secret input owned by a compound method.
type ProviderCredentialInput struct {
	Env         ProviderEnvironmentNames `json:"env" yaml:"env"`
	Description string                   `json:"description,omitempty" yaml:"description,omitempty"`
}

// ProviderCredentialTransport configures non-secret credential placement.
type ProviderCredentialTransport struct {
	Header     string                   `json:"header,omitempty" yaml:"header,omitempty"`
	QueryParam string                   `json:"query_param,omitempty" yaml:"query_param,omitempty"`
	Scheme     ProviderCredentialScheme `json:"scheme,omitempty" yaml:"scheme,omitempty"`
}

// ProviderCredentialScheme identifies API-key transport encoding.
type ProviderCredentialScheme string

const (
	// ProviderCredentialSchemeBearer prefixes the credential with Bearer.
	ProviderCredentialSchemeBearer ProviderCredentialScheme = "bearer"
	// ProviderCredentialSchemeBasic applies HTTP Basic authentication encoding.
	ProviderCredentialSchemeBasic ProviderCredentialScheme = "basic"
	// ProviderCredentialSchemeDirect sends the credential without a prefix.
	ProviderCredentialSchemeDirect ProviderCredentialScheme = "direct"
)

// Normalized returns the exact runtime-independent credential configuration.
func (credential ProviderCredential) Normalized(id ProviderCredentialID) (ProviderCredential, error) {
	if credential.Kind == "" && id == ProviderCredentialID(ProviderCredentialKindAPIKey) {
		credential.Kind = ProviderCredentialKindAPIKey
	}
	if credential.Kind == ProviderCredentialKindCompound {
		return credential, nil
	}
	if credential.Kind != ProviderCredentialKindAPIKey {
		return ProviderCredential{}, &errors.ValidationError{
			Field: "provider.credentials." + string(id) + ".kind", Value: credential.Kind, Message: validationMessageIsNotSupported,
		}
	}
	if credential.Transport.Header == "" && credential.Transport.QueryParam == "" {
		credential.Transport.Header = "Authorization"
	}
	if credential.Transport.Scheme == "" {
		credential.Transport.Scheme = ProviderCredentialSchemeBearer
	}
	return credential, nil
}

// ProviderEnvironmentNames is an ordered, non-empty set of environment names.
// A scalar is the ordinary authoring form; a sequence is reserved for evidenced aliases.
type ProviderEnvironmentNames []string

// UnmarshalYAML accepts the scalar and ordered-list authoring forms.
func (names *ProviderEnvironmentNames) UnmarshalYAML(unmarshal func(any) error) error {
	var value any
	if err := unmarshal(&value); err != nil {
		return err
	}
	decoded, err := providerEnvironmentNames(value)
	if err != nil {
		return err
	}
	*names = decoded
	return nil
}

// MarshalYAML emits the smallest exact authoring form.
func (names ProviderEnvironmentNames) MarshalYAML() (any, error) {
	if len(names) == 1 {
		return names[0], nil
	}
	return []string(names), nil
}

// UnmarshalJSON accepts a string or ordered string array.
func (names *ProviderEnvironmentNames) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	decoded, err := providerEnvironmentNames(value)
	if err != nil {
		return err
	}
	*names = decoded
	return nil
}

// MarshalJSON emits a string for one name and an array for evidenced aliases.
func (names ProviderEnvironmentNames) MarshalJSON() ([]byte, error) {
	if len(names) == 1 {
		return json.Marshal(names[0])
	}
	return json.Marshal([]string(names))
}

func providerEnvironmentNames(value any) (ProviderEnvironmentNames, error) {
	var values []string
	switch typed := value.(type) {
	case string:
		values = []string{typed}
	case []string:
		values = typed
	case []any:
		values = make([]string, len(typed))
		for index, item := range typed {
			name, ok := item.(string)
			if !ok {
				return nil, &errors.ValidationError{Field: catalogProviderCredentialEnv, Value: item, Message: "must contain only environment names"}
			}
			values[index] = name
		}
	default:
		return nil, &errors.ValidationError{Field: catalogProviderCredentialEnv, Value: value, Message: "must be an environment name or ordered list"}
	}
	seen := make(map[string]struct{}, len(values))
	for _, name := range values {
		if strings.TrimSpace(name) == "" {
			return nil, &errors.ValidationError{Field: catalogProviderCredentialEnv, Message: "must not contain an empty name"}
		}
		if _, found := seen[name]; found {
			return nil, &errors.ValidationError{Field: catalogProviderCredentialEnv, Value: name, Message: validationMessageNoDuplicates}
		}
		seen[name] = struct{}{}
	}
	if len(values) == 0 {
		return nil, &errors.ValidationError{Field: catalogProviderCredentialEnv, Message: validationMessageMustNotBeEmpty}
	}
	return ProviderEnvironmentNames(values), nil
}

// ProviderAuthMode identifies one built-in source authentication form.
type ProviderAuthMode string

const (
	// ProviderAuthModeNone prohibits credential resolution and attachment.
	ProviderAuthModeNone ProviderAuthMode = "none"
	// ProviderAuthModeOptional tries conventional credentials before anonymous transport.
	ProviderAuthModeOptional ProviderAuthMode = "optional"
)

// ProviderAuthPolicy is either none, optional, one required method, or ordered required alternatives.
type ProviderAuthPolicy struct {
	Mode    ProviderAuthMode
	Methods []ProviderCredentialID
}

// UnmarshalYAML normalizes scalar and ordered-list auth forms.
func (policy *ProviderAuthPolicy) UnmarshalYAML(unmarshal func(any) error) error {
	var value any
	if err := unmarshal(&value); err != nil {
		return err
	}
	decoded, err := providerAuthPolicy(value)
	if err != nil {
		return err
	}
	*policy = decoded
	return nil
}

// MarshalYAML emits the canonical scalar or ordered-list auth form.
func (policy ProviderAuthPolicy) MarshalYAML() (any, error) { return policy.authoringValue() }

// UnmarshalJSON normalizes scalar and ordered-list auth forms.
func (policy *ProviderAuthPolicy) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	decoded, err := providerAuthPolicy(value)
	if err != nil {
		return err
	}
	*policy = decoded
	return nil
}

// MarshalJSON emits the canonical scalar or ordered-list auth form.
func (policy ProviderAuthPolicy) MarshalJSON() ([]byte, error) {
	value, err := policy.authoringValue()
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func (policy ProviderAuthPolicy) authoringValue() (any, error) {
	if policy.Mode != "" {
		if len(policy.Methods) != 0 {
			return nil, &errors.ValidationError{Field: catalogProviderSourceAuth, Message: "mode and required methods are mutually exclusive"}
		}
		return string(policy.Mode), nil
	}
	if len(policy.Methods) == 1 {
		return string(policy.Methods[0]), nil
	}
	if len(policy.Methods) > 1 {
		values := make([]string, len(policy.Methods))
		for index, method := range policy.Methods {
			values[index] = string(method)
		}
		return values, nil
	}
	return nil, &errors.ValidationError{Field: catalogProviderSourceAuth, Message: validationMessageIsRequired}
}

func providerAuthPolicy(value any) (ProviderAuthPolicy, error) {
	if scalar, ok := value.(string); ok {
		switch ProviderAuthMode(scalar) {
		case ProviderAuthModeNone, ProviderAuthModeOptional:
			return ProviderAuthPolicy{Mode: ProviderAuthMode(scalar)}, nil
		default:
			if strings.TrimSpace(scalar) == "" {
				return ProviderAuthPolicy{}, &errors.ValidationError{Field: catalogProviderSourceAuth, Message: validationMessageMustNotBeEmpty}
			}
			return ProviderAuthPolicy{Methods: []ProviderCredentialID{ProviderCredentialID(scalar)}}, nil
		}
	}
	items, ok := value.([]any)
	if !ok {
		if stringsValue, stringsOK := value.([]string); stringsOK {
			items = make([]any, len(stringsValue))
			for index := range stringsValue {
				items[index] = stringsValue[index]
			}
		} else {
			return ProviderAuthPolicy{}, &errors.ValidationError{Field: catalogProviderSourceAuth, Value: value, Message: "must be a method or ordered method list"}
		}
	}
	methods := make([]ProviderCredentialID, 0, len(items))
	seen := make(map[ProviderCredentialID]struct{}, len(items))
	for _, item := range items {
		method, methodOK := item.(string)
		if !methodOK || strings.TrimSpace(method) == "" {
			return ProviderAuthPolicy{}, &errors.ValidationError{Field: catalogProviderSourceAuth, Value: item, Message: "must contain only non-empty methods"}
		}
		if method == string(ProviderAuthModeNone) || method == string(ProviderAuthModeOptional) {
			return ProviderAuthPolicy{}, &errors.ValidationError{Field: catalogProviderSourceAuth, Value: method, Message: "none and optional must stand alone"}
		}
		id := ProviderCredentialID(method)
		if _, found := seen[id]; found {
			return ProviderAuthPolicy{}, &errors.ValidationError{Field: catalogProviderSourceAuth, Value: method, Message: validationMessageNoDuplicates}
		}
		seen[id] = struct{}{}
		methods = append(methods, id)
	}
	if len(methods) == 0 {
		return ProviderAuthPolicy{}, &errors.ValidationError{Field: catalogProviderSourceAuth, Message: validationMessageMustNotBeEmpty}
	}
	return ProviderAuthPolicy{Methods: methods}, nil
}

// ProviderObservationScope identifies publication eligibility for one source result.
type ProviderObservationScope string

const (
	// ProviderObservationScopeGlobalPublic is globally publishable.
	ProviderObservationScopeGlobalPublic ProviderObservationScope = "global_public"
	// ProviderObservationScopeRegionalPublic is regionally public and publishable.
	ProviderObservationScopeRegionalPublic ProviderObservationScope = "regional_public"
	// ProviderObservationScopeCredentialScoped is contextual and never publicly writable.
	ProviderObservationScopeCredentialScoped ProviderObservationScope = "credential_scoped" //nolint:gosec // Publication scope label, not a credential.
)

// ProviderObservationPolicy selects an invariant scope or auth-dependent scopes.
type ProviderObservationPolicy struct {
	Invariant     ProviderObservationScope
	Anonymous     ProviderObservationScope
	Authenticated ProviderObservationScope
}

// Scope returns the configured scope for one resolved source execution.
func (policy ProviderObservationPolicy) Scope(authenticated bool) ProviderObservationScope {
	if policy.Invariant != "" {
		return policy.Invariant
	}
	if authenticated {
		return policy.Authenticated
	}
	return policy.Anonymous
}

// UnmarshalYAML accepts one invariant scope or an exact auth-dependent mapping.
func (policy *ProviderObservationPolicy) UnmarshalYAML(unmarshal func(any) error) error {
	var value any
	if err := unmarshal(&value); err != nil {
		return err
	}
	decoded, err := providerObservationPolicy(value)
	if err != nil {
		return err
	}
	*policy = decoded
	return nil
}

// MarshalYAML emits the exact invariant or auth-dependent authoring form.
func (policy ProviderObservationPolicy) MarshalYAML() (any, error) { return policy.authoringValue() }

// UnmarshalJSON accepts one invariant scope or an exact auth-dependent mapping.
func (policy *ProviderObservationPolicy) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	decoded, err := providerObservationPolicy(value)
	if err != nil {
		return err
	}
	*policy = decoded
	return nil
}

// MarshalJSON emits the exact invariant or auth-dependent authoring form.
func (policy ProviderObservationPolicy) MarshalJSON() ([]byte, error) {
	value, err := policy.authoringValue()
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func (policy ProviderObservationPolicy) authoringValue() (any, error) {
	if policy.Invariant != "" {
		if policy.Anonymous != "" || policy.Authenticated != "" {
			return nil, &errors.ValidationError{Field: catalogProviderObservationScope, Message: "invariant and auth-dependent scopes are mutually exclusive"}
		}
		return string(policy.Invariant), nil
	}
	if policy.Anonymous == "" || policy.Authenticated == "" {
		return nil, &errors.ValidationError{Field: catalogProviderObservationScope, Message: "anonymous and authenticated scopes are required"}
	}
	return map[string]string{
		"anonymous":     string(policy.Anonymous),
		"authenticated": string(policy.Authenticated),
	}, nil
}

func providerObservationPolicy(value any) (ProviderObservationPolicy, error) {
	if scalar, ok := value.(string); ok {
		return ProviderObservationPolicy{Invariant: ProviderObservationScope(scalar)}, nil
	}
	values, ok := value.(map[string]any)
	if !ok {
		if stringValues, stringOK := value.(map[string]string); stringOK {
			values = make(map[string]any, len(stringValues))
			for key, item := range stringValues {
				values[key] = item
			}
		} else {
			return ProviderObservationPolicy{}, &errors.ValidationError{Field: catalogProviderObservationScope, Value: value, Message: "must be a scope or auth-dependent mapping"}
		}
	}
	if len(values) != 2 {
		return ProviderObservationPolicy{}, &errors.ValidationError{Field: catalogProviderObservationScope, Message: "auth-dependent mapping requires exactly anonymous and authenticated"}
	}
	anonymous, anonymousOK := values["anonymous"].(string)
	authenticated, authenticatedOK := values["authenticated"].(string)
	if !anonymousOK || !authenticatedOK {
		return ProviderObservationPolicy{}, &errors.ValidationError{Field: catalogProviderObservationScope, Message: "anonymous and authenticated must be scopes"}
	}
	return ProviderObservationPolicy{Anonymous: ProviderObservationScope(anonymous), Authenticated: ProviderObservationScope(authenticated)}, nil
}

// ProviderSourceEndpoint configures one exact acquisition endpoint.
type ProviderSourceEndpoint struct {
	Type               EndpointType   `json:"type" yaml:"type"`
	URL                string         `json:"url,omitempty" yaml:"url,omitempty"`
	BaseURLEnv         string         `json:"base_url_env,omitempty" yaml:"base_url_env,omitempty"`
	Path               string         `json:"path,omitempty" yaml:"path,omitempty"`
	ResponseCollection string         `json:"response_collection,omitempty" yaml:"response_collection,omitempty"`
	FieldMappings      []FieldMapping `json:"field_mappings,omitempty" yaml:"field_mappings,omitempty"`
	FeatureRules       []FeatureRule  `json:"feature_rules,omitempty" yaml:"feature_rules,omitempty"`
	AuthorMapping      *AuthorMapping `json:"author_mapping,omitempty" yaml:"author_mapping,omitempty"`
}

// ProviderBindingSource identifies where a source binding obtains values.
type ProviderBindingSource string

const (
	// ProviderBindingSourceEnv resolves a binding from process environment metadata.
	ProviderBindingSourceEnv ProviderBindingSource = "env"
	// ProviderBindingSourceStatic resolves a binding from checked-in source configuration.
	ProviderBindingSourceStatic ProviderBindingSource = "static"
	// ProviderBindingSourceCloudProfile resolves a binding through the provider SDK profile.
	ProviderBindingSourceCloudProfile ProviderBindingSource = "cloud_profile"
	// ProviderBindingSourceAPIResult resolves a binding from an earlier bounded API result.
	ProviderBindingSourceAPIResult ProviderBindingSource = "api_result"
	// ProviderBindingSourceGovernedSweep iterates a reviewed checked-in value set.
	ProviderBindingSourceGovernedSweep ProviderBindingSource = "governed_sweep"
)

// ProviderBindingRole identifies when a source binding participates.
type ProviderBindingRole string

const (
	// ProviderBindingRoleRequiredInput resolves before acquisition.
	ProviderBindingRoleRequiredInput ProviderBindingRole = "required_input"
	// ProviderBindingRoleIteration supplies bounded iteration values.
	ProviderBindingRoleIteration ProviderBindingRole = "iteration"
	// ProviderBindingRoleOutput records safe result metadata.
	ProviderBindingRoleOutput ProviderBindingRole = "output_metadata"
)

// ProviderSourceTopology identifies the bounded execution shape of one source.
type ProviderSourceTopology string

const (
	// ProviderSourceTopologySingleEndpoint performs one protocol request.
	ProviderSourceTopologySingleEndpoint ProviderSourceTopology = "single_endpoint"
	// ProviderSourceTopologyPaginated performs bounded pagination.
	ProviderSourceTopologyPaginated ProviderSourceTopology = "paginated"
	// ProviderSourceTopologyRegionalSweep performs a governed regional sweep.
	ProviderSourceTopologyRegionalSweep ProviderSourceTopology = "regional_sweep"
	// ProviderSourceTopologyGrouped performs one bounded multi-endpoint protocol group.
	ProviderSourceTopologyGrouped ProviderSourceTopology = "grouped"
)

// ProviderScopeBinding configures one typed source scope value.
type ProviderScopeBinding struct {
	Source      ProviderBindingSource    `json:"source" yaml:"source"`
	Name        ProviderEnvironmentNames `json:"name,omitempty" yaml:"name,omitempty"`
	Value       string                   `json:"value,omitempty" yaml:"value,omitempty"`
	Values      []string                 `json:"values,omitempty" yaml:"values,omitempty"`
	Fallback    ProviderBindingSource    `json:"fallback,omitempty" yaml:"fallback,omitempty"`
	Role        ProviderBindingRole      `json:"role" yaml:"role"`
	Description string                   `json:"description,omitempty" yaml:"description,omitempty"`
}

// ProviderEnvironmentAdvisory documents SDK-owned ambient configuration.
// It is descriptive metadata and never participates in Starmap resolution.
type ProviderEnvironmentAdvisory struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// ProviderOptionBinding configures one typed non-secret operational option.
type ProviderOptionBinding struct {
	Source      ProviderBindingSource    `json:"source" yaml:"source"`
	Name        ProviderEnvironmentNames `json:"name,omitempty" yaml:"name,omitempty"`
	Value       string                   `json:"value,omitempty" yaml:"value,omitempty"`
	Description string                   `json:"description,omitempty" yaml:"description,omitempty"`
}

// ProviderSource configures one independently executable logical catalog source.
type ProviderSource struct {
	ID               string                           `json:"id" yaml:"id"`
	Docs             string                           `json:"docs,omitempty" yaml:"docs,omitempty"`
	ObservationScope ProviderObservationPolicy        `json:"observation_scope" yaml:"observation_scope"`
	Auth             ProviderAuthPolicy               `json:"auth" yaml:"auth"`
	Optional         bool                             `json:"optional,omitempty" yaml:"optional,omitempty"`
	AcquisitionGroup string                           `json:"acquisition_group,omitempty" yaml:"acquisition_group,omitempty"`
	Topology         ProviderSourceTopology           `json:"topology,omitempty" yaml:"topology,omitempty"`
	Scopes           map[string]ProviderScopeBinding  `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Options          map[string]ProviderOptionBinding `json:"options,omitempty" yaml:"options,omitempty"`
	Endpoint         ProviderSourceEndpoint           `json:"endpoint" yaml:"endpoint"`
	Offering         *ProviderOfferingDefaults        `json:"offering,omitempty" yaml:"offering,omitempty"`
	Authors          []AuthorID                       `json:"authors,omitempty" yaml:"authors,omitempty"`
}

// ProviderInvocation contains provider inference routes independent of catalog acquisition.
type ProviderInvocation struct {
	Routes []ProviderInvocationRoute `json:"routes" yaml:"routes"`
}

// ProviderInvocationRoute configures one inference route.
type ProviderInvocationRoute struct {
	ID               string                    `json:"id" yaml:"id"`
	API              InvocationAPI             `json:"api" yaml:"api"`
	Auth             ProviderAuthPolicy        `json:"auth" yaml:"auth"`
	Endpoint         string                    `json:"endpoint" yaml:"endpoint"`
	HealthAPIURL     string                    `json:"health_api_url,omitempty" yaml:"health_api_url,omitempty"`
	HealthComponents []ProviderHealthComponent `json:"health_components,omitempty" yaml:"health_components,omitempty"`
}

var (
	safeResponseCollectionSegment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	safeProviderConfigurationID   = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$`)
)

// EndpointType specifies the API style for model listing.
type EndpointType string

const (
	// EndpointTypeOpenAI represents OpenAI-compatible API.
	EndpointTypeOpenAI EndpointType = "openai"
	// EndpointTypeAnthropic represents Anthropic API format.
	EndpointTypeAnthropic EndpointType = "anthropic"
	// EndpointTypeGoogle represents Google AI Studio.
	EndpointTypeGoogle EndpointType = "google"
	// EndpointTypeGoogleCloud represents Google Vertex AI.
	EndpointTypeGoogleCloud EndpointType = "google-cloud"
	// EndpointTypeBedrock represents native Amazon Bedrock runtime contracts.
	EndpointTypeBedrock EndpointType = "amazon-bedrock"
	// EndpointTypeAzureOpenAI represents Microsoft Foundry's Azure OpenAI contract.
	EndpointTypeAzureOpenAI EndpointType = "azure-openai"
	// EndpointTypeCohere represents Cohere's native model and inference contracts.
	EndpointTypeCohere EndpointType = "cohere"
	// EndpointTypeApplication represents a provider application without a public inference endpoint.
	EndpointTypeApplication EndpointType = "application"
	// EndpointTypeTogether represents Together AI's native model inventory.
	EndpointTypeTogether EndpointType = "together"
	// EndpointTypeHuggingFace represents Hugging Face Inference Providers routing inventory.
	EndpointTypeHuggingFace EndpointType = "huggingface"
	// EndpointTypeNVIDIA represents NVIDIA's hosted API Catalog inventory.
	EndpointTypeNVIDIA EndpointType = "nvidia"
	// EndpointTypeDatabricks represents Databricks foundation-model availability.
	EndpointTypeDatabricks EndpointType = "databricks"
	// EndpointTypeDatabricksWorkspace represents authenticated serving-endpoint discovery.
	EndpointTypeDatabricksWorkspace EndpointType = "databricks-workspace"
	// EndpointTypeSnowflake represents Snowflake Cortex session inventory.
	EndpointTypeSnowflake EndpointType = "snowflake"
	// EndpointTypeWatsonx represents IBM watsonx.ai foundation model inventory.
	EndpointTypeWatsonx EndpointType = "watsonx"
	// EndpointTypeWatsonxDeployments represents authenticated project/space deployment discovery.
	EndpointTypeWatsonxDeployments EndpointType = "watsonx-deployments"
	// EndpointTypeOCI represents OCI Generative AI regional contracts.
	EndpointTypeOCI EndpointType = "oci-generative-ai"
	// EndpointTypeCloudflare represents Cloudflare Workers AI contracts.
	EndpointTypeCloudflare EndpointType = "cloudflare-workers-ai"
)

// FieldMapping defines how to map API response fields to model fields.
// Type conversion is automatic based on the destination field type.
type FieldMapping struct {
	From     string                    `yaml:"from" json:"from"`                             // Source field path in API response (e.g., "max_model_len")
	To       string                    `yaml:"to" json:"to"`                                 // Target field path in Model (e.g., "limits.context_window")
	Unit     ProviderNormalizationUnit `yaml:"unit,omitempty" json:"unit,omitempty"`         // Bounded numeric source unit
	Currency ModelPricingCurrency      `yaml:"currency,omitempty" json:"currency,omitempty"` // Required for pricing targets
	Mode     string                    `yaml:"mode,omitempty" json:"mode,omitempty"`         // Optional provider offering mode
	Tier     *ProviderPricingTier      `yaml:"tier,omitempty" json:"tier,omitempty"`         // Optional pricing tier
	Scale    *float64                  `yaml:"scale,omitempty" json:"scale,omitempty"`       // Optional finite non-negative pricing multiplier
	Values   map[string]string         `yaml:"values,omitempty" json:"values,omitempty"`     // Optional exact source-to-canonical enum mapping
}

// FeatureRule defines conditions for inferring model features.
type FeatureRule struct {
	Field    string   `yaml:"field" json:"field"`       // Field to check (e.g., "id", "owned_by")
	Contains []string `yaml:"contains" json:"contains"` // If field contains any of these strings
	Feature  string   `yaml:"feature" json:"feature"`   // Feature to enable (e.g., "tools", "reasoning")
	Value    bool     `yaml:"value" json:"value"`       // Value to set for the feature
}

// AuthorMapping defines how to extract and normalize authors.
type AuthorMapping struct {
	Field      string              `yaml:"field" json:"field"`           // Field to extract from (e.g., "owned_by")
	Normalized map[string]AuthorID `yaml:"normalized" json:"normalized"` // Normalization map (e.g., "Meta" -> "meta")
}

// ProviderOfferingDefaults contains provider-wide defaults applied to models
// discovered from the provider catalog endpoint. Per-model values override
// these defaults during source projection.
type ProviderOfferingDefaults struct {
	Access     OfferingAccess           `yaml:"access" json:"access"`
	Endpoint   ProviderOfferingEndpoint `yaml:"endpoint" json:"endpoint"`
	Deployment ProviderDeployment       `yaml:"deployment" json:"deployment"`
	Regions    []CloudRegion            `yaml:"regions,omitempty" json:"regions,omitempty"`
}

// Validate verifies that the defaults form a complete, routing-safe offering
// contract. The synthetic identity and lifecycle fields are only used to reuse
// canonical offering validation; they are not serialized or published.
func (d ProviderOfferingDefaults) Validate() error {
	return (ProviderOffering{
		ProviderID:      catalogProviderConfigurationID,
		ProviderModelID: catalogProviderConfigurationID,
		DefinitionID:    catalogProviderConfigurationID,
		Availability:    OfferingAvailabilityAvailable,
		Access:          d.Access,
		Regions:         d.Regions,
		Deployment:      d.Deployment,
		Endpoint:        d.Endpoint,
		Lifecycle:       OfferingLifecycleActive,
	}).Validate()
}

// ProviderCatalog represents information about a provider's models.
type ProviderCatalog struct {
	Sources []ProviderSource `yaml:"sources" json:"sources"` // Exact-current logical acquisition sources
}

// ValidateConfiguration verifies the serialized provider catalog contract.
// Runtime credential values are intentionally excluded from this validation.
func (p *Provider) ValidateConfiguration() error {
	if p == nil {
		return &errors.ValidationError{Field: catalogResourceProvider, Message: validationMessageIsRequired}
	}
	if p.Catalog == nil {
		return &errors.ValidationError{Field: "provider.catalog", Message: validationMessageIsRequired}
	}
	if len(p.Catalog.Sources) == 0 {
		return &errors.ValidationError{Field: "provider.catalog.sources", Message: validationMessageMustNotBeEmpty}
	}
	return p.validateSourceConfiguration()
}

func (p *Provider) validateSourceConfiguration() error { //nolint:gocyclo // Explicit source-contract validation remains auditable as one ordered boundary.
	if err := validateProviderCredentials(p.Credentials); err != nil {
		return err
	}
	if err := validateProviderAdvisories(p.Advisories); err != nil {
		return err
	}
	seenSources := make(map[string]struct{}, len(p.Catalog.Sources))
	for index := range p.Catalog.Sources {
		source := p.Catalog.Sources[index]
		field := fmt.Sprintf("provider.catalog.sources[%d]", index)
		if !safeProviderConfigurationID.MatchString(source.ID) {
			return &errors.ValidationError{Field: field + ".id", Value: source.ID, Message: "must be a safe identifier"}
		}
		if _, found := seenSources[source.ID]; found {
			return &errors.ValidationError{Field: field + ".id", Value: source.ID, Message: validationMessageMustBeUnique}
		}
		seenSources[source.ID] = struct{}{}
		if err := validateProviderObservationPolicy(field+".observation_scope", source.ObservationScope, source.Auth); err != nil {
			return err
		}
		if err := validateProviderAuthPolicy(field+".auth", source.Auth, p.Credentials); err != nil {
			return err
		}
		if source.Endpoint.Type == "" {
			return &errors.ValidationError{Field: field + ".endpoint.type", Message: validationMessageIsRequired}
		}
		if source.Endpoint.Type != EndpointTypeApplication && strings.TrimSpace(source.Endpoint.URL) == "" && strings.TrimSpace(source.Endpoint.BaseURLEnv) == "" {
			return &errors.ValidationError{Field: field + ".endpoint.url", Message: "URL or base_url_env is required"}
		}
		if err := validateProviderSourceEndpoint(field+".endpoint", source.Endpoint); err != nil {
			return err
		}
		if err := validateProviderTopology(field+".topology", source.Topology); err != nil {
			return err
		}
		if err := validateProviderScopeBindings(field+".scopes", source.Scopes); err != nil {
			return err
		}
		if err := validateProviderOptions(field+".options", source.Options); err != nil {
			return err
		}
		if err := validateProviderAuthors(field+".authors", source.Authors); err != nil {
			return err
		}
		if err := ValidateProviderFieldMappings(source.Endpoint.FieldMappings); err != nil {
			return err
		}
		if err := ValidateProviderFeatureRules(source.Endpoint.FeatureRules); err != nil {
			return err
		}
		if source.Offering != nil {
			if err := source.Offering.Validate(); err != nil {
				return errors.WrapResource("validate", "provider source offering defaults", source.ID, err)
			}
		}
	}
	if p.Invocation == nil {
		return nil
	}
	seenRoutes := make(map[string]struct{}, len(p.Invocation.Routes))
	for index := range p.Invocation.Routes {
		route := p.Invocation.Routes[index]
		field := fmt.Sprintf("provider.invocation.routes[%d]", index)
		if !safeProviderConfigurationID.MatchString(route.ID) || route.API == "" || strings.TrimSpace(route.Endpoint) == "" {
			return &errors.ValidationError{Field: field, Message: "id, api, and endpoint are required"}
		}
		if _, found := seenRoutes[route.ID]; found {
			return &errors.ValidationError{Field: field + ".id", Value: route.ID, Message: validationMessageMustBeUnique}
		}
		seenRoutes[route.ID] = struct{}{}
		if err := validateProviderAuthPolicy(field+".auth", route.Auth, p.Credentials); err != nil {
			return err
		}
		if err := validateProviderHTTPSURL(field+".endpoint", route.Endpoint); err != nil {
			return err
		}
		if route.HealthAPIURL != "" {
			if err := validateProviderHTTPSURL(field+".health_api_url", route.HealthAPIURL); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateProviderAdvisories(advisories []ProviderEnvironmentAdvisory) error {
	seen := make(map[string]struct{}, len(advisories))
	for index, advisory := range advisories {
		field := fmt.Sprintf("provider.environment_advisories[%d].name", index)
		if strings.TrimSpace(advisory.Name) == "" || strings.TrimSpace(advisory.Name) != advisory.Name {
			return &errors.ValidationError{Field: field, Value: advisory.Name, Message: "must be an exact environment name"}
		}
		if _, found := seen[advisory.Name]; found {
			return &errors.ValidationError{Field: field, Value: advisory.Name, Message: validationMessageMustBeUnique}
		}
		seen[advisory.Name] = struct{}{}
	}
	return nil
}

func validateProviderCredentials(credentials map[ProviderCredentialID]ProviderCredential) error {
	for id, credential := range credentials {
		field := "provider.credentials." + string(id)
		if strings.TrimSpace(string(id)) == "" {
			return &errors.ValidationError{Field: "provider.credentials", Message: "credential names must not be empty"}
		}
		if !safeProviderConfigurationID.MatchString(string(id)) {
			return &errors.ValidationError{Field: "provider.credentials", Value: id, Message: "credential name is unsafe"}
		}
		normalized, err := credential.Normalized(id)
		if err != nil {
			return err
		}
		if normalized.Kind == ProviderCredentialKindCompound {
			if len(normalized.Env) != 0 || normalized.Transport != (ProviderCredentialTransport{}) {
				return &errors.ValidationError{Field: field, Message: "compound credentials own inputs and cannot declare env or transport"}
			}
			if len(normalized.Inputs) < 2 {
				return &errors.ValidationError{Field: field + ".inputs", Message: "compound credentials require at least two inputs"}
			}
			for inputID, input := range normalized.Inputs {
				if !safeProviderConfigurationID.MatchString(inputID) {
					return &errors.ValidationError{Field: field + ".inputs", Value: inputID, Message: "input name is unsafe"}
				}
				if _, inputErr := providerEnvironmentNames([]string(input.Env)); inputErr != nil {
					return &errors.ValidationError{Field: field + ".inputs." + inputID + ".env", Message: inputErr.Error()}
				}
			}
			continue
		}
		if len(normalized.Inputs) != 0 {
			return &errors.ValidationError{Field: field + ".inputs", Message: "api_key cannot declare compound inputs"}
		}
		if _, err := providerEnvironmentNames([]string(normalized.Env)); err != nil {
			return err
		}
		if normalized.Transport.Header != "" && normalized.Transport.QueryParam != "" {
			return &errors.ValidationError{Field: field + ".transport", Message: "header and query_param are mutually exclusive"}
		}
		switch normalized.Transport.Scheme {
		case ProviderCredentialSchemeBearer, ProviderCredentialSchemeBasic, ProviderCredentialSchemeDirect:
		default:
			return &errors.ValidationError{Field: field + ".transport.scheme", Value: normalized.Transport.Scheme, Message: validationMessageIsNotSupported}
		}
		if normalized.Transport.QueryParam != "" && normalized.Transport.Scheme != ProviderCredentialSchemeDirect {
			return &errors.ValidationError{Field: field + ".transport.scheme", Message: "query credentials require direct scheme"}
		}
	}
	return nil
}

func validateProviderObservationScope(field string, scope ProviderObservationScope) error {
	switch scope {
	case ProviderObservationScopeGlobalPublic, ProviderObservationScopeRegionalPublic, ProviderObservationScopeCredentialScoped:
		return nil
	default:
		return &errors.ValidationError{Field: field, Value: scope, Message: validationMessageIsNotSupported}
	}
}

func validateProviderObservationPolicy(field string, policy ProviderObservationPolicy, auth ProviderAuthPolicy) error {
	if _, err := policy.authoringValue(); err != nil {
		return &errors.ValidationError{Field: field, Message: err.Error()}
	}
	if policy.Invariant != "" {
		return validateProviderObservationScope(field, policy.Invariant)
	}
	if auth.Mode != ProviderAuthModeOptional {
		return &errors.ValidationError{Field: field, Message: "auth-dependent scopes require optional auth"}
	}
	if err := validateProviderObservationScope(field+".anonymous", policy.Anonymous); err != nil {
		return err
	}
	return validateProviderObservationScope(field+".authenticated", policy.Authenticated)
}

func validateProviderSourceEndpoint(field string, endpoint ProviderSourceEndpoint) error {
	if endpoint.URL != "" {
		if err := validateProviderHTTPSURL(field+".url", endpoint.URL); err != nil {
			return err
		}
	}
	if endpoint.BaseURLEnv != "" && strings.TrimSpace(endpoint.BaseURLEnv) != endpoint.BaseURLEnv {
		return &errors.ValidationError{Field: field + ".base_url_env", Value: endpoint.BaseURLEnv, Message: "must be an exact environment name"}
	}
	if collection := endpoint.ResponseCollection; collection != "" {
		for index, segment := range strings.Split(collection, ".") {
			if !safeResponseCollectionSegment.MatchString(segment) {
				return &errors.ValidationError{Field: fmt.Sprintf("%s.response_collection[%d]", field, index), Value: segment, Message: "must be a safe dotted JSON object path"}
			}
		}
	}
	return nil
}

func validateProviderHTTPSURL(field, value string) error {
	parsed, err := url.Parse(value)
	loopbackDevelopment := false
	if err == nil && parsed.Scheme == "http" {
		host := parsed.Hostname()
		if address := net.ParseIP(host); address != nil {
			loopbackDevelopment = address.IsLoopback()
		} else {
			loopbackDevelopment = strings.EqualFold(host, "localhost")
		}
	}
	if err != nil || (parsed.Scheme != "https" && !loopbackDevelopment) || parsed.Host == "" || parsed.User != nil {
		return &errors.ValidationError{Field: field, Value: value, Message: "must be an absolute HTTPS or loopback development URL without user info"}
	}
	return nil
}

func validateProviderTopology(field string, topology ProviderSourceTopology) error {
	switch topology {
	case "", ProviderSourceTopologySingleEndpoint, ProviderSourceTopologyPaginated, ProviderSourceTopologyRegionalSweep, ProviderSourceTopologyGrouped:
		return nil
	default:
		return &errors.ValidationError{Field: field, Value: topology, Message: validationMessageIsNotSupported}
	}
}

func validateProviderScopeBindings(field string, bindings map[string]ProviderScopeBinding) error {
	for dimension, binding := range bindings {
		bindingField := field + "." + dimension
		if !safeProviderConfigurationID.MatchString(dimension) {
			return &errors.ValidationError{Field: field, Value: dimension, Message: "scope dimension is unsafe"}
		}
		switch binding.Source {
		case ProviderBindingSourceEnv:
			if _, err := providerEnvironmentNames([]string(binding.Name)); err != nil {
				return &errors.ValidationError{Field: bindingField + ".name", Message: err.Error()}
			}
			if binding.Value != "" || len(binding.Values) != 0 {
				return &errors.ValidationError{Field: bindingField, Message: "env binding cannot contain static values"}
			}
			if binding.Fallback != "" && binding.Fallback != ProviderBindingSourceCloudProfile {
				return &errors.ValidationError{Field: bindingField + ".fallback", Value: binding.Fallback, Message: "env binding supports only cloud_profile fallback"}
			}
		case ProviderBindingSourceStatic:
			if (binding.Value == "") == (len(binding.Values) == 0) {
				return &errors.ValidationError{Field: bindingField, Message: "static binding requires exactly one of value or values"}
			}
		case ProviderBindingSourceCloudProfile:
			if len(binding.Name) != 0 || binding.Value != "" || len(binding.Values) != 0 {
				return &errors.ValidationError{Field: bindingField, Message: "cloud_profile binding cannot contain configured values"}
			}
		case ProviderBindingSourceAPIResult:
			if binding.Role != ProviderBindingRoleOutput || len(binding.Name) != 0 || binding.Value != "" || len(binding.Values) != 0 {
				return &errors.ValidationError{Field: bindingField, Message: "api_result is output metadata without configured values"}
			}
		case ProviderBindingSourceGovernedSweep:
			if binding.Role != ProviderBindingRoleIteration || len(binding.Values) == 0 || len(binding.Name) != 0 || binding.Value != "" {
				return &errors.ValidationError{Field: bindingField, Message: "governed_sweep requires iteration values"}
			}
		default:
			return &errors.ValidationError{Field: bindingField + ".source", Value: binding.Source, Message: validationMessageIsNotSupported}
		}
		if binding.Source != ProviderBindingSourceEnv && binding.Fallback != "" {
			return &errors.ValidationError{Field: bindingField + ".fallback", Value: binding.Fallback, Message: "is supported only after env"}
		}
		switch binding.Role {
		case ProviderBindingRoleRequiredInput, ProviderBindingRoleIteration, ProviderBindingRoleOutput:
		default:
			return &errors.ValidationError{Field: bindingField + ".role", Value: binding.Role, Message: validationMessageIsNotSupported}
		}
	}
	return nil
}

func validateProviderOptions(field string, options map[string]ProviderOptionBinding) error {
	for name, option := range options {
		optionField := field + "." + name
		if !safeProviderConfigurationID.MatchString(name) {
			return &errors.ValidationError{Field: field, Value: name, Message: "option name is unsafe"}
		}
		switch option.Source {
		case ProviderBindingSourceEnv:
			if _, err := providerEnvironmentNames([]string(option.Name)); err != nil {
				return &errors.ValidationError{Field: optionField + ".name", Message: err.Error()}
			}
			if option.Value != "" {
				return &errors.ValidationError{Field: optionField, Message: "env option cannot contain a static value"}
			}
		case ProviderBindingSourceStatic:
			if len(option.Name) != 0 || option.Value == "" {
				return &errors.ValidationError{Field: optionField, Message: "static option requires one value"}
			}
		default:
			return &errors.ValidationError{Field: optionField + ".source", Value: option.Source, Message: "must be env or static"}
		}
	}
	return nil
}

func validateProviderAuthors(field string, authors []AuthorID) error {
	seen := make(map[AuthorID]struct{}, len(authors))
	for index, author := range authors {
		if strings.TrimSpace(string(author)) == "" {
			return &errors.ValidationError{Field: fmt.Sprintf("%s[%d]", field, index), Message: validationMessageMustNotBeEmpty}
		}
		if _, found := seen[author]; found {
			return &errors.ValidationError{Field: field, Value: author, Message: validationMessageNoDuplicates}
		}
		seen[author] = struct{}{}
	}
	return nil
}

func validateProviderAuthPolicy(field string, policy ProviderAuthPolicy, credentials map[ProviderCredentialID]ProviderCredential) error {
	if _, err := policy.authoringValue(); err != nil {
		return &errors.ValidationError{Field: field, Message: err.Error()}
	}
	if policy.Mode == ProviderAuthModeOptional {
		if _, found := credentials[ProviderCredentialID(ProviderCredentialKindAPIKey)]; !found {
			return &errors.ValidationError{Field: field, Message: "optional requires conventional api_key metadata or a registered cloud chain"}
		}
		return nil
	}
	for _, method := range policy.Methods {
		if method == "cloud_chain" {
			continue
		}
		if _, found := credentials[method]; !found {
			return &errors.ValidationError{Field: field, Value: method, Message: "references an unknown credential"}
		}
	}
	return nil
}

// ProviderHealthComponent represents a specific component to monitor in a provider's health API.
type ProviderHealthComponent struct {
	ID   string `json:"id" yaml:"id"`                         // Component ID from the health API
	Name string `json:"name,omitempty" yaml:"name,omitempty"` // Human-readable component name
}

// ProviderID represents a provider identifier type for compile-time safety.
type ProviderID string

// String returns the string representation of a ProviderID.
func (pid ProviderID) String() string {
	return string(pid)
}

// Provider ID constants for compile-time safety and consistency.
const (
	ProviderIDAlibabaQwen      ProviderID = "alibaba"
	ProviderIDAlibabaCloud     ProviderID = "alibaba"
	ProviderIDAmazonBedrock    ProviderID = "amazon-bedrock"
	ProviderIDAnthropic        ProviderID = "anthropic"
	ProviderIDAnyscale         ProviderID = "anyscale"
	ProviderIDCerebras         ProviderID = "cerebras"
	ProviderIDCheckstep        ProviderID = "checkstep"
	ProviderIDCohere           ProviderID = "cohere"
	ProviderIDCursor           ProviderID = "cursor"
	ProviderIDConectys         ProviderID = "conectys"
	ProviderIDCove             ProviderID = "cove"
	ProviderIDDeepMind         ProviderID = "deepmind"
	ProviderIDDeepInfra        ProviderID = "deepinfra"
	ProviderIDDeepSeek         ProviderID = "deepseek"
	ProviderIDFireworksAI      ProviderID = "fireworks-ai"
	ProviderIDGoogleAIStudio   ProviderID = "google-ai-studio"
	ProviderIDGoogleVertex     ProviderID = "google-vertex"
	ProviderIDGroq             ProviderID = "groq"
	ProviderIDHuggingFace      ProviderID = "huggingface"
	ProviderIDMeta             ProviderID = "meta"
	ProviderIDMicrosoft        ProviderID = "microsoft"
	ProviderIDMicrosoftFoundry ProviderID = "microsoft-foundry"
	ProviderIDMistralAI        ProviderID = "mistral"
	ProviderIDNVIDIA           ProviderID = "nvidia"
	ProviderIDDatabricks       ProviderID = "databricks"
	ProviderIDSnowflake        ProviderID = "snowflake"
	ProviderIDWatsonx          ProviderID = "watsonx"
	ProviderIDOCI              ProviderID = "oracle-oci-generative-ai"
	ProviderIDCloudflare       ProviderID = "cloudflare-workers-ai"
	ProviderIDSambaNova        ProviderID = "sambanova"
	ProviderIDBaseten          ProviderID = "baseten"
	ProviderIDScaleway         ProviderID = "scaleway"
	ProviderIDHyperbolic       ProviderID = "hyperbolic"
	ProviderIDNovita           ProviderID = "novita"
	ProviderIDMoonshotAI       ProviderID = "moonshot-ai"
	ProviderIDOpenAI           ProviderID = "openai"
	ProviderIDOpenRouter       ProviderID = "openrouter"
	ProviderIDPerplexity       ProviderID = "perplexity"
	ProviderIDReplicate        ProviderID = "replicate"
	ProviderIDSafetyKit        ProviderID = "safetykit"
	ProviderIDTogetherAI       ProviderID = "together"
	ProviderIDVirtuousAI       ProviderID = "virtuousai"
	ProviderIDWebPurify        ProviderID = "webpurify"
	ProviderIDXAI              ProviderID = "xai"
)

// ProviderRetentionType represents different types of data retention policies.
type ProviderRetentionType string

// String returns the string representation of a ProviderRetentionType.
func (prt ProviderRetentionType) String() string {
	return string(prt)
}

// ProviderRetention types.
const (
	ProviderRetentionTypeFixed       ProviderRetentionType = "fixed"       // Specific duration (use Duration field)
	ProviderRetentionTypeNone        ProviderRetentionType = "none"        // No retention (immediate deletion)
	ProviderRetentionTypeIndefinite  ProviderRetentionType = "indefinite"  // Forever (duration = nil)
	ProviderRetentionTypeConditional ProviderRetentionType = "conditional" // Based on conditions (e.g., "until account deletion")
)

// ProviderPrivacyPolicy represents data collection and usage practices.
type ProviderPrivacyPolicy struct {
	PrivacyPolicyURL  *string `json:"privacy_policy_url,omitempty" yaml:"privacy_policy_url,omitempty"`     // Link to privacy policy
	TermsOfServiceURL *string `json:"terms_of_service_url,omitempty" yaml:"terms_of_service_url,omitempty"` // Link to terms of service
	RetainsData       *bool   `json:"retains_data,omitempty" yaml:"retains_data,omitempty"`                 // Whether provider stores/retains user data
	TrainsOnData      *bool   `json:"trains_on_data,omitempty" yaml:"trains_on_data,omitempty"`             // Whether provider trains models on user data
}

// ProviderRetentionPolicy represents how long data is kept and deletion practices.
type ProviderRetentionPolicy struct {
	Type     ProviderRetentionType `json:"type" yaml:"type"`                             // Type of retention policy
	Duration *time.Duration        `json:"duration,omitempty" yaml:"duration,omitempty"` // nil = forever, 0 = immediate deletion
	Details  *string               `json:"details,omitempty" yaml:"details,omitempty"`   // Human-readable description
}

// ProviderGovernancePolicy represents oversight and moderation practices.
type ProviderGovernancePolicy struct {
	ModerationRequired *bool   `json:"moderation_required,omitempty" yaml:"moderation_required,omitempty"` // Whether the provider requires moderation
	Moderated          *bool   `json:"moderated,omitempty" yaml:"moderated,omitempty"`                     // Whether provider content is moderated
	Moderator          *string `json:"moderator,omitempty" yaml:"moderator,omitempty"`                     // Who moderates the provider
}

// ProviderModerator represents a moderator for a provider.
type ProviderModerator string

// String returns the string representation of a ProviderModerator.
func (pm ProviderModerator) String() string {
	return string(pm)
}

// ProviderModerators.
const (
	// AI Platform Aggregators/Moderators.
	ProviderModeratorAnyscale    ProviderModerator = "anyscale"
	ProviderModeratorHuggingFace ProviderModerator = "huggingface"
	ProviderModeratorOpenRouter  ProviderModerator = "openrouter"
	ProviderModeratorReplicate   ProviderModerator = "replicate"
	ProviderModeratorTogetherAI  ProviderModerator = "together"

	// Specialized AI Safety/Moderation Companies.
	ProviderModeratorCheckstep  ProviderModerator = "checkstep"
	ProviderModeratorConectys   ProviderModerator = "conectys"
	ProviderModeratorCove       ProviderModerator = "cove"
	ProviderModeratorSafetyKit  ProviderModerator = "safetykit"
	ProviderModeratorVirtuousAI ProviderModerator = "virtuousai"
	ProviderModeratorWebPurify  ProviderModerator = "webpurify"

	// Self-Moderated (Major AI Companies).
	ProviderModeratorAnthropic      ProviderModerator = "anthropic"
	ProviderModeratorGoogleAIStudio ProviderModerator = "google-ai-studio"
	ProviderModeratorGoogleVertex   ProviderModerator = "google-vertex"
	ProviderModeratorGroq           ProviderModerator = "groq"
	ProviderModeratorMicrosoft      ProviderModerator = "microsoft"
	ProviderModeratorOpenAI         ProviderModerator = "openai"

	// Unknown/Unspecified.
	ProviderModeratorUnknown ProviderModerator = "unknown"
)

func catalogBaseURL(endpointURL, endpointPath string) string {
	endpointURL = strings.TrimRight(strings.TrimSpace(endpointURL), "/")
	endpointPath = "/" + strings.Trim(strings.TrimSpace(endpointPath), "/")
	if endpointPath == "/" || !strings.HasSuffix(endpointURL, endpointPath) {
		return endpointURL
	}
	return strings.TrimSuffix(endpointURL, endpointPath)
}

// Model retrieves a specific model from the provider.
func (p *Provider) Model(modelID string) (*Model, error) {
	if p == nil || p.Models == nil {
		return nil, &errors.ValidationError{
			Field:   catalogResourceProvider,
			Message: "provider or models not initialized",
		}
	}

	model, exists := p.Models[modelID]
	if !exists {
		return nil, &errors.NotFoundError{
			Resource: catalogResourceModel,
			ID:       modelID,
		}
	}

	return model, nil
}
