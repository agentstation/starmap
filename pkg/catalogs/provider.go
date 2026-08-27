package catalogs

import (
	"fmt"
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
	Description  *string      `json:"description,omitempty" yaml:"description,omitempty"`   // Short description of the provider's inference service
	DocsURL      *string      `json:"docs_url,omitempty" yaml:"docs_url,omitempty"`         // Link to the provider's API documentation
	Headquarters *string      `json:"headquarters,omitempty" yaml:"headquarters,omitempty"` // Company headquarters location
	IconURL      *string      `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`         // Provider icon/logo URL

	// Logo holds the provider's SVG brand mark. The bytes travel in the JSON
	// catalog payload but stay out of providers.yaml; on a filesystem catalog
	// they live in the providers/<id>/logo.svg sidecar file.
	Logo []byte `json:"logo_svg,omitempty" yaml:"-"`

	// Secret-free credential metadata for catalog acquisition and inference.
	Credentials *ProviderCredentials `json:"credentials,omitempty" yaml:"credentials,omitempty"`

	// Models
	Catalog *ProviderCatalog  `json:"catalog,omitempty" yaml:"catalog,omitempty"` // Models catalog configuration
	Models  map[string]*Model `json:"-" yaml:"-"`                                 // Available models indexed by model ID - not serialized to YAML

	// Status & Health
	StatusPageURL *string            `json:"status_page_url,omitempty" yaml:"status_page_url,omitempty"` // Link to service status page
	Inference     *ProviderInference `json:"inference,omitempty" yaml:"inference,omitempty"`             // Provider inference service contract

	// Privacy, Retention, and Governance Policies
	PrivacyPolicy    *ProviderPrivacyPolicy    `json:"privacy_policy,omitempty" yaml:"privacy_policy,omitempty"`       // Data collection and usage practices
	RetentionPolicy  *ProviderRetentionPolicy  `json:"retention_policy,omitempty" yaml:"retention_policy,omitempty"`   // Data retention and deletion practices
	GovernancePolicy *ProviderGovernancePolicy `json:"governance_policy,omitempty" yaml:"governance_policy,omitempty"` // Oversight and moderation practices

	// Extensions - controlled source-specific fields that are not canonical schema
	Extensions SourceExtensions `json:"extensions,omitempty" yaml:"extensions,omitempty"`
}

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
	// EndpointTypeOllama represents the native Ollama API.
	EndpointTypeOllama EndpointType = "ollama"
)

// FieldMapping defines how to map API response fields to model fields.
// Type conversion is automatic based on the destination field type.
type FieldMapping struct {
	From string `yaml:"from" json:"from"` // Source field path in API response (e.g., "max_model_len")
	To   string `yaml:"to" json:"to"`     // Target field path in Model (e.g., "limits.context_window")
}

// ProviderCapabilityCombination defines how multiple source predicates prove
// one canonical capability.
type ProviderCapabilityCombination string

const (
	// ProviderCapabilityConflict accepts equal known values and rejects contradictions.
	ProviderCapabilityConflict ProviderCapabilityCombination = "conflict"
	// ProviderCapabilityFirstKnown selects the first present source in YAML order.
	ProviderCapabilityFirstKnown ProviderCapabilityCombination = "first-known"
	// ProviderCapabilityAny requires any known true, or all known false.
	ProviderCapabilityAny ProviderCapabilityCombination = "any"
	// ProviderCapabilityAll requires any known false, or all known true.
	ProviderCapabilityAll ProviderCapabilityCombination = "all"
)

// CapabilityMapping maps one typed provider predicate to each canonical fact
// that the cited provider contract entails.
type CapabilityMapping struct {
	From     string                        `yaml:"from" json:"from"`
	To       []ModelFeature                `yaml:"to" json:"to"`
	Combine  ProviderCapabilityCombination `yaml:"combine,omitempty" json:"combine,omitempty"`
	Evidence string                        `yaml:"evidence" json:"evidence"`
}

// AuthorMapping defines how to extract and normalize authors.
type AuthorMapping struct {
	Field      string              `yaml:"field" json:"field"`           // Field to extract from (e.g., "owned_by")
	Normalized map[string]AuthorID `yaml:"normalized" json:"normalized"` // Normalization map (e.g., "Meta" -> "meta")
}

// ProviderEndpoint configures how to access the provider's model catalog.
type ProviderEndpoint struct {
	Type               EndpointType                   `yaml:"type" json:"type"`                                                   // Required: API style
	URL                string                         `yaml:"url" json:"url"`                                                     // Required: API endpoint
	ProtocolOptions    ProviderCatalogProtocolOptions `yaml:"protocol_options,omitempty" json:"protocol_options,omitempty"`       // Typed wire-protocol facts
	FieldMappings      []FieldMapping                 `yaml:"field_mappings,omitempty" json:"field_mappings,omitempty"`           // Field mappings
	CapabilityMappings []CapabilityMapping            `yaml:"capability_mappings,omitempty" json:"capability_mappings,omitempty"` // Typed capability predicates
	AuthorMapping      *AuthorMapping                 `yaml:"author_mapping,omitempty" json:"author_mapping,omitempty"`           // Author extraction
}

// ProviderCatalogProtocolOptions is a typed union of catalog-transport facts.
type ProviderCatalogProtocolOptions struct {
	OpenAI    *ProviderOpenAICatalogProtocolOptions    `json:"openai,omitempty" yaml:"openai,omitempty"`
	Anthropic *ProviderAnthropicCatalogProtocolOptions `json:"anthropic,omitempty" yaml:"anthropic,omitempty"`
}

// ProviderTokenPriceUnit identifies the unit used by one provider payload.
type ProviderTokenPriceUnit string

const (
	// ProviderTokenPriceUnitPerToken means USD per token.
	// #nosec G101 -- This value identifies a price unit, not authentication material.
	ProviderTokenPriceUnitPerToken ProviderTokenPriceUnit = "usd-per-token"
	// ProviderTokenPriceUnitPerMillion means USD per one million tokens.
	// #nosec G101 -- This value identifies a price unit, not authentication material.
	ProviderTokenPriceUnitPerMillion ProviderTokenPriceUnit = "usd-per-million-tokens"
)

// ProviderOpenAICatalogProtocolOptions defines OpenAI-compatible payload facts.
type ProviderOpenAICatalogProtocolOptions struct {
	TokenPriceUnit ProviderTokenPriceUnit `json:"token_price_unit" yaml:"token_price_unit"`
}

// ProviderAnthropicCatalogProtocolOptions defines Anthropic wire-version facts.
type ProviderAnthropicCatalogProtocolOptions struct {
	Version string `json:"version" yaml:"version"`
}

// ProviderCatalog represents information about a provider's models.
type ProviderCatalog struct {
	Docs     *string          `yaml:"docs" json:"docs"`         // Documentation URL
	Endpoint ProviderEndpoint `yaml:"endpoint" json:"endpoint"` // API endpoint configuration
}

// ProviderOperation identifies one provider inference operation.
type ProviderOperation string

const (
	// ProviderOperationChatCompletions generates chat completions.
	ProviderOperationChatCompletions ProviderOperation = "chat-completions"
	// ProviderOperationEmbeddings generates vector embeddings.
	ProviderOperationEmbeddings ProviderOperation = "embeddings"
	// ProviderOperationImagesGenerations generates an image from a prompt.
	ProviderOperationImagesGenerations ProviderOperation = "images-generations"
	// ProviderOperationImagesEdits generates an image from a prompt and an image.
	ProviderOperationImagesEdits ProviderOperation = "images-edits"
	// ProviderOperationAudioSpeech generates speech from text.
	ProviderOperationAudioSpeech ProviderOperation = "audio-speech"
	// ProviderOperationAudioTranscriptions transcribes speech in its own language.
	ProviderOperationAudioTranscriptions ProviderOperation = "audio-transcriptions"
	// ProviderOperationAudioTranslations transcribes speech into English.
	ProviderOperationAudioTranslations ProviderOperation = "audio-translations"
	// ProviderOperationVideosGenerations generates a video from a prompt. The
	// provider answers with a job rather than a video, so a consumer submits,
	// polls, and collects.
	ProviderOperationVideosGenerations ProviderOperation = "videos-generations"
)

// ProviderInference defines stable provider-level inference service facts.
// Gateway consumers supply runtime endpoint overrides and inference credentials.
type ProviderInference struct {
	BaseURL          string                      `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	Endpoints        []ProviderInferenceEndpoint `json:"endpoints" yaml:"endpoints"`
	HealthAPIURL     *string                     `json:"health_api_url,omitempty" yaml:"health_api_url,omitempty"`
	HealthComponents []ProviderHealthComponent   `json:"health_components,omitempty" yaml:"health_components,omitempty"`
}

// ProviderInferenceEndpoint defines one operation path and wire protocol.
type ProviderInferenceEndpoint struct {
	Operation           ProviderOperation         `json:"operation" yaml:"operation"`
	Type                EndpointType              `json:"type" yaml:"type"`
	Path                string                    `json:"path" yaml:"path"`
	StreamPath          string                    `json:"stream_path,omitempty" yaml:"stream_path,omitempty"`
	ProtocolsByAuthor   map[AuthorID]EndpointType `json:"protocols_by_author,omitempty" yaml:"protocols_by_author,omitempty"`
	PathsByAuthor       map[AuthorID]string       `json:"paths_by_author,omitempty" yaml:"paths_by_author,omitempty"`
	StreamPathsByAuthor map[AuthorID]string       `json:"stream_paths_by_author,omitempty" yaml:"stream_paths_by_author,omitempty"`
}

// Endpoint returns the endpoint for an exact inference operation.
func (i *ProviderInference) Endpoint(operation ProviderOperation) (ProviderInferenceEndpoint, bool) {
	if i == nil {
		return ProviderInferenceEndpoint{}, false
	}
	for _, endpoint := range i.Endpoints {
		if endpoint.Operation == operation {
			return endpoint, true
		}
	}
	return ProviderInferenceEndpoint{}, false
}

// EndpointURL resolves an endpoint against a runtime base URL override.
func (i *ProviderInference) EndpointURL(endpoint ProviderInferenceEndpoint, baseURLOverride string) string {
	if i == nil {
		return ""
	}
	baseURL := strings.TrimSpace(baseURLOverride)
	if baseURL == "" {
		baseURL = i.BaseURL
	}
	return joinEndpointURL(baseURL, endpoint.Path)
}

var inferenceEndpointVariable = regexp.MustCompile(`\{[a-z][a-z0-9_]*\}`)

// BindOfferingEndpoint applies runtime endpoint bindings to one immutable
// offering endpoint. Catalog data owns URL templates. Consumers supply only
// tenant-specific values and an optional base URL override.
func (i *ProviderInference) BindOfferingEndpoint(
	endpoint ProviderOfferingEndpoint,
	baseURLOverride string,
	bindings map[string]string,
) (ProviderOfferingEndpoint, error) {
	if i == nil {
		return ProviderOfferingEndpoint{}, fmt.Errorf("provider inference service is required")
	}
	bound := endpoint
	var err error
	bound.URL, err = i.bindOfferingURL(endpoint.URL, baseURLOverride, bindings)
	if err != nil {
		return ProviderOfferingEndpoint{}, fmt.Errorf("bind %s endpoint: %w", endpoint.Operation, err)
	}
	if endpoint.StreamURL != "" {
		bound.StreamURL, err = i.bindOfferingURL(endpoint.StreamURL, baseURLOverride, bindings)
		if err != nil {
			return ProviderOfferingEndpoint{}, fmt.Errorf("bind %s stream endpoint: %w", endpoint.Operation, err)
		}
	}
	return bound, nil
}

func (i *ProviderInference) bindOfferingURL(
	endpointURL string,
	baseURLOverride string,
	bindings map[string]string,
) (string, error) {
	resolved := strings.TrimSpace(endpointURL)
	if resolved == "" {
		return "", fmt.Errorf("endpoint URL is required")
	}
	if override := strings.TrimRight(strings.TrimSpace(baseURLOverride), "/"); override != "" {
		catalogBase := strings.TrimRight(strings.TrimSpace(i.BaseURL), "/")
		switch {
		case catalogBase != "" && strings.HasPrefix(resolved, catalogBase):
			resolved = override + strings.TrimPrefix(resolved, catalogBase)
		case strings.HasPrefix(resolved, "/"):
			resolved = override + resolved
		default:
			return "", fmt.Errorf("endpoint URL cannot use the base URL override")
		}
	}
	for name, value := range bindings {
		resolved = strings.ReplaceAll(resolved, "{"+name+"}", value)
	}
	if variable := inferenceEndpointVariable.FindString(resolved); variable != "" {
		return "", fmt.Errorf("endpoint binding %s is required", variable)
	}
	return resolved, nil
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
	ProviderIDConectys       ProviderID = "conectys"
	ProviderIDCove           ProviderID = "cove"
	ProviderIDDeepMind       ProviderID = "deepmind"
	ProviderIDDeepInfra      ProviderID = "deepinfra"
	ProviderIDDeepSeek       ProviderID = "deepseek"
	ProviderIDFireworksAI    ProviderID = "fireworks-ai"
	ProviderIDGoogleAIStudio ProviderID = "google-ai-studio"
	ProviderIDGoogleVertex   ProviderID = "google-vertex"
	ProviderIDGroq           ProviderID = "groq"
	ProviderIDHetzner        ProviderID = "hetzner"
	ProviderIDHuggingFace    ProviderID = "huggingface"
	ProviderIDMeta           ProviderID = "meta"
	ProviderIDMicrosoft      ProviderID = "microsoft"
	ProviderIDMistralAI      ProviderID = "mistral"
	ProviderIDAzureOpenAI    ProviderID = "azure-openai"
	ProviderIDOllama         ProviderID = "ollama"
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
	Type     ProviderRetentionType `json:"type" yaml:"type"`                                                   // Type of retention policy
	Duration *time.Duration        `json:"duration,omitempty" yaml:"duration,omitempty" swaggertype:"integer"` // nil = forever, 0 = immediate deletion
	Details  *string               `json:"details,omitempty" yaml:"details,omitempty"`                         // Human-readable description
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

// IsCatalogAuthRequired reports whether catalog acquisition requires credentials.
func (p *Provider) IsCatalogAuthRequired() bool {
	return p != nil && p.Credentials != nil && p.Credentials.CatalogAcquisition.Required
}

// CatalogEndpointURL returns the resolved model catalog endpoint URL.
func (p *Provider) CatalogEndpointURL() string {
	if p == nil || p.Catalog == nil {
		return ""
	}
	return p.Catalog.Endpoint.URL
}

// BindCatalogEndpoint resolves catalog-declared endpoint variables.
func (p *Provider) BindCatalogEndpoint(bindings map[string]string) (string, error) {
	if p == nil || p.Catalog == nil {
		return "", &errors.ValidationError{
			Field: "provider.catalog.endpoint", Message: "is required",
		}
	}
	resolved := p.Catalog.Endpoint.URL
	for name, value := range bindings {
		resolved = strings.ReplaceAll(resolved, "{"+name+"}", value)
	}
	if variable := inferenceEndpointVariable.FindString(resolved); variable != "" {
		return "", &errors.ValidationError{
			Field: "provider.catalog.endpoint.url", Value: variable,
			Message: "endpoint binding is required",
		}
	}
	parsed, err := url.Parse(resolved)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.Host == "" || parsed.User != nil {
		return "", &errors.ValidationError{
			Field: "provider.catalog.endpoint.url", Value: resolved,
			Message: "must resolve to an HTTP or HTTPS URL",
		}
	}
	return resolved, nil
}

func joinEndpointURL(baseURL, endpointPath string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpointPath = strings.TrimLeft(strings.TrimSpace(endpointPath), "/")
	if endpointPath == "" {
		return baseURL
	}
	return baseURL + "/" + endpointPath
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
