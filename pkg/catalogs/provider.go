package catalogs

import (
	"fmt"
	"os"
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

	// API key configuration
	APIKey *ProviderAPIKey `json:"api_key,omitempty" yaml:"api_key,omitempty"` // API key configuration

	// Environment variables configuration
	EnvVars []ProviderEnvVar `json:"env_vars,omitempty" yaml:"env_vars,omitempty"` // Required environment variables

	// Models
	Catalog *ProviderCatalog  `json:"catalog,omitempty" yaml:"catalog,omitempty"` // Models catalog configuration
	Models  map[string]*Model `json:"-" yaml:"-"`                                 // Available models indexed by model ID - not serialized to YAML

	// Status & Health
	StatusPageURL   *string                  `json:"status_page_url,omitempty" yaml:"status_page_url,omitempty"`   // Link to service status page
	ChatCompletions *ProviderChatCompletions `json:"chat_completions,omitempty" yaml:"chat_completions,omitempty"` // Chat completions API configuration

	// Privacy, Retention, and Governance Policies
	PrivacyPolicy    *ProviderPrivacyPolicy    `json:"privacy_policy,omitempty" yaml:"privacy_policy,omitempty"`       // Data collection and usage practices
	RetentionPolicy  *ProviderRetentionPolicy  `json:"retention_policy,omitempty" yaml:"retention_policy,omitempty"`   // Data retention and deletion practices
	GovernancePolicy *ProviderGovernancePolicy `json:"governance_policy,omitempty" yaml:"governance_policy,omitempty"` // Oversight and moderation practices

	// Extensions - controlled source-specific fields that are not canonical schema
	Extensions SourceExtensions `json:"extensions,omitempty" yaml:"extensions,omitempty"`

	// Runtime fields (not serialized)
	apiKeyValue  string            `json:"-" yaml:"-"` // Actual API key value loaded from environment
	EnvVarValues map[string]string `json:"-" yaml:"-"` // Actual environment variable values loaded at runtime
}

var safeResponseCollectionSegment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
	// EndpointTypeSnowflake represents Snowflake Cortex session inventory.
	EndpointTypeSnowflake EndpointType = "snowflake"
	// EndpointTypeWatsonx represents IBM watsonx.ai foundation model inventory.
	EndpointTypeWatsonx EndpointType = "watsonx"
	// EndpointTypeOCI represents OCI Generative AI regional contracts.
	EndpointTypeOCI EndpointType = "oci-generative-ai"
	// EndpointTypeCloudflare represents Cloudflare Workers AI contracts.
	EndpointTypeCloudflare EndpointType = "cloudflare-workers-ai"
	// EndpointTypeSambaNova represents SambaNova Cloud model contracts.
	EndpointTypeSambaNova EndpointType = "sambanova"
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

// ProviderEndpoint configures how to access the provider's model catalog.
type ProviderEndpoint struct {
	Type               EndpointType   `yaml:"type" json:"type"`                                                   // Required: API style
	URL                string         `yaml:"url" json:"url"`                                                     // Required: API endpoint
	BaseURLEnvVar      string         `yaml:"base_url_env_var,omitempty" json:"base_url_env_var,omitempty"`       // Optional env var for overriding the endpoint base URL
	Path               string         `yaml:"path,omitempty" json:"path,omitempty"`                               // Path appended when BaseURLEnvVar is set
	ResponseCollection string         `yaml:"response_collection,omitempty" json:"response_collection,omitempty"` // Optional dotted path to the model array in the response
	AuthRequired       bool           `yaml:"auth_required" json:"auth_required"`                                 // Required: Whether auth needed
	FieldMappings      []FieldMapping `yaml:"field_mappings,omitempty" json:"field_mappings,omitempty"`           // Field mappings
	FeatureRules       []FeatureRule  `yaml:"feature_rules,omitempty" json:"feature_rules,omitempty"`             // Feature inference rules
	AuthorMapping      *AuthorMapping `yaml:"author_mapping,omitempty" json:"author_mapping,omitempty"`           // Author extraction
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
		ProviderID:      "provider-configuration",
		ProviderModelID: "provider-configuration",
		DefinitionID:    "provider-configuration",
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
	Docs     *string                   `yaml:"docs" json:"docs"`                             // Documentation URL
	Endpoint ProviderEndpoint          `yaml:"endpoint" json:"endpoint"`                     // API endpoint configuration
	Offering *ProviderOfferingDefaults `yaml:"offering,omitempty" json:"offering,omitempty"` // Provider-wide offering defaults
	Authors  []AuthorID                `json:"authors,omitempty" yaml:"authors,omitempty"`   // List of authors to fetch from (for providers like Google Vertex AI)
}

// ValidateConfiguration verifies the serialized provider catalog contract.
// Runtime credential values are intentionally excluded from this validation.
func (p *Provider) ValidateConfiguration() error {
	if p == nil {
		return &errors.ValidationError{Field: "provider", Message: "is required"}
	}
	if p.Catalog == nil {
		return &errors.ValidationError{Field: "provider.catalog", Message: "is required"}
	}
	if p.Catalog.Endpoint.Type == "" {
		return &errors.ValidationError{Field: "provider.catalog.endpoint.type", Message: "is required"}
	}
	if collection := p.Catalog.Endpoint.ResponseCollection; collection != "" {
		for index, segment := range strings.Split(collection, ".") {
			if !safeResponseCollectionSegment.MatchString(segment) {
				return &errors.ValidationError{
					Field:   fmt.Sprintf("provider.catalog.endpoint.response_collection[%d]", index),
					Value:   segment,
					Message: "must be a safe dotted JSON object path",
				}
			}
		}
	}
	if p.Catalog.Offering != nil {
		if err := p.Catalog.Offering.Validate(); err != nil {
			return errors.WrapResource("validate", "provider offering defaults", string(p.ID), err)
		}
		if p.Catalog.Offering.Endpoint.BaseURL == "" {
			if strings.TrimSpace(p.Catalog.Endpoint.Path) == "" {
				return &errors.ValidationError{
					Field:   "provider.catalog.endpoint.path",
					Message: "is required when the offering base URL is derived from the catalog endpoint",
				}
			}
			if catalogBaseURL(p.Catalog.Endpoint.URL, p.Catalog.Endpoint.Path) == p.Catalog.Endpoint.URL {
				return &errors.ValidationError{
					Field:   "provider.catalog.endpoint.path",
					Value:   p.Catalog.Endpoint.Path,
					Message: "must be the catalog endpoint URL suffix when deriving the offering base URL",
				}
			}
		}
	}
	if err := ValidateProviderFieldMappings(p.Catalog.Endpoint.FieldMappings); err != nil {
		return err
	}
	return nil
}

// ProviderAPIKey represents configuration for an API key to access a provider's catalog.
type ProviderAPIKey struct {
	Name       string               `json:"name" yaml:"name"`               // Name of the API key parameter
	Pattern    string               `json:"pattern" yaml:"pattern"`         // Glob pattern to match the API key
	Header     string               `json:"header" yaml:"header"`           // Header name to send the API key in
	Scheme     ProviderAPIKeyScheme `json:"scheme" yaml:"scheme"`           // Authentication scheme (e.g., "Bearer", "Basic", or empty for direct value)
	QueryParam string               `json:"query_param" yaml:"query_param"` // Query parameter name to send the API key in
}

// ProviderEnvVar represents an environment variable required by a provider.
type ProviderEnvVar struct {
	Name        string `json:"name" yaml:"name"`                                   // Environment variable name
	Required    bool   `json:"required" yaml:"required"`                           // Whether this env var is required
	Description string `json:"description,omitempty" yaml:"description,omitempty"` // Human-readable description
	Pattern     string `json:"pattern,omitempty" yaml:"pattern,omitempty"`         // Optional validation pattern
}

// ProviderAPIKeyScheme represents different authentication schemes for API keys.
type ProviderAPIKeyScheme string

// String returns the string representation of a ProviderAPIKeyScheme.
func (paks ProviderAPIKeyScheme) String() string {
	return string(paks)
}

// API key authentication schemes.
const (
	ProviderAPIKeySchemeBearer ProviderAPIKeyScheme = "Bearer" // Bearer token authentication (OAuth 2.0 style)
	ProviderAPIKeySchemeBasic  ProviderAPIKeyScheme = "Basic"  // Basic authentication
	ProviderAPIKeySchemeDirect ProviderAPIKeyScheme = ""       // Direct value (no scheme prefix)
)

// ProviderChatCompletions represents configuration for chat completions API.
type ProviderChatCompletions struct {
	URL              *string                   `json:"url,omitempty" yaml:"url,omitempty"`                             // Chat completions API endpoint URL
	HealthAPIURL     *string                   `json:"health_api_url,omitempty" yaml:"health_api_url,omitempty"`       // URL to health/status API for this service
	HealthComponents []ProviderHealthComponent `json:"health_components,omitempty" yaml:"health_components,omitempty"` // Specific components to monitor for chat completions
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
	ProviderIDAlibabaQwen    ProviderID = "alibaba"
	ProviderIDAlibabaCloud   ProviderID = "alibaba"
	ProviderIDAnthropic      ProviderID = "anthropic"
	ProviderIDAnyscale       ProviderID = "anyscale"
	ProviderIDCerebras       ProviderID = "cerebras"
	ProviderIDCheckstep      ProviderID = "checkstep"
	ProviderIDCohere         ProviderID = "cohere"
	ProviderIDCursor         ProviderID = "cursor"
	ProviderIDConectys       ProviderID = "conectys"
	ProviderIDCove           ProviderID = "cove"
	ProviderIDDeepMind       ProviderID = "deepmind"
	ProviderIDDeepInfra      ProviderID = "deepinfra"
	ProviderIDDeepSeek       ProviderID = "deepseek"
	ProviderIDFireworksAI    ProviderID = "fireworks-ai"
	ProviderIDGoogleAIStudio ProviderID = "google-ai-studio"
	ProviderIDGoogleVertex   ProviderID = "google-vertex"
	ProviderIDGroq           ProviderID = "groq"
	ProviderIDHuggingFace    ProviderID = "huggingface"
	ProviderIDMeta           ProviderID = "meta"
	ProviderIDMicrosoft      ProviderID = "microsoft"
	ProviderIDMistralAI      ProviderID = "mistral"
	ProviderIDNVIDIA         ProviderID = "nvidia"
	ProviderIDDatabricks     ProviderID = "databricks"
	ProviderIDSnowflake      ProviderID = "snowflake"
	ProviderIDWatsonx        ProviderID = "watsonx"
	ProviderIDOCI            ProviderID = "oracle-oci-generative-ai"
	ProviderIDCloudflare     ProviderID = "cloudflare-workers-ai"
	ProviderIDSambaNova      ProviderID = "sambanova"
	ProviderIDBaseten        ProviderID = "baseten"
	ProviderIDScaleway       ProviderID = "scaleway"
	ProviderIDHyperbolic     ProviderID = "hyperbolic"
	ProviderIDNovita         ProviderID = "novita"
	ProviderIDMoonshotAI     ProviderID = "moonshot-ai"
	ProviderIDOpenAI         ProviderID = "openai"
	ProviderIDOpenRouter     ProviderID = "openrouter"
	ProviderIDPerplexity     ProviderID = "perplexity"
	ProviderIDReplicate      ProviderID = "replicate"
	ProviderIDSafetyKit      ProviderID = "safetykit"
	ProviderIDTogetherAI     ProviderID = "together"
	ProviderIDVirtuousAI     ProviderID = "virtuousai"
	ProviderIDWebPurify      ProviderID = "webpurify"
	ProviderIDXAI            ProviderID = "xai"
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

// IsAPIKeyRequired checks if a provider requires an API key.
func (p *Provider) IsAPIKeyRequired() bool {
	return p.Catalog != nil && p.Catalog.Endpoint.AuthRequired
}

// ProviderValidationStatus represents the validation status of a provider.
type ProviderValidationStatus string

const (
	// ProviderValidationStatusConfigured indicates the provider is properly configured and ready to use.
	ProviderValidationStatusConfigured ProviderValidationStatus = "configured"
	// ProviderValidationStatusMissing indicates the provider is missing required API key configuration.
	ProviderValidationStatusMissing ProviderValidationStatus = "missing"
	// ProviderValidationStatusOptional indicates the provider has optional API key that is not configured (still usable).
	ProviderValidationStatusOptional ProviderValidationStatus = "optional"
	// ProviderValidationStatusUnsupported indicates the provider doesn't have client implementation yet.
	ProviderValidationStatusUnsupported ProviderValidationStatus = "unsupported"
)

// String returns the string representation of ProviderValidationStatus.
func (pvs ProviderValidationStatus) String() string {
	return string(pvs)
}

// ProviderValidationResult contains the result of validating a provider.
type ProviderValidationResult struct {
	Status             ProviderValidationStatus `json:"status"`
	HasAPIKey          bool                     `json:"has_api_key"`
	IsAPIKeyRequired   bool                     `json:"is_api_key_required"`
	HasRequiredEnvVars bool                     `json:"has_required_env_vars"`
	MissingEnvVars     []string                 `json:"missing_env_vars,omitempty"`
	IsConfigured       bool                     `json:"is_configured"`
	IsSupported        bool                     `json:"is_supported"`
	Error              error                    `json:"error,omitempty"`
}

// LoadAPIKey loads the API key value from environment into the provider.
// This should be called when the provider is loaded from the catalog.
func (p *Provider) LoadAPIKey() {
	if p.APIKey != nil {
		p.apiKeyValue = os.Getenv(p.APIKey.Name)
	}
}

// LoadEnvVars loads environment variable values from the system into the provider.
// This should be called when the provider is loaded from the catalog.
func (p *Provider) LoadEnvVars() {
	if len(p.EnvVars) == 0 {
		return
	}

	if p.EnvVarValues == nil {
		p.EnvVarValues = make(map[string]string)
	}

	for _, envVar := range p.EnvVars {
		p.EnvVarValues[envVar.Name] = os.Getenv(envVar.Name)
	}
}

// APIKeyValue retrieves and validates the API key for this provider.
// Uses the loaded apiKeyValue if available, otherwise falls back to environment.
func (p *Provider) APIKeyValue() (string, error) {
	if p.APIKey == nil {
		return "", nil
	}

	// Use loaded value or get from environment
	apiKey := p.apiKeyValue
	if apiKey == "" {
		apiKey = os.Getenv(p.APIKey.Name)
	}

	if apiKey == "" {
		// Check if API key is required
		if p.IsAPIKeyRequired() {
			return "", &errors.ConfigError{
				Component: string(p.ID),
				Message:   fmt.Sprintf("environment variable %s not set", p.APIKey.Name),
			}
		}
		return "", nil
	}

	// Validate against pattern if specified
	if p.APIKey.Pattern != "" && p.APIKey.Pattern != ".*" {
		matched, err := regexp.MatchString(p.APIKey.Pattern, apiKey)
		if err != nil {
			return "", errors.WrapParse("regex", p.APIKey.Pattern, err)
		}
		if !matched {
			return "", &errors.ValidationError{
				Field:   "api_key",
				Message: fmt.Sprintf("API key does not match required pattern for provider %s", p.ID),
			}
		}
	}

	return apiKey, nil
}

// EnvVar returns the value of a specific environment variable.
func (p *Provider) EnvVar(name string) string {
	if p.EnvVarValues != nil {
		if value, exists := p.EnvVarValues[name]; exists {
			return value
		}
	}
	// Fallback to direct environment lookup
	return os.Getenv(name)
}

// CatalogEndpointURL returns the resolved model catalog endpoint URL.
func (p *Provider) CatalogEndpointURL() string {
	if p == nil || p.Catalog == nil {
		return ""
	}

	endpoint := p.Catalog.Endpoint
	if endpoint.BaseURLEnvVar != "" {
		if baseURL := strings.TrimSpace(p.EnvVar(endpoint.BaseURLEnvVar)); baseURL != "" {
			return joinEndpointURL(baseURL, endpoint.Path)
		}
	}

	return endpoint.URL
}

// CatalogOfferingEndpoint returns the effective inference endpoint contract.
// When configuration omits an explicit offering base URL, it derives the base
// from the same resolved catalog endpoint used for acquisition. Runtime base
// URL overrides therefore affect acquisition and publication together.
func (p *Provider) CatalogOfferingEndpoint() ProviderOfferingEndpoint {
	if p == nil || p.Catalog == nil || p.Catalog.Offering == nil {
		return ProviderOfferingEndpoint{}
	}
	endpoint := p.Catalog.Offering.Endpoint
	if endpoint.BaseURL == "" {
		endpoint.BaseURL = catalogBaseURL(p.CatalogEndpointURL(), p.Catalog.Endpoint.Path)
	}
	return endpoint
}

func catalogBaseURL(endpointURL, endpointPath string) string {
	endpointURL = strings.TrimRight(strings.TrimSpace(endpointURL), "/")
	endpointPath = "/" + strings.Trim(strings.TrimSpace(endpointPath), "/")
	if endpointPath == "/" || !strings.HasSuffix(endpointURL, endpointPath) {
		return endpointURL
	}
	return strings.TrimSuffix(endpointURL, endpointPath)
}

func joinEndpointURL(baseURL, endpointPath string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpointPath = strings.TrimLeft(strings.TrimSpace(endpointPath), "/")
	if endpointPath == "" {
		return baseURL
	}
	return baseURL + "/" + endpointPath
}

// HasRequiredEnvVars checks if all required environment variables are set.
func (p *Provider) HasRequiredEnvVars() bool {
	for _, envVar := range p.EnvVars {
		if envVar.Required {
			value := p.EnvVar(envVar.Name)
			if value == "" {
				return false
			}

			// Validate against pattern if specified
			if envVar.Pattern != "" && envVar.Pattern != ".*" {
				matched, err := regexp.MatchString(envVar.Pattern, value)
				if err != nil || !matched {
					return false
				}
			}
		}
	}
	return true
}

// MissingRequiredEnvVars returns a list of required environment variables that are not set.
func (p *Provider) MissingRequiredEnvVars() []string {
	var missing []string
	for _, envVar := range p.EnvVars {
		if envVar.Required {
			value := p.EnvVar(envVar.Name)
			if value == "" {
				missing = append(missing, envVar.Name)
				continue
			}

			// Check pattern validation
			if envVar.Pattern != "" && envVar.Pattern != ".*" {
				matched, err := regexp.MatchString(envVar.Pattern, value)
				if err != nil || !matched {
					missing = append(missing, envVar.Name)
				}
			}
		}
	}
	return missing
}

// HasAPIKey checks if the provider has a valid API key configured.
// This checks both existence and validation (pattern matching).
func (p *Provider) HasAPIKey() bool {
	apiKey, err := p.APIKeyValue()
	return err == nil && apiKey != ""
}

// Validate performs validation checks on this provider and returns the result.
// The supportedProviders parameter is a set of provider IDs that have client implementations.
func (p *Provider) Validate(supportedProviders map[ProviderID]bool) ProviderValidationResult {
	result := ProviderValidationResult{
		HasAPIKey:          p.HasAPIKey(),
		IsAPIKeyRequired:   p.IsAPIKeyRequired(),
		HasRequiredEnvVars: p.HasRequiredEnvVars(),
		MissingEnvVars:     p.MissingRequiredEnvVars(),
		IsSupported:        supportedProviders[p.ID],
	}

	// Provider is configured if it has all required auth (API key and/or env vars)
	result.IsConfigured = true
	if result.IsAPIKeyRequired && !result.HasAPIKey {
		result.IsConfigured = false
	}
	if len(result.MissingEnvVars) > 0 {
		result.IsConfigured = false
	}

	// Check if provider has client implementation
	if !result.IsSupported {
		result.Status = ProviderValidationStatusUnsupported
		return result
	}

	// Determine status based on configuration
	if result.IsConfigured {
		// Validate API key format if present and required
		if result.IsAPIKeyRequired && result.HasAPIKey {
			_, err := p.APIKeyValue()
			if err != nil {
				result.Error = err
				result.Status = ProviderValidationStatusMissing
				return result
			}
		}
		result.Status = ProviderValidationStatusConfigured
	} else {
		// Check what's missing
		var missingParts []string
		if result.IsAPIKeyRequired && !result.HasAPIKey {
			missingParts = append(missingParts, fmt.Sprintf("API key %s", p.APIKey.Name))
		}
		if len(result.MissingEnvVars) > 0 {
			missingParts = append(missingParts, fmt.Sprintf("environment variables: %v", result.MissingEnvVars))
		}

		if len(missingParts) > 0 {
			result.Error = &errors.ConfigError{
				Component: string(p.ID),
				Message:   fmt.Sprintf("missing required configuration: %v", missingParts),
			}
			result.Status = ProviderValidationStatusMissing
		} else {
			// No auth required at all
			result.Status = ProviderValidationStatusOptional
		}
	}

	return result
}

// Model retrieves a specific model from the provider.
func (p *Provider) Model(modelID string) (*Model, error) {
	if p == nil || p.Models == nil {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Message: "provider or models not initialized",
		}
	}

	model, exists := p.Models[modelID]
	if !exists {
		return nil, &errors.NotFoundError{
			Resource: "model",
			ID:       modelID,
		}
	}

	return model, nil
}
