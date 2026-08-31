package openai

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestAPerSecondPriceLandsUnderTheOperationTheOutputNames(t *testing.T) {
	t.Parallel()

	const perSecond = 0.05

	tests := []struct {
		name         string
		output       []catalogs.ModelModality
		wantVideoGen bool
	}{
		{
			name:         "video output prices the video operation",
			output:       []catalogs.ModelModality{catalogs.ModelModalityVideo},
			wantVideoGen: true,
		},
		{
			name:   "audio output prices the audio operation",
			output: []catalogs.ModelModality{catalogs.ModelModalityAudio},
		},
		{
			name:   "an undeclared output stays on the audio operation",
			output: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			model := &catalogs.Model{
				ID: "provider/model",
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: test.output,
					},
				},
			}
			pricing := &catalogs.ModelPricing{}
			seconds := perSecond

			applyOpenAICompatibleMetadataPricing(model, pricing, &ModelMetadataPricing{
				OutputSeconds: &seconds,
			})

			if pricing.Operations == nil {
				t.Fatal("no operation pricing was recorded")
			}
			video := pricing.Operations.VideoGen
			audio := pricing.Operations.AudioGen
			if test.wantVideoGen {
				if video == nil || *video != perSecond {
					t.Errorf("video_gen = %v, want %v", video, perSecond)
				}
				if audio != nil {
					t.Errorf("audio_gen = %v, want none", *audio)
				}
				return
			}
			if audio == nil || *audio != perSecond {
				t.Errorf("audio_gen = %v, want %v", audio, perSecond)
			}
			if video != nil {
				t.Errorf("video_gen = %v, want none", *video)
			}
		})
	}
}

// TestAnExistingPerSecondPriceSurvivesTheProviderAnswer keeps the acquisition
// path from overwriting a price an authoritative source already recorded. Every
// other field in this function defers the same way.
func TestAnExistingPerSecondPriceSurvivesTheProviderAnswer(t *testing.T) {
	t.Parallel()

	const recorded = 0.02
	const reported = 0.09

	model := &catalogs.Model{
		ID: "provider/video",
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Output: []catalogs.ModelModality{catalogs.ModelModalityVideo},
			},
		},
	}
	existing := recorded
	pricing := &catalogs.ModelPricing{
		Operations: &catalogs.ModelOperationPricing{VideoGen: &existing},
	}
	seconds := reported

	applyOpenAICompatibleMetadataPricing(model, pricing, &ModelMetadataPricing{
		OutputSeconds: &seconds,
	})

	if got := pricing.Operations.VideoGen; got == nil || *got != recorded {
		t.Errorf("video_gen = %v, want the recorded %v", got, recorded)
	}
	if pricing.Operations.AudioGen != nil {
		t.Errorf("audio_gen = %v, want none", *pricing.Operations.AudioGen)
	}
}
