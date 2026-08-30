package catalogs

import (
	"slices"
	"testing"
)

func price(value float64) *float64 { return &value }

func textModel(pricing *ModelPricing, tags ...ModelTag) Model {
	model := Model{
		Features: &ModelFeatures{
			Modalities: ModelModalities{
				Input:  []ModelModality{ModelModalityText},
				Output: []ModelModality{ModelModalityText},
			},
		},
		Pricing: pricing,
	}
	if len(tags) > 0 {
		model.Metadata = &ModelMetadata{Tags: tags}
	}
	return model
}

func TestOfferingOperationsFollowServedOperationNotOperationPricing(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		model Model
		want  []ProviderOperation
	}{
		{
			name:  "no pricing at all",
			model: textModel(nil),
			want:  []ProviderOperation{ProviderOperationChatCompletions},
		},
		{
			// The Groq shape: a flat request fee and an image price of zero,
			// which together state that the provider charges for neither.
			name: "zero request and image generation prices",
			model: textModel(&ModelPricing{Operations: &ModelOperationPricing{
				Request:  price(0),
				ImageGen: price(0),
			}}),
			want: []ProviderOperation{ProviderOperationChatCompletions},
		},
		{
			// The Gemini Flash shape: audio costs extra on the way in.
			name: "audio input surcharge",
			model: textModel(&ModelPricing{Operations: &ModelOperationPricing{
				AudioInput: price(1.5),
			}}),
			want: []ProviderOperation{ProviderOperationChatCompletions},
		},
		{
			name: "billed request fee",
			model: textModel(&ModelPricing{Operations: &ModelOperationPricing{
				Request: price(0.004),
			}}),
			want: []ProviderOperation{ProviderOperationChatCompletions},
		},
		{
			// The realtime and Live API shape: the model produces audio.
			name: "billed audio generation",
			model: textModel(&ModelPricing{Operations: &ModelOperationPricing{
				AudioInput: price(32),
				AudioGen:   price(64),
			}}),
			want: nil,
		},
		{
			name: "billed image generation",
			model: textModel(&ModelPricing{Operations: &ModelOperationPricing{
				ImageGen: price(0.04),
			}}),
			want: nil,
		},
		{
			name: "billed video generation",
			model: textModel(&ModelPricing{Operations: &ModelOperationPricing{
				VideoGen: price(0.1),
			}}),
			want: nil,
		},
		{
			name:  "tagged speech to text",
			model: textModel(nil, ModelTagSpeechToText),
			want:  nil,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := deriveOfferingCapabilities(testCase.model).Operations
			if !slices.Equal(got, testCase.want) {
				t.Fatalf("operations = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestEmbeddingModelsServeEmbeddingsInsteadOfChatCompletions guards the reverse
// direction. A model that returns vectors must never offer a chat route,
// regardless of its pricing.
func TestEmbeddingModelsServeEmbeddingsInsteadOfChatCompletions(t *testing.T) {
	embedding := Model{
		Features: &ModelFeatures{
			Modalities: ModelModalities{
				Input:  []ModelModality{ModelModalityText},
				Output: []ModelModality{ModelModalityEmbedding},
			},
		},
		Metadata: &ModelMetadata{Tags: []ModelTag{ModelTagEmbedding}},
		Pricing:  &ModelPricing{Operations: &ModelOperationPricing{AudioInput: price(6.5)}},
	}
	got := deriveOfferingCapabilities(embedding).Operations
	want := []ProviderOperation{ProviderOperationEmbeddings}
	if !slices.Equal(got, want) {
		t.Fatalf("operations = %v, want %v", got, want)
	}
}
