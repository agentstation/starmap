package catalogs

import (
	"strings"
	"sync"

	"github.com/agentstation/utc"
)

// Author represents a known model author or organization.
type Author struct {
	ID          AuthorID   `json:"id" yaml:"id"`                                       // Unique identifier for the author
	Aliases     []AuthorID `json:"aliases,omitempty" yaml:"aliases,omitempty"`         // Alternative IDs this author is known by (e.g., in provider catalogs)
	Name        string     `json:"name" yaml:"name"`                                   // Display name of the author
	Description *string    `json:"description,omitempty" yaml:"description,omitempty"` // Description of what the author is known for

	// Company/organization info
	Headquarters *string `json:"headquarters,omitempty" yaml:"headquarters,omitempty"` // Company headquarters location
	IconURL      *string `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`         // Author icon/logo URL

	// Website, social links, and other relevant URLs
	Website     *string `json:"website,omitempty" yaml:"website,omitempty"`         // Official website URL
	HuggingFace *string `json:"huggingface,omitempty" yaml:"huggingface,omitempty"` // Hugging Face profile/organization URL
	GitHub      *string `json:"github,omitempty" yaml:"github,omitempty"`           // GitHub profile/organization URL
	Twitter     *string `json:"twitter,omitempty" yaml:"twitter,omitempty"`         // X (formerly Twitter) profile URL

	// Catalog and models
	Catalog *AuthorCatalog    `json:"catalog,omitempty" yaml:"catalog,omitempty"` // Primary provider catalog for this author's models
	Models  map[string]*Model `json:"-" yaml:"-"`                                 // Models published by this author - not serialized

	// Timestamps for record keeping and auditing
	CreatedAt utc.Time `json:"created_at" yaml:"created_at"` // Created date (YYYY-MM or YYYY-MM-DD format)
	UpdatedAt utc.Time `json:"updated_at" yaml:"updated_at"` // Last updated date (YYYY-MM or YYYY-MM-DD format)
}

// AuthorCatalog represents the relationship between an author and their authoritative provider catalog.
// This contains the attribution configuration for identifying the author's models across providers.
type AuthorCatalog struct {
	Description *string            `json:"description,omitempty" yaml:"description,omitempty"` // Optional description of this mapping relationship
	Attribution *AuthorAttribution `json:"attribution,omitempty" yaml:"attribution,omitempty"` // Model attribution configuration for multi-provider inference
}

// AuthorAttribution defines how to identify an author's models across providers.
// Uses standard Go glob pattern syntax for case-insensitive model ID matching.
//
// Supports three modes:
//  1. Provider-only: provider_id set, no patterns - all models from that provider belong to this author
//  2. Provider + patterns: provider_id + patterns - only matching models from that provider, then cross-provider attribution
//  3. Global patterns: patterns only - direct case-insensitive pattern matching across all providers
//
// Glob Pattern Syntax (case-insensitive):
//   - matches any sequence of characters (except path separators)
//     ? matches any single character
//     [abc] matches any character in the set
//     [a-z] matches any character in the range
//
// Examples:
//
//	"llama*" matches llama-3, Llama3.1-8b, LLAMA-BIG
//	"*-llama-*" matches deepseek-r1-distill-llama-70b, DeepSeek-R1-Distill-LLAMA-70B
//	"gpt-*" matches gpt-4, GPT-3.5-turbo, Gpt-4o
type AuthorAttribution struct {
	ProviderID ProviderID `json:"provider_id,omitempty" yaml:"provider_id,omitempty"` // Optional provider to source models from
	Patterns   []string   `json:"patterns,omitempty" yaml:"patterns,omitempty"`       // Glob patterns to match model IDs
}

// AuthorID is a unique identifier for an author.
type AuthorID string

// String returns the string representation of an AuthorID.
func (id AuthorID) String() string {
	return string(id)
}

// Author ID constants for compile-time safety and consistency.
const (
	// Major AI Companies.
	AuthorIDOpenAI    AuthorID = "openai"
	AuthorIDAnthropic AuthorID = "anthropic"
	AuthorIDGoogle    AuthorID = "google"
	AuthorIDDeepMind  AuthorID = "deepmind"
	AuthorIDMeta      AuthorID = "meta"
	AuthorIDMicrosoft AuthorID = "microsoft"
	AuthorIDMistralAI AuthorID = "mistral"
	AuthorIDCohere    AuthorID = "cohere"
	// AuthorIDCerebras removed - Cerebras is an inference provider, not a model creator.
	AuthorIDGroq        AuthorID = "groq"
	AuthorIDAlibabaQwen AuthorID = "alibaba"
	AuthorIDQwen        AuthorID = "qwen"
	AuthorIDXAI         AuthorID = "xai"

	// Research Institutions.
	AuthorIDStanford    AuthorID = "stanford"
	AuthorIDMIT         AuthorID = "mit"
	AuthorIDCMU         AuthorID = "cmu"
	AuthorIDUCBerkeley  AuthorID = "uc-berkeley"
	AuthorIDCornell     AuthorID = "cornell"
	AuthorIDPrinceton   AuthorID = "princeton"
	AuthorIDHarvard     AuthorID = "harvard"
	AuthorIDOxford      AuthorID = "oxford"
	AuthorIDCambridge   AuthorID = "cambridge"
	AuthorIDETHZurich   AuthorID = "eth-zurich"
	AuthorIDUWashington AuthorID = "uw"
	AuthorIDUChicago    AuthorID = "uchicago"
	AuthorIDYale        AuthorID = "yale"
	AuthorIDDuke        AuthorID = "duke"
	AuthorIDCaltech     AuthorID = "caltech"

	// Open Source Communities & Platforms.
	AuthorIDHuggingFace AuthorID = "huggingface"
	AuthorIDEleutherAI  AuthorID = "eleutherai"
	AuthorIDTogether    AuthorID = "together"
	AuthorIDMosaicML    AuthorID = "mosaicml"
	AuthorIDStabilityAI AuthorID = "stability"
	AuthorIDRunwayML    AuthorID = "runway"
	AuthorIDMidjourney  AuthorID = "midjourney"
	AuthorIDLAION       AuthorID = "laion"
	AuthorIDBigScience  AuthorID = "bigscience"
	AuthorIDAlignmentRC AuthorID = "alignment-research"
	AuthorIDH2OAI       AuthorID = "h2o.ai"
	AuthorIDMoxin       AuthorID = "moxin"

	// Chinese Organizations.
	AuthorIDBaidu      AuthorID = "baidu"
	AuthorIDTencent    AuthorID = "tencent"
	AuthorIDByteDance  AuthorID = "bytedance"
	AuthorIDDeepSeek   AuthorID = "deepseek"
	AuthorIDBAAI       AuthorID = "baai"
	AuthorID01AI       AuthorID = "01.ai"
	AuthorIDBaichuan   AuthorID = "baichuan"
	AuthorIDMiniMax    AuthorID = "minimax"
	AuthorIDMoonshot   AuthorID = "moonshotai"
	AuthorIDShanghaiAI AuthorID = "shanghai-ai-lab"
	AuthorIDZhipuAI    AuthorID = "zhipu-ai"
	AuthorIDSenseTime  AuthorID = "sensetime"
	AuthorIDHuawei     AuthorID = "huawei"
	AuthorIDTsinghua   AuthorID = "tsinghua"
	AuthorIDPeking     AuthorID = "peking"

	// Other Notable Organizations.
	AuthorIDNVIDIA     AuthorID = "nvidia"
	AuthorIDSalesforce AuthorID = "salesforce"
	AuthorIDIBM        AuthorID = "ibm"
	AuthorIDApple      AuthorID = "apple"
	AuthorIDAmazon     AuthorID = "amazon"
	AuthorIDAdept      AuthorID = "adept"
	AuthorIDAI21       AuthorID = "ai21"
	AuthorIDInflection AuthorID = "inflection"
	AuthorIDCharacter  AuthorID = "character"
	AuthorIDPerplexity AuthorID = "perplexity"
	AuthorIDAnysphere  AuthorID = "anysphere"
	AuthorIDCursor     AuthorID = "cursor"

	// Notable Fine-Tuned Model Creators & Publishers.
	AuthorIDCognitiveComputations AuthorID = "cognitivecomputations"
	AuthorIDEricHartford          AuthorID = "ehartford"
	AuthorIDNousResearch          AuthorID = "nousresearch"
	AuthorIDTeknium               AuthorID = "teknium"
	AuthorIDJonDurbin             AuthorID = "jondurbin"
	AuthorIDLMSYS                 AuthorID = "lmsys"
	AuthorIDVicuna                AuthorID = "vicuna-team"
	AuthorIDAlpacaTeam            AuthorID = "stanford-alpaca"
	AuthorIDWizardLM              AuthorID = "wizardlm"
	AuthorIDOpenOrca              AuthorID = "open-orca"
	AuthorIDPhind                 AuthorID = "phind"
	AuthorIDCodeFuse              AuthorID = "codefuse"
	AuthorIDTHUDM                 AuthorID = "thudm"
	AuthorIDGeorgiaTechRI         AuthorID = "gatech"
	AuthorIDFastChat              AuthorID = "fastchat"

	// Special constant for unknown authors.
	AuthorIDUnknown AuthorID = "unknown"
)

// ParseAuthorID attempts to parse a string into an AuthorID.
// It first tries to find the author by exact ID match, then by aliases,
// and finally normalizes the string if no match is found.
func ParseAuthorID(s string) AuthorID {
	if s == "" {
		return AuthorIDUnknown
	}

	// Try to get embedded catalog for lookup
	catalog := getEmbeddedCatalogSingleton()
	if catalog != nil {
		// First try exact ID match
		if author, found := catalog.Authors().Get(AuthorID(s)); found {
			return author.ID
		}

		// Then try aliases
		var foundAuthor *Author
		catalog.Authors().ForEach(func(_ AuthorID, author *Author) bool {
			for _, alias := range author.Aliases {
				if string(alias) == s {
					foundAuthor = author
					return false // Stop iteration
				}
			}
			return true // Continue iteration
		})

		if foundAuthor != nil {
			return foundAuthor.ID
		}
	}

	// If not found, normalize the string and return as AuthorID
	normalized := normalizeAuthorString(s)
	return AuthorID(normalized)
}

// normalizeAuthorString normalizes a string for use as an AuthorID.
// It converts to lowercase and trims whitespace.
func normalizeAuthorString(s string) string {
	// Convert to lowercase and trim spaces
	normalized := strings.ToLower(strings.TrimSpace(s))

	// Replace spaces with hyphens for consistency
	normalized = strings.ReplaceAll(normalized, " ", "-")

	return normalized
}

// Singleton for embedded catalog to avoid repeated loading.
var (
	embeddedCatalogOnce sync.Once
	embeddedCatalog     Catalog
)

// getEmbeddedCatalogSingleton returns a singleton instance of the embedded catalog.
// Returns nil if the catalog cannot be loaded.
func getEmbeddedCatalogSingleton() Catalog {
	embeddedCatalogOnce.Do(func() {
		if cat, err := NewEmbedded(); err == nil {
			embeddedCatalog = cat
		}
		// If error, embeddedCatalog remains nil
	})
	return embeddedCatalog
}
