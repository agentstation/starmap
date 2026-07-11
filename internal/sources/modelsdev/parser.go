package modelsdev

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
	"github.com/agentstation/starmap/pkg/sources"
)

// API represents the structure of models.dev api.json.
type API map[string]Provider

// Provider represents a provider in models.dev.
type Provider struct {
	ID            string                           `json:"id"`
	Env           []string                         `json:"env"`
	NPM           string                           `json:"npm"`
	API           *string                          `json:"api,omitempty"`
	Name          string                           `json:"name"`
	Doc           string                           `json:"doc"`
	Models        map[string]Model                 `json:"models"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
}

// UnmarshalJSON retains fingerprints for additive provider fields.
func (p *Provider) UnmarshalJSON(data []byte) error {
	type providerAlias Provider
	var decoded providerAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "provider")
	if err != nil {
		return err
	}
	*p = Provider(decoded)
	p.UnknownFields = unknown
	return nil
}

// Model represents a model in models.dev.
type Model struct {
	ID               string                           `json:"id"`
	Name             string                           `json:"name"`
	Description      string                           `json:"description"`
	Family           string                           `json:"family"`
	Status           string                           `json:"status,omitempty"`
	Attachment       bool                             `json:"attachment"`
	Reasoning        bool                             `json:"reasoning"`
	ReasoningOptions []ReasoningOption                `json:"reasoning_options,omitempty"`
	StructuredOutput bool                             `json:"structured_output"`
	Temperature      bool                             `json:"temperature"`
	ToolCall         bool                             `json:"tool_call"`
	Knowledge        *string                          `json:"knowledge,omitempty"`
	Provider         *ModelProvider                   `json:"provider,omitempty"`
	Interleaved      *Interleaved                     `json:"interleaved,omitempty"`
	ReleaseDate      string                           `json:"release_date"`
	LastUpdated      string                           `json:"last_updated"`
	Modalities       Modalities                       `json:"modalities"`
	OpenWeights      *bool                            `json:"open_weights,omitempty"`
	Cost             *Cost                            `json:"cost,omitempty"`
	Limit            Limit                            `json:"limit"`
	Experimental     *Experimental                    `json:"experimental,omitempty"`
	UnknownFields    []sourcepayload.UnknownJSONField `json:"-"`
}

// UnmarshalJSON retains fingerprints for additive model fields.
func (m *Model) UnmarshalJSON(data []byte) error {
	type modelAlias Model
	var decoded modelAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "models[]")
	if err != nil {
		return err
	}
	*m = Model(decoded)
	m.UnknownFields = unknown
	return nil
}

// Modalities represents input/output modalities.
type Modalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// ReasoningOption represents a configurable reasoning option in models.dev.
type ReasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values"`
	Min    *int     `json:"min,omitempty"`
	Max    *int     `json:"max,omitempty"`
}

// ModelProvider represents model-level provider invocation metadata.
type ModelProvider struct {
	NPM   string `json:"npm,omitempty"`
	API   string `json:"api,omitempty"`
	Shape string `json:"shape,omitempty"`
}

// Interleaved represents models.dev interleaved reasoning response metadata.
type Interleaved struct {
	Enabled bool
	Field   string
}

// Cost represents pricing information.
type Cost struct {
	Input           *float64    `json:"input,omitempty"`
	Output          *float64    `json:"output,omitempty"`
	Reasoning       *float64    `json:"reasoning,omitempty"`
	Cache           *float64    `json:"cache,omitempty"`       // Legacy cache field
	CacheRead       *float64    `json:"cache_read,omitempty"`  // Cache read costs
	CacheWrite      *float64    `json:"cache_write,omitempty"` // Cache write costs
	InputAudio      *float64    `json:"input_audio,omitempty"`
	OutputAudio     *float64    `json:"output_audio,omitempty"`
	Tiers           []CostTier  `json:"tiers,omitempty"`
	ContextOver200K *TierPrices `json:"context_over_200k,omitempty"`
}

// CostTier represents a conditional pricing tier in models.dev.
type CostTier struct {
	TierPrices
	Tier CostTierInfo `json:"tier"`
}

// CostTierInfo represents the dimension and threshold for a models.dev pricing tier.
type CostTierInfo struct {
	Type string `json:"type"`
	Size int64  `json:"size"`
}

// TierPrices represents prices that may appear in a pricing tier.
type TierPrices struct {
	Input      *float64 `json:"input,omitempty"`
	Output     *float64 `json:"output,omitempty"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
	InputAudio *float64 `json:"input_audio,omitempty"`
}

// Experimental represents experimental models.dev model metadata.
type Experimental struct {
	Enabled bool                        `json:"-"`
	Modes   map[string]ExperimentalMode `json:"modes,omitempty"`
}

// ExperimentalMode represents a mode-specific models.dev override.
type ExperimentalMode struct {
	Cost     *TierPrices               `json:"cost,omitempty"`
	Provider *ExperimentalModeProvider `json:"provider,omitempty"`
}

// ExperimentalModeProvider represents provider request overrides for a mode.
type ExperimentalModeProvider struct {
	Headers map[string]string `json:"headers,omitempty"`
	Body    map[string]any    `json:"body,omitempty"`
}

// UnmarshalJSON accepts both the legacy boolean marker and the current object
// form containing mode overrides.
func (e *Experimental) UnmarshalJSON(data []byte) error {
	if strings.EqualFold(strings.TrimSpace(string(data)), "null") {
		return nil
	}

	var enabled bool
	if err := json.Unmarshal(data, &enabled); err == nil {
		e.Enabled = enabled
		return nil
	}

	type experimental Experimental
	var object experimental
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("parse experimental metadata: %w", err)
	}
	*e = Experimental(object)
	e.Enabled = true
	return nil
}

// Limit represents model limits.
type Limit struct {
	Context int `json:"context"`
	Input   int `json:"input"`
	Output  int `json:"output"`
}

const modelsDevExtensionSource = "models.dev"

// UnmarshalJSON accepts both boolean and object forms of models.dev interleaved metadata.
func (i *Interleaved) UnmarshalJSON(data []byte) error {
	if strings.EqualFold(strings.TrimSpace(string(data)), "null") {
		return nil
	}

	var enabled bool
	if err := json.Unmarshal(data, &enabled); err == nil {
		i.Enabled = enabled
		return nil
	}

	var object struct {
		Field string `json:"field,omitempty"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("parse interleaved metadata: %w", err)
	}
	i.Enabled = true
	i.Field = object.Field
	return nil
}

// ParseAPI parses the api.json file and returns an API.
func ParseAPI(apiPath string) (*API, error) {
	data, err := os.ReadFile(apiPath) //nolint:gosec // Input paths are controlled by internal tooling
	if err != nil {
		return nil, errors.WrapIO("read", apiPath, err)
	}
	return parseAPIData(data)
}

func parseAPIData(data []byte) (*API, error) {
	if err := sources.ValidateJSONPayload(data); err != nil {
		return nil, err
	}
	var api API
	if err := json.Unmarshal(data, &api); err != nil {
		return nil, errors.WrapParse("json", "api.json", err)
	}

	return &api, nil
}

// ToStarmapProvider converts a Provider to a starmap.Provider.
func (p *Provider) ToStarmapProvider() (*catalogs.Provider, error) {
	provider := p.toStarmapProviderMetadata()

	// Convert models
	if len(p.Models) > 0 {
		provider.Models = make(map[string]*catalogs.Model)
		for modelID, model := range p.Models {
			starmapModel, err := model.ToStarmapModel()
			if err != nil {
				return nil, errors.WrapResource("convert", "model", modelID, err)
			}
			provider.Models[modelID] = starmapModel
		}
	}

	return provider, nil
}

func (p *Provider) toStarmapProviderMetadata() *catalogs.Provider {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderID(p.ID),
		Name: p.Name,
	}
	if p.Doc != "" {
		provider.Catalog = &catalogs.ProviderCatalog{}
		doc := p.Doc
		provider.Catalog.Docs = &doc
	}
	if len(p.Env) > 0 {
		provider.EnvVars = make([]catalogs.ProviderEnvVar, 0, len(p.Env))
		for _, envName := range p.Env {
			if envName == "" {
				continue
			}
			provider.EnvVars = append(provider.EnvVars, catalogs.ProviderEnvVar{
				Name:     envName,
				Required: false,
			})
		}
	}
	fields := make(map[string]any)
	if p.NPM != "" || p.API != nil {
		if p.NPM != "" {
			fields["npm"] = p.NPM
		}
		if p.API != nil && *p.API != "" {
			fields["api"] = *p.API
		}
	}
	if len(p.UnknownFields) > 0 {
		fields["unknown_fields"] = p.UnknownFields
	}
	if len(fields) > 0 {
		provider.Extensions = catalogs.SourceExtensions{modelsDevExtensionSource: {Fields: catalogs.NormalizeExtensionFields(fields)}}
	}
	return provider
}

// ToStarmapModel converts a Model to a starmap.Model.
func (m *Model) ToStarmapModel() (*catalogs.Model, error) {
	model := &catalogs.Model{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Status:      convertModelStatus(m.Status),
	}

	applyModelsDevLineage(model, m.Family)
	if m.OpenWeights != nil {
		model.Metadata = &catalogs.ModelMetadata{
			OpenWeights: *m.OpenWeights,
		}
	}
	applyModelsDevFeatures(model, m)
	model.Limits = convertModelLimits(m.Limit)
	model.Pricing = convertModelPricing(m.Cost)
	model.Modes = convertExperimentalModes(m.Experimental)
	model.Extensions = convertModelsDevExtensions(m)
	applyModelsDevDates(model, m)

	return model, nil
}

func applyModelsDevLineage(model *catalogs.Model, family string) {
	if family == "" {
		return
	}
	model.Lineage = &catalogs.ModelLineage{
		Family: family,
	}
}

func applyModelsDevFeatures(model *catalogs.Model, source *Model) {
	if !hasFeatureData(source) {
		return
	}

	model.Features = &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  convertModalities(source.Modalities.Input),
			Output: convertModalities(source.Modalities.Output),
		},
		Temperature:       source.Temperature,
		ToolCalls:         source.ToolCall,
		Tools:             source.ToolCall,
		ToolChoice:        source.ToolCall,
		Reasoning:         source.Reasoning,
		Attachments:       source.Attachment,
		StructuredOutputs: source.StructuredOutput,
	}

	if levels := convertReasoningLevels(source.ReasoningOptions); len(levels) > 0 {
		model.Features.ReasoningEffort = true
		model.Reasoning = &catalogs.ModelControlLevels{
			Levels: levels,
		}
	}
	if tokenRange := convertReasoningTokenRange(source.ReasoningOptions); tokenRange != nil {
		model.Features.ReasoningTokens = true
		model.ReasoningTokens = tokenRange
	}
}

func convertModelLimits(limit Limit) *catalogs.ModelLimits {
	if limit.Context == 0 && limit.Input == 0 && limit.Output == 0 {
		return nil
	}
	return &catalogs.ModelLimits{
		ContextWindow: int64(limit.Context),
		InputTokens:   int64(limit.Input),
		OutputTokens:  int64(limit.Output),
	}
}

func convertModelPricing(cost *Cost) *catalogs.ModelPricing {
	if cost == nil {
		return nil
	}

	pricing := &catalogs.ModelPricing{
		Currency: catalogs.ModelPricingCurrencyUSD, // models.dev uses USD
		Tokens:   convertModelTokenPricing(cost),
		Tiers:    convertPricingTiers(cost),
	}

	if cost.InputAudio != nil || cost.OutputAudio != nil {
		pricing.Operations = &catalogs.ModelOperationPricing{}
		if cost.InputAudio != nil {
			pricing.Operations.AudioInput = cost.InputAudio
		}
		if cost.OutputAudio != nil {
			pricing.Operations.AudioGen = cost.OutputAudio
		}
	}

	return pricing
}

func convertModelTokenPricing(cost *Cost) *catalogs.ModelTokenPricing {
	tokenPricing := &catalogs.ModelTokenPricing{}

	if cost.Input != nil {
		tokenPricing.Input = &catalogs.ModelTokenCost{
			Per1M: *cost.Input,
		}
	}
	if cost.Output != nil {
		tokenPricing.Output = &catalogs.ModelTokenCost{
			Per1M: *cost.Output,
		}
	}
	if cost.Reasoning != nil {
		tokenPricing.Reasoning = &catalogs.ModelTokenCost{
			Per1M: *cost.Reasoning,
		}
	}
	tokenPricing.Cache = convertModelTokenCachePricing(cost)
	return tokenPricing
}

func convertModelTokenCachePricing(cost *Cost) *catalogs.ModelTokenCachePricing {
	if cost.CacheRead == nil && cost.CacheWrite == nil && cost.Cache == nil {
		return nil
	}

	cacheCost := &catalogs.ModelTokenCachePricing{}
	if cost.CacheRead != nil {
		cacheCost.Read = &catalogs.ModelTokenCost{
			Per1M: *cost.CacheRead,
		}
	}
	if cost.CacheWrite != nil {
		cacheCost.Write = &catalogs.ModelTokenCost{
			Per1M: *cost.CacheWrite,
		}
	}
	// Legacy fallback: if no specific cache_read/cache_write, use cache for write.
	if cost.Cache != nil && cacheCost.Read == nil && cacheCost.Write == nil {
		cacheCost.Write = &catalogs.ModelTokenCost{
			Per1M: *cost.Cache,
		}
	}
	return cacheCost
}

func applyModelsDevDates(model *catalogs.Model, source *Model) {
	if source.ReleaseDate != "" {
		if releaseDate, err := parseDate(source.ReleaseDate); err == nil {
			ensureModelsDevMetadata(model).ReleaseDate = utc.Time{Time: *releaseDate}
		}
	}
	if source.LastUpdated != "" {
		if lastUpdated, err := parseDate(source.LastUpdated); err == nil {
			model.UpdatedAt = utc.Time{Time: *lastUpdated}
		}
	}
	if source.Knowledge != nil && *source.Knowledge != "" {
		if knowledgeDate, err := parseDate(*source.Knowledge); err == nil {
			knowledgeCutoff := utc.Time{Time: *knowledgeDate}
			ensureModelsDevMetadata(model).KnowledgeCutoff = &knowledgeCutoff
		}
	}
}

func ensureModelsDevMetadata(model *catalogs.Model) *catalogs.ModelMetadata {
	if model.Metadata == nil {
		model.Metadata = &catalogs.ModelMetadata{}
	}
	return model.Metadata
}

// convertModalities converts string modalities to starmap.ModelModality.
func convertModalities(modalities []string) []catalogs.ModelModality {
	var result []catalogs.ModelModality
	for _, modality := range modalities {
		switch strings.ToLower(modality) {
		case "text":
			result = append(result, catalogs.ModelModalityText)
		case "image":
			result = append(result, catalogs.ModelModalityImage)
		case "audio":
			result = append(result, catalogs.ModelModalityAudio)
		case "video":
			result = append(result, catalogs.ModelModalityVideo)
		case "pdf":
			result = append(result, catalogs.ModelModalityPDF)
		case "embedding":
			result = append(result, catalogs.ModelModalityEmbedding)
		}
	}
	return result
}

func hasFeatureData(model *Model) bool {
	return len(model.Modalities.Input) > 0 ||
		len(model.Modalities.Output) > 0 ||
		model.Temperature ||
		model.ToolCall ||
		model.Reasoning ||
		model.Attachment ||
		model.StructuredOutput ||
		len(model.ReasoningOptions) > 0
}

func (m Model) hasCatalogData() bool {
	return m.Description != "" ||
		m.Family != "" ||
		m.Status != "" ||
		m.ReleaseDate != "" ||
		m.OpenWeights != nil ||
		(m.Knowledge != nil && *m.Knowledge != "") ||
		hasFeatureData(&m) ||
		m.hasCostData() ||
		m.Limit.Context > 0 ||
		m.Limit.Input > 0 ||
		m.Limit.Output > 0 ||
		m.hasModelProviderData() ||
		m.Interleaved != nil ||
		m.hasExperimentalData()
}

func (m Model) hasCostData() bool {
	if m.Cost == nil {
		return false
	}
	return m.Cost.Input != nil ||
		m.Cost.Output != nil ||
		m.Cost.Reasoning != nil ||
		m.Cost.Cache != nil ||
		m.Cost.CacheRead != nil ||
		m.Cost.CacheWrite != nil ||
		m.Cost.InputAudio != nil ||
		m.Cost.OutputAudio != nil ||
		len(m.Cost.Tiers) > 0 ||
		m.Cost.ContextOver200K != nil
}

func (m Model) hasExperimentalData() bool {
	return m.Experimental != nil && (m.Experimental.Enabled || len(m.Experimental.Modes) > 0)
}

func (m Model) hasModelProviderData() bool {
	return m.Provider != nil && (m.Provider.NPM != "" || m.Provider.API != "" || m.Provider.Shape != "")
}

func convertExperimentalModes(experimental *Experimental) map[string]catalogs.ModelMode {
	if experimental == nil || len(experimental.Modes) == 0 {
		return nil
	}
	modes := make(map[string]catalogs.ModelMode, len(experimental.Modes))
	for name, sourceMode := range experimental.Modes {
		mode := catalogs.ModelMode{}
		if sourceMode.Cost != nil {
			mode.Pricing = &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
				Tokens:   tierTokenPricing(*sourceMode.Cost),
			}
		}
		if sourceMode.Provider != nil {
			mode.Provider = &catalogs.ModelProviderMode{
				Headers: copyStringMap(sourceMode.Provider.Headers),
				Body:    catalogs.NormalizeExtensionFields(sourceMode.Provider.Body),
			}
		}
		modes[name] = mode
	}
	return modes
}

func copyStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	copied := make(map[string]string, len(input))
	maps.Copy(copied, input)
	return copied
}

func convertPricingTiers(cost *Cost) []catalogs.ModelPricingTier {
	if cost == nil {
		return nil
	}
	tiers := make([]catalogs.ModelPricingTier, 0, len(cost.Tiers)+1)
	for _, sourceTier := range cost.Tiers {
		tier := catalogs.ModelPricingTier{
			Type:   catalogs.ModelPricingTierType(sourceTier.Tier.Type),
			Size:   sourceTier.Tier.Size,
			Tokens: tierTokenPricing(sourceTier.TierPrices),
		}
		if sourceTier.InputAudio != nil {
			tier.Operations = &catalogs.ModelOperationPricing{
				AudioInput: sourceTier.InputAudio,
			}
		}
		tiers = append(tiers, tier)
	}
	if cost.ContextOver200K != nil {
		tiers = append(tiers, catalogs.ModelPricingTier{
			Name:   "context_over_200k",
			Type:   catalogs.ModelPricingTierTypeContext,
			Size:   200000,
			Tokens: tierTokenPricing(*cost.ContextOver200K),
		})
	}
	return tiers
}

func tierTokenPricing(prices TierPrices) *catalogs.ModelTokenPricing {
	pricing := &catalogs.ModelTokenPricing{}
	if prices.Input != nil {
		pricing.Input = &catalogs.ModelTokenCost{Per1M: *prices.Input}
	}
	if prices.Output != nil {
		pricing.Output = &catalogs.ModelTokenCost{Per1M: *prices.Output}
	}
	if prices.CacheRead != nil || prices.CacheWrite != nil {
		pricing.Cache = &catalogs.ModelTokenCachePricing{}
		if prices.CacheRead != nil {
			pricing.Cache.Read = &catalogs.ModelTokenCost{Per1M: *prices.CacheRead}
		}
		if prices.CacheWrite != nil {
			pricing.Cache.Write = &catalogs.ModelTokenCost{Per1M: *prices.CacheWrite}
		}
	}
	if pricing.Input == nil && pricing.Output == nil && pricing.Cache == nil {
		return nil
	}
	return pricing
}

func convertModelStatus(status string) catalogs.ModelStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return ""
	case "active":
		return catalogs.ModelStatusActive
	case "beta":
		return catalogs.ModelStatusBeta
	case "preview":
		return catalogs.ModelStatusPreview
	case "deprecated":
		return catalogs.ModelStatusDeprecated
	default:
		return catalogs.ModelStatusUnknown
	}
}

func convertModelsDevExtensions(model *Model) catalogs.SourceExtensions {
	fields := make(map[string]any)
	unknown := append([]sourcepayload.UnknownJSONField(nil), model.UnknownFields...)
	if model.Status != "" && convertModelStatus(model.Status) == catalogs.ModelStatusUnknown {
		unknown = append(unknown, sourcepayload.FingerprintValue("status", model.Status))
	}
	if len(unknown) > 0 {
		fields["unknown_fields"] = unknown
	}
	if model.Provider != nil {
		providerFields := make(map[string]any)
		if model.Provider.NPM != "" {
			providerFields["npm"] = model.Provider.NPM
		}
		if model.Provider.API != "" {
			providerFields["api"] = model.Provider.API
		}
		if model.Provider.Shape != "" {
			providerFields["shape"] = model.Provider.Shape
		}
		if len(providerFields) > 0 {
			fields["provider"] = providerFields
		}
	}
	if model.Interleaved != nil {
		interleavedFields := map[string]any{
			"enabled": model.Interleaved.Enabled,
		}
		if model.Interleaved.Field != "" {
			interleavedFields["field"] = model.Interleaved.Field
		}
		fields["interleaved"] = interleavedFields
	}
	if options := nonCanonicalReasoningOptions(model.ReasoningOptions); len(options) > 0 {
		fields["reasoning_options"] = options
	}
	if model.Experimental != nil && model.Experimental.Enabled && len(model.Experimental.Modes) == 0 {
		fields["experimental"] = true
	}
	if len(fields) == 0 {
		return nil
	}
	return catalogs.SourceExtensions{
		modelsDevExtensionSource: {
			Fields: catalogs.NormalizeExtensionFields(fields),
		},
	}
}

func convertReasoningLevels(options []ReasoningOption) []catalogs.ModelControlLevel {
	for _, option := range options {
		if !strings.EqualFold(option.Type, "effort") {
			continue
		}

		levels := make([]catalogs.ModelControlLevel, 0, len(option.Values))
		for _, value := range option.Values {
			if level, ok := convertReasoningLevel(value); ok {
				levels = append(levels, level)
			}
		}
		return levels
	}
	return nil
}

func convertReasoningTokenRange(options []ReasoningOption) *catalogs.IntRange {
	for _, option := range options {
		if !isReasoningTokenOption(option.Type) {
			continue
		}
		if option.Min == nil && option.Max == nil {
			continue
		}
		tokenRange := &catalogs.IntRange{}
		if option.Min != nil {
			tokenRange.Min = *option.Min
		}
		if option.Max != nil {
			tokenRange.Max = *option.Max
		}
		return tokenRange
	}
	return nil
}

func nonCanonicalReasoningOptions(options []ReasoningOption) []any {
	preserved := make([]any, 0)
	for _, option := range options {
		if isReasoningTokenOption(option.Type) {
			continue
		}
		values := append([]string(nil), option.Values...)
		if strings.EqualFold(option.Type, "effort") {
			values = nonCanonicalReasoningEffortValues(option.Values)
			if len(values) == 0 && option.Min == nil && option.Max == nil {
				continue
			}
		}
		fields := map[string]any{
			"type": option.Type,
		}
		if len(values) > 0 {
			fields["values"] = values
		}
		if option.Min != nil {
			fields["min"] = *option.Min
		}
		if option.Max != nil {
			fields["max"] = *option.Max
		}
		preserved = append(preserved, fields)
	}
	return preserved
}

func nonCanonicalReasoningEffortValues(values []string) []string {
	nonCanonical := make([]string, 0)
	for _, value := range values {
		if _, ok := convertReasoningLevel(value); ok {
			continue
		}
		nonCanonical = append(nonCanonical, value)
	}
	return nonCanonical
}

func isReasoningTokenOption(optionType string) bool {
	switch strings.ToLower(strings.TrimSpace(optionType)) {
	case "budget_tokens", "tokens", "reasoning_tokens":
		return true
	default:
		return false
	}
}

func convertReasoningLevel(value string) (catalogs.ModelControlLevel, bool) {
	switch strings.ToLower(value) {
	case "minimal", "minimum":
		return catalogs.ModelControlLevelMinimum, true
	case "low":
		return catalogs.ModelControlLevelLow, true
	case "medium":
		return catalogs.ModelControlLevelMedium, true
	case "high":
		return catalogs.ModelControlLevelHigh, true
	case "max", "maximum":
		return catalogs.ModelControlLevelMaximum, true
	case "none":
		return "", false
	default:
		return "", false
	}
}

// parseDate parses various date formats used in models.dev.
func parseDate(dateStr string) (*time.Time, error) {
	// Try different date formats
	formats := []string{
		"2006-01-02",           // YYYY-MM-DD
		"2006-01",              // YYYY-MM
		"2006",                 // YYYY
		time.RFC3339,           // ISO 8601 with timezone
		"2006-01-02T15:04:05Z", // ISO 8601 UTC
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t, nil
		}
	}

	// If all parsing fails, try to extract year from the string
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil && year > 1900 && year < 3000 {
			t := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
			return &t, nil
		}
	}

	return nil, errors.WrapParse("date", dateStr, errors.New("unsupported format"))
}

// GetProvider returns a specific provider from the API data.
func (api *API) GetProvider(providerID catalogs.ProviderID) (*Provider, bool) {
	provider, exists := (*api)[string(providerID)]
	return &provider, exists
}

// Model returns a specific model from a provider.
func (p *Provider) Model(modelID string) (*Model, bool) {
	model, exists := p.Models[modelID]
	return &model, exists
}
