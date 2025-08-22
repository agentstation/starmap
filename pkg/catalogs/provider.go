package catalogs

import (
	"fmt"
	"os"
	"regexp"
	"time"
)

// Provider represents a provider configuration.
type Provider struct {
	// Core identification and integration
	ID           ProviderID `json:"id" yaml:"id"`                                         // Unique provider identifier
	Name         string     `json:"name" yaml:"name"`                                     // Display name (must not be empty)
	Headquarters *string    `json:"headquarters,omitempty" yaml:"headquarters,omitempty"` // Company headquarters location
	IconURL      *string    `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`         // Provider icon/logo URL

	// API key configuration
	APIKey *ProviderAPIKey `json:"api_key,omitempty" yaml:"api_key,omitempty"` // API key configuration

	// Environment variables configuration
	EnvVars []ProviderEnvVar `json:"env_vars,omitempty" yaml:"env_vars,omitempty"` // Required environment variables

	// Models
	Catalog *ProviderCatalog `json:"catalog,omitempty" yaml:"catalog,omitempty"` // Models catalog configuration
	Authors []AuthorID       `json:"authors,omitempty" yaml:"authors,omitempty"` // List of authors to fetch from (for providers like Google Vertex AI)
	Models  map[string]Model `json:"-" yaml:"-"` // Available models indexed by model ID - not serialized to YAML

	// Status & Health
	StatusPageURL   *string                  `json:"status_page_url,omitempty" yaml:"status_page_url,omitempty"`   // Link to service status page
	ChatCompletions *ProviderChatCompletions `json:"chat_completions,omitempty" yaml:"chat_completions,omitempty"` // Chat completions API configuration

	// Privacy, Retention, and Governance Policies
	PrivacyPolicy    *ProviderPrivacyPolicy    `json:"privacy_policy,omitempty" yaml:"privacy_policy,omitempty"`       // Data collection and usage practices
	RetentionPolicy  *ProviderRetentionPolicy  `json:"retention_policy,omitempty" yaml:"retention_policy,omitempty"`   // Data retention and deletion practices
	GovernancePolicy *ProviderGovernancePolicy `json:"governance_policy,omitempty" yaml:"governance_policy,omitempty"` // Oversight and moderation practices

	// Runtime fields (not serialized)
	APIKeyValue string            `json:"-" yaml:"-"` // Actual API key value loaded from environment
	EnvVarValues map[string]string `json:"-" yaml:"-"` // Actual environment variable values loaded at runtime
}

// ProviderCatalog represents information about a provider's models.
type ProviderCatalog struct {
	DocsURL        *string `json:"docs_url,omitempty" yaml:"docs_url,omitempty"`                 // Models API documentation URL
	APIURL         *string `json:"api_url,omitempty" yaml:"api_url,omitempty"`                   // Models API endpoint URL
	APIKeyRequired *bool   `json:"api_key_required,omitempty" yaml:"api_key_required,omitempty"` // Whether the provider requires an API key to access the catalog
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
	Name        string `json:"name" yaml:"name"`                                 // Environment variable name
	Required    bool   `json:"required" yaml:"required"`                         // Whether this env var is required
	Description string `json:"description,omitempty" yaml:"description,omitempty"` // Human-readable description
	Pattern     string `json:"pattern,omitempty" yaml:"pattern,omitempty"`       // Optional validation pattern
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
	ProviderIDAnthropic      ProviderID = "anthropic"
	ProviderIDAnyscale       ProviderID = "anyscale"
	ProviderIDCerebras       ProviderID = "cerebras"
	ProviderIDCheckstep      ProviderID = "checkstep"
	ProviderIDCohere         ProviderID = "cohere"
	ProviderIDConectys       ProviderID = "conectys"
	ProviderIDCove           ProviderID = "cove"
	ProviderIDDeepMind       ProviderID = "deepmind"
	ProviderIDDeepSeek       ProviderID = "deepseek"
	ProviderIDGoogleAIStudio ProviderID = "google-ai-studio"
	ProviderIDGoogleVertex   ProviderID = "google-vertex"
	ProviderIDGroq           ProviderID = "groq"
	ProviderIDHuggingFace    ProviderID = "huggingface"
	ProviderIDMeta           ProviderID = "meta"
	ProviderIDMicrosoft      ProviderID = "microsoft"
	ProviderIDMistralAI      ProviderID = "mistral"
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
	// AI Platform Aggregators/Moderators
	ProviderModeratorAnyscale    ProviderModerator = "anyscale"
	ProviderModeratorHuggingFace ProviderModerator = "huggingface"
	ProviderModeratorOpenRouter  ProviderModerator = "openrouter"
	ProviderModeratorReplicate   ProviderModerator = "replicate"
	ProviderModeratorTogetherAI  ProviderModerator = "together"

	// Specialized AI Safety/Moderation Companies
	ProviderModeratorCheckstep  ProviderModerator = "checkstep"
	ProviderModeratorConectys   ProviderModerator = "conectys"
	ProviderModeratorCove       ProviderModerator = "cove"
	ProviderModeratorSafetyKit  ProviderModerator = "safetykit"
	ProviderModeratorVirtuousAI ProviderModerator = "virtuousai"
	ProviderModeratorWebPurify  ProviderModerator = "webpurify"

	// Self-Moderated (Major AI Companies)
	ProviderModeratorAnthropic      ProviderModerator = "anthropic"
	ProviderModeratorGoogleAIStudio ProviderModerator = "google-ai-studio"
	ProviderModeratorGoogleVertex   ProviderModerator = "google-vertex"
	ProviderModeratorGroq           ProviderModerator = "groq"
	ProviderModeratorMicrosoft      ProviderModerator = "microsoft"
	ProviderModeratorOpenAI         ProviderModerator = "openai"

	// Unknown/Unspecified
	ProviderModeratorUnknown ProviderModerator = "unknown"
)

// IsAPIKeyRequired checks if a provider requires an API key.
func (p *Provider) IsAPIKeyRequired() bool {
	return p.Catalog != nil &&
		p.Catalog.APIKeyRequired != nil &&
		*p.Catalog.APIKeyRequired
}

// ProviderValidationStatus represents the validation status of a provider.
type ProviderValidationStatus string

const (
	// Provider is properly configured and ready to use
	ProviderValidationStatusConfigured ProviderValidationStatus = "configured"
	// Provider is missing required API key configuration
	ProviderValidationStatusMissing ProviderValidationStatus = "missing"
	// Provider has optional API key that is not configured (still usable)
	ProviderValidationStatusOptional ProviderValidationStatus = "optional"
	// Provider doesn't have client implementation yet
	ProviderValidationStatusUnsupported ProviderValidationStatus = "unsupported"
)

// String returns the string representation of ProviderValidationStatus.
func (pvs ProviderValidationStatus) String() string {
	return string(pvs)
}

// ProviderValidationResult contains the result of validating a provider.
type ProviderValidationResult struct {
	Status          ProviderValidationStatus `json:"status"`
	HasAPIKey       bool                     `json:"has_api_key"`
	IsAPIKeyRequired bool                    `json:"is_api_key_required"`
	HasRequiredEnvVars bool                  `json:"has_required_env_vars"`
	MissingEnvVars  []string                 `json:"missing_env_vars,omitempty"`
	IsConfigured    bool                     `json:"is_configured"`
	IsSupported     bool                     `json:"is_supported"`
	Error           error                    `json:"error,omitempty"`
}

// LoadAPIKey loads the API key value from environment into the provider.
// This should be called when the provider is loaded from the catalog.
func (p *Provider) LoadAPIKey() {
	if p.APIKey != nil {
		p.APIKeyValue = os.Getenv(p.APIKey.Name)
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

// GetAPIKeyValue returns the loaded API key value.
func (p *Provider) GetAPIKeyValue() string {
	return p.APIKeyValue
}

// GetEnvVar returns the value of a specific environment variable.
func (p *Provider) GetEnvVar(name string) string {
	if p.EnvVarValues != nil {
		if value, exists := p.EnvVarValues[name]; exists {
			return value
		}
	}
	// Fallback to direct environment lookup
	return os.Getenv(name)
}

// HasRequiredEnvVars checks if all required environment variables are set.
func (p *Provider) HasRequiredEnvVars() bool {
	for _, envVar := range p.EnvVars {
		if envVar.Required {
			value := p.GetEnvVar(envVar.Name)
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

// GetMissingEnvVars returns a list of required environment variables that are not set.
func (p *Provider) GetMissingEnvVars() []string {
	var missing []string
	for _, envVar := range p.EnvVars {
		if envVar.Required {
			value := p.GetEnvVar(envVar.Name)
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

// HasAPIKey checks if the provider has an API key configured.
// This checks the loaded APIKeyValue for efficiency.
func (p *Provider) HasAPIKey() bool {
	if p.APIKey == nil {
		return false
	}
	// Use loaded value if available, otherwise check environment
	if p.APIKeyValue != "" {
		return true
	}
	return os.Getenv(p.APIKey.Name) != ""
}

// GetAPIKey retrieves and validates the API key for this provider.
// Uses the loaded APIKeyValue if available, otherwise falls back to environment.
func (p *Provider) GetAPIKey() (string, error) {
	if p.APIKey == nil {
		return "", nil
	}

	// Use loaded value or get from environment
	apiKey := p.APIKeyValue
	if apiKey == "" {
		apiKey = os.Getenv(p.APIKey.Name)
	}

	if apiKey == "" {
		// Check if API key is required
		if p.IsAPIKeyRequired() {
			return "", fmt.Errorf("environment variable %s not set", p.APIKey.Name)
		}
		return "", nil
	}

	// Validate against pattern if specified
	if p.APIKey.Pattern != "" && p.APIKey.Pattern != ".*" {
		matched, err := regexp.MatchString(p.APIKey.Pattern, apiKey)
		if err != nil {
			return "", fmt.Errorf("invalid pattern %s: %w", p.APIKey.Pattern, err)
		}
		if !matched {
			return "", fmt.Errorf("API key does not match required pattern for provider %s", p.ID)
		}
	}

	return apiKey, nil
}

// Validate performs validation checks on this provider and returns the result.
// The supportedProviders parameter is a set of provider IDs that have client implementations.
func (p *Provider) Validate(supportedProviders map[ProviderID]bool) ProviderValidationResult {
	result := ProviderValidationResult{
		HasAPIKey:          p.HasAPIKey(),
		IsAPIKeyRequired:   p.IsAPIKeyRequired(),
		HasRequiredEnvVars: p.HasRequiredEnvVars(),
		MissingEnvVars:     p.GetMissingEnvVars(),
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
			_, err := p.GetAPIKey()
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
			result.Error = fmt.Errorf("missing required configuration: %s", fmt.Sprintf("%v", missingParts))
			result.Status = ProviderValidationStatusMissing
		} else {
			// No auth required at all
			result.Status = ProviderValidationStatusOptional
		}
	}

	return result
}

// ClientGetterFunc is a function type for getting a client from the registry.
type ClientGetterFunc func(*Provider) (Client, error)

// clientGetter is the injected function for getting clients from the registry.
var clientGetter ClientGetterFunc

// SetClientGetter sets the function used to retrieve clients from the registry.
// This is called by the registry package to inject the lookup function.
func SetClientGetter(getter ClientGetterFunc) {
	clientGetter = getter
}

// ClientOption is a function type for configuring client options.
type ClientOption func(*ClientOptions)

// ClientOptions configures how a client is retrieved for a provider.
type ClientOptions struct {
	AllowMissingAPIKey bool
}

// ClientResult contains the result of getting a provider client.
type ClientResult struct {
	Client         Client
	APIKeyRequired bool
	APIKeyPresent  bool
	Error          error
}

// WithAllowMissingAPIKey allows retrieving a client even if the API key is missing.
func WithAllowMissingAPIKey(allow bool) ClientOption {
	return func(opts *ClientOptions) {
		opts.AllowMissingAPIKey = allow
	}
}

// GetClient retrieves a configured client for this provider.
func (p *Provider) GetClient(opts ...ClientOption) (*ClientResult, error) {
	// Apply options
	options := &ClientOptions{}
	for _, opt := range opts {
		opt(options)
	}

	result := &ClientResult{
		APIKeyRequired: p.IsAPIKeyRequired(),
		APIKeyPresent:  p.HasAPIKey(),
	}

	// Check if API key is required but missing
	if result.APIKeyRequired && !result.APIKeyPresent && !options.AllowMissingAPIKey {
		result.Error = fmt.Errorf("provider %s requires API key %s but it is not configured", p.ID, p.APIKey.Name)
		return result, nil
	}

	// Check if required environment variables are missing
	missingEnvVars := p.GetMissingEnvVars()
	if len(missingEnvVars) > 0 && !options.AllowMissingAPIKey {
		result.Error = fmt.Errorf("provider %s is missing required environment variables: %v", p.ID, missingEnvVars)
		return result, nil
	}

	// Use the injected client getter function
	if clientGetter == nil {
		result.Error = fmt.Errorf("client registry not initialized")
		return result, nil
	}

	client, err := clientGetter(p)
	if err != nil {
		result.Error = err
		return result, nil
	}

	result.Client = client
	return result, nil
}

