package catalogs

import (
	"github.com/agentstation/utc"
)

// Author represents a known model author or organization.
type Author struct {
	ID          AuthorID `json:"id" yaml:"id"`                                       // Unique identifier for the author
	Name        string   `json:"name" yaml:"name"`                                   // Display name of the author
	Description *string  `json:"description,omitempty" yaml:"description,omitempty"` // Description of what the author is known for

	// Website, social links, and other relevant URLs
	Website     *string `json:"website,omitempty" yaml:"website,omitempty"`         // Official website URL
	HuggingFace *string `json:"huggingface,omitempty" yaml:"huggingface,omitempty"` // Hugging Face profile/organization URL
	GitHub      *string `json:"github,omitempty" yaml:"github,omitempty"`           // GitHub profile/organization URL
	Twitter     *string `json:"twitter,omitempty" yaml:"twitter,omitempty"`         // X (formerly Twitter) profile URL

	// Catalog and modelss
	Catalog *AuthorCatalog   `json:"catalog,omitempty" yaml:"catalog,omitempty"` // Primary provider catalog for this author's models
	Models  map[string]Model // Models published by this author

	// Timestamps for record keeping and auditing
	CreatedAt utc.Time `json:"created_at" yaml:"created_at"` // Created date (YYYY-MM or YYYY-MM-DD format)
	UpdatedAt utc.Time `json:"updated_at" yaml:"updated_at"` // Last updated date (YYYY-MM or YYYY-MM-DD format)
}

// AuthorCatalog represents the relationship between an author and their authoritative provider catalog.
// This specifies which provider catalog is the primary source for an author's models and how to identify them.
type AuthorCatalog struct {
	ProviderID  ProviderID `json:"provider_id" yaml:"provider_id"`                     // Which provider's catalog contains this author's models
	Patterns    []string   `json:"patterns,omitempty" yaml:"patterns,omitempty"`       // Optional glob patterns to match model IDs (if provider hosts multiple authors)
	Description *string    `json:"description,omitempty" yaml:"description,omitempty"` // Optional description of this mapping relationship
}

// AuthorID is a unique identifier for an author.
type AuthorID string

// String returns the string representation of an AuthorID.
func (id AuthorID) String() string {
	return string(id)
}

// Author ID constants for compile-time safety and consistency.
const (
	// Major AI Companies
	AuthorIDOpenAI      AuthorID = "openai"
	AuthorIDAnthropic   AuthorID = "anthropic"
	AuthorIDGoogle      AuthorID = "google"
	AuthorIDDeepMind    AuthorID = "deepmind"
	AuthorIDMeta        AuthorID = "meta"
	AuthorIDMicrosoft   AuthorID = "microsoft"
	AuthorIDMistralAI   AuthorID = "mistral"
	AuthorIDCohere      AuthorID = "cohere"
	AuthorIDCerebras    AuthorID = "cerebras"
	AuthorIDAlibabaQwen AuthorID = "alibaba"
	AuthorIDQwen        AuthorID = "qwen"
	AuthorIDXAI         AuthorID = "xai"

	// Research Institutions
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

	// Open Source Communities & Platforms
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

	// Chinese Organizations
	AuthorIDBaidu      AuthorID = "baidu"
	AuthorIDTencent    AuthorID = "tencent"
	AuthorIDByteDance  AuthorID = "bytedance"
	AuthorIDDeepSeek   AuthorID = "deepseek"
	AuthorIDBAAI       AuthorID = "baai"
	AuthorID01AI       AuthorID = "01.ai"
	AuthorIDBaichuan   AuthorID = "baichuan"
	AuthorIDMiniMax    AuthorID = "minimax"
	AuthorIDMoonshot   AuthorID = "moonshot"
	AuthorIDShanghaiAI AuthorID = "shanghai-ai-lab"
	AuthorIDZhipuAI    AuthorID = "zhipu-ai"
	AuthorIDSenseTime  AuthorID = "sensetime"
	AuthorIDHuawei     AuthorID = "huawei"
	AuthorIDTsinghua   AuthorID = "tsinghua"
	AuthorIDPeking     AuthorID = "peking"

	// Other Notable Organizations
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

	// Notable Fine-Tuned Model Creators & Publishers
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
)
