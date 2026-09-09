package reconciler

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestLegacyCompositeValuesPreserveQuotedKeysAndPresence(t *testing.T) {
	for _, scenario := range []struct {
		name, path string
		root, want any
		present    bool
	}{
		{"null", `["source"].fields["null"]`, map[string]any{"source": map[string]any{"fields": map[string]any{"null": nil}}}, nil, true},
		{"missing", `["source"].fields["missing"]`, map[string]any{"source": map[string]any{"fields": map[string]any{"null": nil}}}, nil, false},
		{"false", ".tool_calls", map[string]any{"tool_calls": false}, false, true},
		{"empty-record", `["fast"].provider.present`, map[string]any{"fast": map[string]any{"provider": map[string]any{}}}, true, true},
		{"quoted-key", `["a.b\"c"].fields["x[y]"]`, map[string]any{`a.b"c`: map[string]any{"fields": map[string]any{"x[y]": json.Number("9007199254740993")}}}, json.Number("9007199254740993"), true},
		{"author-membership", `["author"].present`, []any{map[string]any{"id": "author", "name": "Prior name"}}, true, true},
		{"new-author", `["added"].present`, []any{map[string]any{"id": "author"}}, nil, false},
		{"tag", `.tags["code"]`, map[string]any{"tags": []any{"code", "chat"}}, true, true},
		{"modality", ".modalities.input.image", map[string]any{"modalities": map[string]any{"input": []any{"image"}}}, true, true},
		{"invalid-key", `["unfinished]`, map[string]any{}, nil, false},
		{"invalid-escape", `["\q"]`, map[string]any{}, nil, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			got, present := legacyCompositeValue(scenario.root, scenario.path)
			if present != scenario.present || (present && !semanticValueEqual(scenario.path, got, scenario.want)) {
				t.Fatalf("value=%v present=%t, want value=%v present=%t", got, present, scenario.want, scenario.present)
			}
		})
	}
}

func TestLegacyCompositeTypedFeaturesKeepExplicitFalse(t *testing.T) {
	features := &catalogs.ModelFeatures{}
	features.SetSupport(catalogs.ModelFeatureToolCalls, false)
	root, err := normalizedSemanticValue("", features)
	if err != nil {
		t.Fatal(err)
	}
	value, present := legacyCompositeValue(root, ".tool_calls")
	if !present || value != false {
		t.Fatalf("value=%v present=%t, want explicit false", value, present)
	}
}
