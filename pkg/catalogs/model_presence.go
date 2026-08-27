package catalogs

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
)

// ModelFeature identifies one boolean model capability.
type ModelFeature string

// Model feature identifiers.
const (
	ModelFeatureToolCalls                     ModelFeature = "tool_calls"
	ModelFeatureTools                         ModelFeature = "tools"
	ModelFeatureToolChoice                    ModelFeature = "tool_choice"
	ModelFeatureWebSearch                     ModelFeature = "web_search"
	ModelFeatureAttachments                   ModelFeature = "attachments"
	ModelFeatureReasoning                     ModelFeature = "reasoning"
	ModelFeatureReasoningEffort               ModelFeature = "reasoning_effort"
	ModelFeatureReasoningTokens               ModelFeature = "reasoning_tokens"
	ModelFeatureIncludeReasoning              ModelFeature = "include_reasoning"
	ModelFeatureVerbosity                     ModelFeature = "verbosity"
	ModelFeatureTemperature                   ModelFeature = "temperature"
	ModelFeatureTopP                          ModelFeature = "top_p"
	ModelFeatureTopK                          ModelFeature = "top_k"
	ModelFeatureTopA                          ModelFeature = "top_a"
	ModelFeatureMinP                          ModelFeature = "min_p"
	ModelFeatureTypicalP                      ModelFeature = "typical_p"
	ModelFeatureTFS                           ModelFeature = "tfs"
	ModelFeatureMaxTokens                     ModelFeature = "max_tokens"
	ModelFeatureMaxOutputTokens               ModelFeature = "max_output_tokens"
	ModelFeatureStop                          ModelFeature = "stop"
	ModelFeatureStopTokenIDs                  ModelFeature = "stop_token_ids"
	ModelFeatureFrequencyPenalty              ModelFeature = "frequency_penalty"
	ModelFeaturePresencePenalty               ModelFeature = "presence_penalty"
	ModelFeatureRepetitionPenalty             ModelFeature = "repetition_penalty"
	ModelFeatureNoRepeatNgramSize             ModelFeature = "no_repeat_ngram_size"
	ModelFeatureLengthPenalty                 ModelFeature = "length_penalty"
	ModelFeatureLogitBias                     ModelFeature = "logit_bias"
	ModelFeatureBadWords                      ModelFeature = "bad_words"
	ModelFeatureAllowedTokens                 ModelFeature = "allowed_tokens"
	ModelFeatureSeed                          ModelFeature = "seed"
	ModelFeatureLogprobs                      ModelFeature = "logprobs"
	ModelFeatureTopLogprobs                   ModelFeature = "top_logprobs"
	ModelFeatureEcho                          ModelFeature = "echo"
	ModelFeatureN                             ModelFeature = "n"
	ModelFeatureBestOf                        ModelFeature = "best_of"
	ModelFeatureMirostat                      ModelFeature = "mirostat"
	ModelFeatureMirostatTau                   ModelFeature = "mirostat_tau"
	ModelFeatureMirostatEta                   ModelFeature = "mirostat_eta"
	ModelFeatureContrastiveSearchPenaltyAlpha ModelFeature = "contrastive_search_penalty_alpha"
	ModelFeatureNumBeams                      ModelFeature = "num_beams"
	ModelFeatureEarlyStopping                 ModelFeature = "early_stopping"
	ModelFeatureDiversityPenalty              ModelFeature = "diversity_penalty"
	ModelFeatureFormatResponse                ModelFeature = "format_response"
	ModelFeatureStructuredOutputs             ModelFeature = "structured_outputs"
	ModelFeatureStreaming                     ModelFeature = "streaming"
)

// ModelLimit identifies one model token limit.
type ModelLimit string

// Model limit identifiers.
const (
	ModelLimitContextWindow ModelLimit = "context_window"
	ModelLimitInputTokens   ModelLimit = "input_tokens"
	ModelLimitOutputTokens  ModelLimit = "output_tokens"
	ModelLimitDocumentPages ModelLimit = "document_pages"
)

// modelLimitOrder is every model limit in published order.
//
// One list, because a limit that is added to the struct and forgotten in a
// codec is a limit that survives a round trip in one format and vanishes in
// another. Every place that walks the limits walks this.
var modelLimitOrder = []ModelLimit{
	ModelLimitContextWindow,
	ModelLimitInputTokens,
	ModelLimitOutputTokens,
	ModelLimitDocumentPages,
}

// SetSupport records an explicit supported or unsupported capability.
func (f *ModelFeatures) SetSupport(feature ModelFeature, supported bool) bool {
	if f == nil {
		return false
	}
	field, ok := modelFeatureField(reflect.ValueOf(f).Elem(), feature)
	if !ok {
		return false
	}
	field.SetBool(supported)
	f.markFeature(feature, ValueKnown)
	return true
}

// SetSupportUnknown records that a capability was explicitly reported as
// unknown.
func (f *ModelFeatures) SetSupportUnknown(feature ModelFeature) bool {
	if f == nil {
		return false
	}
	field, ok := modelFeatureField(reflect.ValueOf(f).Elem(), feature)
	if !ok {
		return false
	}
	field.SetBool(false)
	f.markFeature(feature, ValueUnknown)
	return true
}

// UnsetSupport removes a capability claim.
func (f *ModelFeatures) UnsetSupport(feature ModelFeature) bool {
	if f == nil {
		return false
	}
	field, ok := modelFeatureField(reflect.ValueOf(f).Elem(), feature)
	if !ok {
		return false
	}
	field.SetBool(false)
	index := modelFeatureIndexes[feature]
	mask := uint64(1) << index
	f.featurePresence &^= mask
	f.featureKnown &^= mask
	return true
}

// Support returns the capability value and its presence state.
func (f *ModelFeatures) Support(feature ModelFeature) (bool, ValuePresence) {
	if f == nil {
		return false, ValueMissing
	}
	field, ok := modelFeatureField(reflect.ValueOf(f).Elem(), feature)
	if !ok {
		return false, ValueMissing
	}
	value := field.Bool()
	index := modelFeatureIndexes[feature]
	mask := uint64(1) << index
	if f.featurePresence&mask != 0 {
		if f.featureKnown&mask != 0 {
			return value, ValueKnown
		}
		return false, ValueUnknown
	}
	if value {
		return true, ValueKnown
	}
	return false, ValueMissing
}

func (f *ModelFeatures) markFeature(feature ModelFeature, state ValuePresence) {
	index, exists := modelFeatureIndexes[feature]
	if !exists {
		return
	}
	mask := uint64(1) << index
	f.featurePresence |= mask
	if state == ValueKnown {
		f.featureKnown |= mask
	} else {
		f.featureKnown &^= mask
	}
}

var (
	modelFeatureIndexes, allModelFeatures = indexModelFeatures()
)

func indexModelFeatures() (map[ModelFeature]int, []ModelFeature) {
	typ := reflect.TypeOf(ModelFeatures{})
	indexes := make(map[ModelFeature]int)
	features := make([]ModelFeature, 0, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.Type.Kind() != reflect.Bool {
			continue
		}
		name := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		feature := ModelFeature(name)
		indexes[feature] = index
		features = append(features, feature)
	}
	return indexes, features
}

func modelFeatureField(value reflect.Value, feature ModelFeature) (reflect.Value, bool) {
	index, ok := modelFeatureIndexes[feature]
	if !ok {
		return reflect.Value{}, false
	}
	return value.Field(index), true
}

func modelFeatures() []ModelFeature {
	return allModelFeatures
}

// Set records an explicit model limit, including zero.
func (l *ModelLimits) Set(limit ModelLimit, value int64) bool {
	if l == nil {
		return false
	}
	if !l.setLimitValue(limit, value) {
		return false
	}
	l.markLimit(limit, ValueKnown)
	return true
}

// SetUnknown records that a model limit was explicitly reported as unknown.
func (l *ModelLimits) SetUnknown(limit ModelLimit) bool {
	if l == nil {
		return false
	}
	if !l.setLimitValue(limit, 0) {
		return false
	}
	l.markLimit(limit, ValueUnknown)
	return true
}

// Unset removes a model limit claim.
func (l *ModelLimits) Unset(limit ModelLimit) bool {
	if l == nil {
		return false
	}
	if !l.setLimitValue(limit, 0) {
		return false
	}
	mask := modelLimitMask(limit)
	l.limitPresence &^= mask
	l.limitKnown &^= mask
	return true
}

// Value returns a model limit and its presence state.
func (l *ModelLimits) Value(limit ModelLimit) (int64, ValuePresence) {
	if l == nil {
		return 0, ValueMissing
	}
	value, ok := l.limitValue(limit)
	if !ok {
		return 0, ValueMissing
	}
	mask := modelLimitMask(limit)
	if l.limitPresence&mask != 0 {
		if l.limitKnown&mask != 0 {
			return value, ValueKnown
		}
		return 0, ValueUnknown
	}
	if value != 0 {
		return value, ValueKnown
	}
	return 0, ValueMissing
}

func (l *ModelLimits) markLimit(limit ModelLimit, state ValuePresence) {
	mask := modelLimitMask(limit)
	if mask == 0 {
		return
	}
	l.limitPresence |= mask
	if state == ValueKnown {
		l.limitKnown |= mask
	} else {
		l.limitKnown &^= mask
	}
}

func modelLimitMask(limit ModelLimit) uint8 {
	switch limit {
	case ModelLimitContextWindow:
		return 1 << 0
	case ModelLimitInputTokens:
		return 1 << 1
	case ModelLimitOutputTokens:
		return 1 << 2
	case ModelLimitDocumentPages:
		return 1 << 3
	default:
		return 0
	}
}

func (l *ModelLimits) limitValue(limit ModelLimit) (int64, bool) {
	switch limit {
	case ModelLimitContextWindow:
		return l.ContextWindow, true
	case ModelLimitInputTokens:
		return l.InputTokens, true
	case ModelLimitOutputTokens:
		return l.OutputTokens, true
	case ModelLimitDocumentPages:
		return l.DocumentPages, true
	default:
		return 0, false
	}
}

func (l *ModelLimits) setLimitValue(limit ModelLimit, value int64) bool {
	switch limit {
	case ModelLimitContextWindow:
		l.ContextWindow = value
	case ModelLimitInputTokens:
		l.InputTokens = value
	case ModelLimitOutputTokens:
		l.OutputTokens = value
	case ModelLimitDocumentPages:
		l.DocumentPages = value
	default:
		return false
	}
	return true
}

// SetDescription records an explicit model description, including an empty
// description.
func (m *Model) SetDescription(description string) {
	m.Description = description
	m.descriptionPresence = ValueKnown
}

// SetDescriptionUnknown records that the description is explicitly unknown.
func (m *Model) SetDescriptionUnknown() {
	m.Description = ""
	m.descriptionPresence = ValueUnknown
}

// UnsetDescription removes the model's description claim.
func (m *Model) UnsetDescription() {
	m.Description = ""
	m.descriptionPresence = ValueMissing
}

// DescriptionValue returns the description and its presence state.
func (m *Model) DescriptionValue() (string, ValuePresence) {
	if m == nil {
		return "", ValueMissing
	}
	if m.descriptionPresence != ValueMissing {
		return m.Description, m.descriptionPresence
	}
	if m.Description != "" {
		return m.Description, ValueKnown
	}
	return "", ValueMissing
}

// SetOpenWeights records an explicit open-weights value.
func (m *ModelMetadata) SetOpenWeights(open bool) {
	m.OpenWeights = open
	m.openWeightsPresence = ValueKnown
}

// SetOpenWeightsUnknown records that open-weights status is explicitly
// unknown.
func (m *ModelMetadata) SetOpenWeightsUnknown() {
	m.OpenWeights = false
	m.openWeightsPresence = ValueUnknown
}

// UnsetOpenWeights removes the open-weights claim.
func (m *ModelMetadata) UnsetOpenWeights() {
	m.OpenWeights = false
	m.openWeightsPresence = ValueMissing
}

// OpenWeightsValue returns open-weights support and its presence state.
func (m *ModelMetadata) OpenWeightsValue() (bool, ValuePresence) {
	if m == nil {
		return false, ValueMissing
	}
	if m.openWeightsPresence != ValueMissing {
		return m.OpenWeights, m.openWeightsPresence
	}
	if m.OpenWeights {
		return true, ValueKnown
	}
	return false, ValueMissing
}

func isJSONNull(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

// Equal reports whether two models have the same serialized facts and presence
// semantics.
func (m Model) Equal(other Model) bool {
	return equalPresenceJSON(m, other)
}

func equalPresenceJSON(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}
