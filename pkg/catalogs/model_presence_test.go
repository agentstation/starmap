package catalogs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestF008PresenceRoundTripsThroughHumanYAMLAndCatalogJSON(t *testing.T) {
	model := Model{
		ID:       "presence",
		Name:     "Presence",
		Status:   ModelStatusUnknown,
		Metadata: &ModelMetadata{},
		Features: &ModelFeatures{},
		Limits:   &ModelLimits{},
	}
	model.SetDescription("")
	model.Metadata.SetOpenWeights(false)
	model.Features.SetSupport(ModelFeatureToolCalls, false)
	model.Features.SetSupportUnknown(ModelFeatureTools)
	model.Limits.Set(ModelLimitContextWindow, 0)
	model.Limits.SetUnknown(ModelLimitInputTokens)

	encoded, err := model.EncodeYAML()
	if err != nil {
		t.Fatalf("EncodeYAML: %v", err)
	}
	for _, expected := range []string{
		`description: ""`,
		"status: unknown",
		"open_weights: false",
		"tool_calls: false",
		"tools: null",
		"context_window: 0",
		"input_tokens: null",
	} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("YAML missing %q:\n%s", expected, encoded)
		}
	}
	for _, absent := range []string{"modalities:", "\n  output_tokens:"} {
		if strings.Contains(encoded, absent) {
			t.Fatalf("YAML contains missing claim %q:\n%s", absent, encoded)
		}
	}
	if !strings.Contains(encoded, "web_search: false") {
		t.Fatalf("YAML does not render missing Boolean capability as false:\n%s", encoded)
	}

	var fromYAML Model
	if err := yaml.Unmarshal([]byte(encoded), &fromYAML); err != nil {
		t.Fatalf("Unmarshal YAML: %v", err)
	}
	assertF008Presence(t, &fromYAML, ValueKnown)

	payload, err := json.Marshal(fromYAML)
	if err != nil {
		t.Fatalf("Marshal JSON: %v", err)
	}
	var fromJSON Model
	if err := json.Unmarshal(payload, &fromJSON); err != nil {
		t.Fatalf("Unmarshal JSON: %v", err)
	}
	assertF008Presence(t, &fromJSON, ValueKnown)
}

func TestF008DirectNonZeroValuesAreKnownWithoutSetterBoilerplate(t *testing.T) {
	model := Model{
		Description: "documented",
		Metadata:    &ModelMetadata{OpenWeights: true},
		Features:    &ModelFeatures{ToolCalls: true},
		Limits:      &ModelLimits{ContextWindow: 128000},
	}
	if value, state := model.DescriptionValue(); value != "documented" || state != ValueKnown {
		t.Fatalf("description = %q, %v; want documented, known", value, state)
	}
	if value, state := model.Metadata.OpenWeightsValue(); !value || state != ValueKnown {
		t.Fatalf("open weights = %v, %v; want true, known", value, state)
	}
	if value, state := model.Features.Support(ModelFeatureToolCalls); !value || state != ValueKnown {
		t.Fatalf("tool calls = %v, %v; want true, known", value, state)
	}
	if value, state := model.Limits.Value(ModelLimitContextWindow); value != 128000 || state != ValueKnown {
		t.Fatalf("context window = %d, %v; want 128000, known", value, state)
	}
}

func TestPresenceStateIsDeepCopied(t *testing.T) {
	original := Model{
		ID:       "presence",
		Name:     "Presence",
		Features: &ModelFeatures{},
		Limits:   &ModelLimits{},
	}
	original.Features.SetSupport(ModelFeatureTools, false)
	original.Limits.Set(ModelLimitContextWindow, 0)

	copied := DeepCopyModel(original)
	copied.Features.SetSupport(ModelFeatureTools, true)
	copied.Limits.SetUnknown(ModelLimitContextWindow)

	if value, state := original.Features.Support(ModelFeatureTools); value || state != ValueKnown {
		t.Fatalf("original tools = %v, %v; want false, known", value, state)
	}
	if value, state := original.Limits.Value(ModelLimitContextWindow); value != 0 || state != ValueKnown {
		t.Fatalf("original context = %d, %v; want zero, known", value, state)
	}
}

func TestModelFeaturePresenceFitsCompactRepresentation(t *testing.T) {
	if got := len(modelFeatures()); got > 64 {
		t.Fatalf("model feature count = %d, exceeds 64-bit presence capacity", got)
	}
}

func TestEveryModelFeaturePresenceStateRoundTrips(t *testing.T) {
	tests := []struct {
		name      string
		apply     func(*ModelFeatures, ModelFeature)
		wantValue bool
		wantState ValuePresence
	}{
		{name: "missing", apply: func(*ModelFeatures, ModelFeature) {}, wantState: ValueMissing},
		{
			name: "unknown", apply: func(features *ModelFeatures, feature ModelFeature) {
				features.SetSupportUnknown(feature)
			}, wantState: ValueUnknown,
		},
		{
			name: "known-false", apply: func(features *ModelFeatures, feature ModelFeature) {
				features.SetSupport(feature, false)
			}, wantState: ValueKnown,
		},
		{
			name: "known-true", apply: func(features *ModelFeatures, feature ModelFeature) {
				features.SetSupport(feature, true)
			}, wantValue: true, wantState: ValueKnown,
		},
	}

	for _, feature := range modelFeatures() {
		for _, test := range tests {
			t.Run(string(feature)+"/"+test.name, func(t *testing.T) {
				model := Model{ID: "presence", Name: "Presence", Features: &ModelFeatures{}}
				test.apply(model.Features, feature)
				assertModelPresenceRoundTrips(t, model, func(t *testing.T, decoded *Model, format string) {
					t.Helper()
					value, state := decoded.Features.Support(feature)
					wantState := test.wantState
					if format == "yaml" && wantState == ValueMissing {
						wantState = ValueKnown
					}
					if value != test.wantValue || state != wantState {
						t.Fatalf("Support(%q) after %s = %v/%v, want %v/%v", feature, format, value, state, test.wantValue, wantState)
					}
				})
			})
		}
	}
}

func TestEveryModelLimitPresenceStateRoundTrips(t *testing.T) {
	limits := []ModelLimit{
		ModelLimitContextWindow,
		ModelLimitInputTokens,
		ModelLimitOutputTokens,
	}
	tests := []struct {
		name      string
		apply     func(*ModelLimits, ModelLimit)
		wantValue int64
		wantState ValuePresence
	}{
		{name: "missing", apply: func(*ModelLimits, ModelLimit) {}, wantState: ValueMissing},
		{
			name: "unknown", apply: func(limits *ModelLimits, limit ModelLimit) {
				limits.SetUnknown(limit)
			}, wantState: ValueUnknown,
		},
		{
			name: "known-zero", apply: func(limits *ModelLimits, limit ModelLimit) {
				limits.Set(limit, 0)
			}, wantState: ValueKnown,
		},
		{
			name: "known-positive", apply: func(limits *ModelLimits, limit ModelLimit) {
				limits.Set(limit, 128000)
			}, wantValue: 128000, wantState: ValueKnown,
		},
	}

	for _, limit := range limits {
		for _, test := range tests {
			t.Run(string(limit)+"/"+test.name, func(t *testing.T) {
				model := Model{ID: "presence", Name: "Presence", Limits: &ModelLimits{}}
				test.apply(model.Limits, limit)
				assertModelPresenceRoundTrips(t, model, func(t *testing.T, decoded *Model, _ string) {
					t.Helper()
					value, state := decoded.Limits.Value(limit)
					if value != test.wantValue || state != test.wantState {
						t.Fatalf("Value(%q) = %d/%v, want %d/%v", limit, value, state, test.wantValue, test.wantState)
					}
				})
			})
		}
	}
}

func assertModelPresenceRoundTrips(
	t *testing.T,
	model Model,
	assert func(*testing.T, *Model, string),
) {
	t.Helper()
	assert(t, &model, "memory")

	copied := DeepCopyModel(model)
	assert(t, &copied, "copy")

	jsonData, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("Marshal JSON: %v", err)
	}
	var fromJSON Model
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("Unmarshal JSON: %v", err)
	}
	assert(t, &fromJSON, "json")

	yamlData, err := model.EncodeYAML()
	if err != nil {
		t.Fatalf("Encode YAML: %v", err)
	}
	var fromYAML Model
	if err := yaml.Unmarshal([]byte(yamlData), &fromYAML); err != nil {
		t.Fatalf("Unmarshal YAML: %v", err)
	}
	assert(t, &fromYAML, "yaml")
}

func TestModelPresenceYAMLPreservesTypedCollectionShapes(t *testing.T) {
	model := Model{
		ID:      "typed-shapes",
		Name:    "Typed Shapes",
		Authors: []Author{{ID: "author", Name: "Author"}},
		Modes: map[string]ModelMode{
			"fast": {},
		},
	}
	model.SetDescription("")
	encoded, err := model.EncodeYAML()
	if err != nil {
		t.Fatalf("EncodeYAML: %v", err)
	}
	if !strings.Contains(encoded, "authors:\n- id: author") {
		t.Fatalf("authors were not encoded as a sequence:\n%s", encoded)
	}
	var decoded Model
	if err := yaml.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("Unmarshal YAML: %v\n%s", err, encoded)
	}
	if len(decoded.Authors) != 1 || decoded.Authors[0].ID != "author" {
		t.Fatalf("authors = %#v", decoded.Authors)
	}
	if _, exists := decoded.Modes["fast"]; !exists {
		t.Fatalf("modes = %#v", decoded.Modes)
	}
}

func assertF008Presence(t *testing.T, model *Model, missingFeatureState ValuePresence) {
	t.Helper()
	if value, state := model.DescriptionValue(); value != "" || state != ValueKnown {
		t.Fatalf("description = %q, %v; want empty, known", value, state)
	}
	if model.Status != ModelStatusUnknown {
		t.Fatalf("status = %q, want unknown", model.Status)
	}
	if value, state := model.Metadata.OpenWeightsValue(); value || state != ValueKnown {
		t.Fatalf("open weights = %v, %v; want false, known", value, state)
	}
	if value, state := model.Features.Support(ModelFeatureToolCalls); value || state != ValueKnown {
		t.Fatalf("tool_calls = %v, %v; want false, known", value, state)
	}
	if _, state := model.Features.Support(ModelFeatureTools); state != ValueUnknown {
		t.Fatalf("tools presence = %v, want unknown", state)
	}
	if _, state := model.Features.Support(ModelFeatureWebSearch); state != missingFeatureState {
		t.Fatalf("web_search presence = %v, want %v", state, missingFeatureState)
	}
	if value, state := model.Limits.Value(ModelLimitContextWindow); value != 0 || state != ValueKnown {
		t.Fatalf("context window = %d, %v; want 0, known", value, state)
	}
	if _, state := model.Limits.Value(ModelLimitInputTokens); state != ValueUnknown {
		t.Fatalf("input tokens presence = %v, want unknown", state)
	}
	if _, state := model.Limits.Value(ModelLimitOutputTokens); state != ValueMissing {
		t.Fatalf("output tokens presence = %v, want missing", state)
	}
}
