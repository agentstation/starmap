package catalogs

import (
	"testing"
	"time"

	"github.com/agentstation/utc"
)

func TestValidateModelFactConsistencyRejectsContradictions(t *testing.T) {
	tests := []struct {
		name  string
		model Model
	}{
		{
			name: "timestamp order",
			model: Model{
				CreatedAt: utc.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				UpdatedAt: utc.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
		},
		{
			name: "reasoning control without capability",
			model: Model{
				Features:  &ModelFeatures{},
				Reasoning: &ModelControlLevels{Levels: []ModelControlLevel{ModelControlLevelLow}},
			},
		},
		{
			name: "embedding tag without embedding output",
			model: Model{
				Metadata: &ModelMetadata{Tags: []ModelTag{"embed"}},
				Features: &ModelFeatures{Modalities: ModelModalities{
					Input:  []ModelModality{ModelModalityText},
					Output: []ModelModality{ModelModalityText},
				}},
			},
		},
		{
			name: "speech tag without audio input",
			model: Model{
				Metadata: &ModelMetadata{Tags: []ModelTag{"stt"}},
				Features: &ModelFeatures{Modalities: ModelModalities{
					Input:  []ModelModality{ModelModalityText},
					Output: []ModelModality{ModelModalityText},
				}},
			},
		},
		{
			name: "canonical text to video tag without video output",
			model: Model{
				Metadata: &ModelMetadata{Tags: []ModelTag{ModelTagTextToVideo}},
				Features: &ModelFeatures{Modalities: ModelModalities{
					Input:  []ModelModality{ModelModalityText},
					Output: []ModelModality{ModelModalityText},
				}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateModelFactConsistency(test.model); err == nil {
				t.Fatal("validateModelFactConsistency returned nil")
			}
		})
	}
}

func TestValidateModelFactConsistencyAcceptsCoherentOperationalModel(t *testing.T) {
	model := Model{
		CreatedAt: utc.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		UpdatedAt: utc.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
		Metadata:  &ModelMetadata{Tags: []ModelTag{"stt"}},
		Features: &ModelFeatures{
			Modalities: ModelModalities{
				Input:  []ModelModality{ModelModalityAudio},
				Output: []ModelModality{ModelModalityText},
			},
			Reasoning:       true,
			ReasoningEffort: true,
		},
		Reasoning: &ModelControlLevels{Levels: []ModelControlLevel{ModelControlLevelLow}},
	}
	if err := validateModelFactConsistency(model); err != nil {
		t.Fatalf("validateModelFactConsistency: %v", err)
	}
}
