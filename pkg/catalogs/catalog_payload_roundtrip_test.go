package catalogs

import (
	"bytes"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/provenance"
)

func TestCatalogPayloadSurvivesHumanWorkspaceRoundTrip(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	author := Author{ID: "qwen", Name: "Qwen"}
	features := &ModelFeatures{}
	for _, feature := range modelFeatures() {
		features.SetSupport(feature, false)
	}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel(author.ID, Model{
		ID: "qwen3-tts", Name: "Qwen3-TTS", Authors: []Author{author}, Features: features,
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	model := Model{
		ID:       "Qwen/Qwen3-TTS",
		ModelRef: "qwen/qwen3-tts",
		Name:     "Qwen/Qwen3-TTS",
		Features: features,
		Description: `Key capabilities: instruction control with natural language ` +
			`(e.g. "speak slowly and calmly", "excited tone")`,
	}
	audioInputPrice := 0.00005
	if err := builder.SetProvider(Provider{
		ID: "deepinfra", Name: "DeepInfra",
		Models: map[string]*Model{model.ID: &model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	builder.SetProvenance(provenance.Map{
		"model:deepinfra/Qwen%2FQwen3-TTS:Description": {{
			Source: catalogmeta.ProvidersID, Field: "Description", Value: model.Description,
		}},
		"model:deepinfra/Qwen%2FQwen3-TTS:Authors": {{
			Source: catalogmeta.ProvidersID, Field: "Authors", Value: []Author{author},
		}},
		"model:deepinfra/Qwen%2FQwen3-TTS:pricing": {{
			Source: catalogmeta.ProvidersID, Field: "pricing", Value: ModelPricing{
				Currency: ModelPricingCurrencyUSD,
				Operations: &ModelOperationPricing{
					AudioInput: &audioInputPrice,
				},
				Tokens: &ModelTokenPricing{
					Input: &ModelTokenCost{Per1M: 0.005},
				},
			},
		}},
	})
	original, err := builder.Build()
	if err != nil {
		t.Fatalf("Build original: %v", err)
	}
	originalPayload, err := EncodeCatalogPayload(original)
	if err != nil {
		t.Fatalf("Encode original: %v", err)
	}

	path := t.TempDir()
	if err := builder.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	reloadedBuilder, err := NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	reloaded, err := reloadedBuilder.Build()
	if err != nil {
		t.Fatalf("Build reloaded: %v", err)
	}
	reloadedPayload, err := EncodeCatalogPayload(reloaded)
	if err != nil {
		t.Fatalf("Encode reloaded: %v", err)
	}
	if !bytes.Equal(originalPayload, reloadedPayload) {
		t.Fatalf(
			"catalog payload changed across human workspace round trip:\noriginal: %s\nreloaded: %s",
			originalPayload,
			reloadedPayload,
		)
	}
}
