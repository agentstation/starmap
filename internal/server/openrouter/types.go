// Package openrouter adapts Starmap's immutable catalog read model to the
// OpenRouter model and endpoint discovery HTTP contracts.
package openrouter

// ModelEnvelope is the successful single-model response envelope.
type ModelEnvelope struct {
	Data Model `json:"data"`
}

// EndpointsEnvelope is the successful model-endpoints response envelope.
type EndpointsEnvelope struct {
	Data Endpoints `json:"data"`
}

// ErrorEnvelope is the OpenRouter-compatible error response envelope.
type ErrorEnvelope struct {
	Error Error `json:"error"`
}

// Error is the numeric OpenRouter-compatible error payload.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Model is the OpenRouter-compatible provider-independent model response.
type Model struct {
	Architecture        Architecture   `json:"architecture"`
	CanonicalSlug       string         `json:"canonical_slug"`
	ContextLength       int64          `json:"context_length"`
	Created             int64          `json:"created"`
	DefaultParameters   map[string]any `json:"default_parameters"`
	Description         string         `json:"description"`
	ExpirationDate      *string        `json:"expiration_date"`
	ID                  string         `json:"id"`
	KnowledgeCutoff     *string        `json:"knowledge_cutoff"`
	Links               ModelLinks     `json:"links"`
	Name                string         `json:"name"`
	PerRequestLimits    map[string]any `json:"per_request_limits"`
	Pricing             *Pricing       `json:"pricing"`
	Reasoning           *Reasoning     `json:"reasoning,omitempty"`
	SupportedParameters []string       `json:"supported_parameters"`
	SupportedVoices     []string       `json:"supported_voices"`
	TopProvider         *TopProvider   `json:"top_provider"`
}

// Endpoints is the OpenRouter-compatible response for all eligible provider
// offerings of one canonical model.
type Endpoints struct {
	Architecture Architecture `json:"architecture"`
	Created      int64        `json:"created"`
	Description  string       `json:"description"`
	Endpoints    []Endpoint   `json:"endpoints"`
	ID           string       `json:"id"`
	Name         string       `json:"name"`
}

// Architecture describes model input/output behavior using OpenRouter field
// names without changing Starmap's canonical catalog types.
type Architecture struct {
	InputModalities  []string `json:"input_modalities"`
	InstructType     *string  `json:"instruct_type"`
	Modality         string   `json:"modality"`
	OutputModalities []string `json:"output_modalities"`
	Tokenizer        string   `json:"tokenizer"`
}

// ModelLinks contains related OpenRouter-compatible discovery routes.
type ModelLinks struct {
	Details string `json:"details"`
}

// Pricing contains USD prices in OpenRouter's per-token, per-request, or
// per-operation string units. Nil fields are omitted rather than invented.
type Pricing struct {
	Prompt            *string `json:"prompt,omitempty"`
	Completion        *string `json:"completion,omitempty"`
	Request           *string `json:"request,omitempty"`
	Image             *string `json:"image,omitempty"`
	WebSearch         *string `json:"web_search,omitempty"`
	InternalReasoning *string `json:"internal_reasoning,omitempty"`
	InputCacheRead    *string `json:"input_cache_read,omitempty"`
	InputCacheWrite   *string `json:"input_cache_write,omitempty"`
}

// Reasoning describes supported reasoning controls.
type Reasoning struct {
	DefaultEffort    *string  `json:"default_effort"`
	DefaultEnabled   bool     `json:"default_enabled"`
	Mandatory        bool     `json:"mandatory"`
	SupportedEfforts []string `json:"supported_efforts"`
}

// TopProvider summarizes the deterministic preferred eligible offering.
type TopProvider struct {
	ContextLength       *int64 `json:"context_length"`
	IsModerated         *bool  `json:"is_moderated"`
	MaxCompletionTokens *int64 `json:"max_completion_tokens"`
}

// Endpoint is one provider's serving record for a canonical model.
type Endpoint struct {
	ContextLength           *int64       `json:"context_length"`
	LatencyLast30m          *Percentiles `json:"latency_last_30m,omitempty"`
	MaxCompletionTokens     *int64       `json:"max_completion_tokens"`
	MaxPromptTokens         *int64       `json:"max_prompt_tokens"`
	ModelID                 string       `json:"model_id"`
	ModelName               string       `json:"model_name"`
	Name                    string       `json:"name"`
	Pricing                 *Pricing     `json:"pricing,omitempty"`
	ProviderName            string       `json:"provider_name"`
	Quantization            string       `json:"quantization"`
	Status                  int          `json:"status"`
	SupportedParameters     []string     `json:"supported_parameters"`
	SupportsImplicitCaching bool         `json:"supports_implicit_caching"`
	Tag                     string       `json:"tag"`
	ThroughputLast30m       *Percentiles `json:"throughput_last_30m,omitempty"`
	UptimeLast1d            *float64     `json:"uptime_last_1d,omitempty"`
	UptimeLast30m           *float64     `json:"uptime_last_30m,omitempty"`
	UptimeLast5m            *float64     `json:"uptime_last_5m,omitempty"`
}

// Percentiles is the optional runtime latency or throughput summary accepted
// by the OpenRouter endpoint contract. Starmap omits it without real samples.
type Percentiles struct {
	P50 float64 `json:"p50"`
	P75 float64 `json:"p75"`
	P90 float64 `json:"p90"`
	P99 float64 `json:"p99"`
}
