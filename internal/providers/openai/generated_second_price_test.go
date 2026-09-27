package openai

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestAnExistingPerSecondPriceSurvivesTheProviderAnswer keeps the acquisition
// path from overwriting a price an authoritative source already recorded. Every
// other field in this function defers the same way.
func TestAnExistingPerSecondPriceSurvivesTheProviderAnswer(t *testing.T) {
	t.Parallel()

	const recorded = 0.02
	const reported = 0.09

	existing := recorded
	pricing := &catalogs.ModelPricing{
		Operations: &catalogs.ModelOperationPricing{OutputSecond: &existing},
	}
	seconds := reported

	applyOpenAICompatibleMetadataPricing(pricing, &ModelMetadataPricing{
		OutputSeconds: &seconds,
	})

	if got := pricing.Operations.OutputSecond; got == nil || *got != recorded {
		t.Errorf("output_second = %v, want the recorded %v", got, recorded)
	}
	if pricing.Operations.AudioGen != nil {
		t.Errorf("audio_gen = %v, want none", *pricing.Operations.AudioGen)
	}
}
