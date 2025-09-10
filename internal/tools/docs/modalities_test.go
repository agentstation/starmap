package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestHasModality(t *testing.T) {
	tests := []struct {
		name     string
		features *catalogs.ModelFeatures
		modality catalogs.ModelModality
		expected bool
	}{
		{
			name:     "nil features",
			features: nil,
			modality: catalogs.ModelModalityText,
			expected: false,
		},
		{
			name: "has text input",
			features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			modality: catalogs.ModelModalityText,
			expected: true,
		},
		{
			name: "has text output",
			features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			modality: catalogs.ModelModalityText,
			expected: true,
		},
		{
			name: "does not have image",
			features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			modality: catalogs.ModelModalityImage,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasModality(tt.features, tt.modality)
			assert.Equal(t, tt.expected, result)
		})
	}
}


func TestModalitySpecificHelpers(t *testing.T) {
	multimodal := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input: []catalogs.ModelModality{
				catalogs.ModelModalityText,
				catalogs.ModelModalityImage,
				catalogs.ModelModalityAudio,
				catalogs.ModelModalityVideo,
				catalogs.ModelModalityPDF,
			},
			Output: []catalogs.ModelModality{
				catalogs.ModelModalityText,
				catalogs.ModelModalityEmbedding,
			},
		},
	}

	textOnly := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
	}

	t.Run("hasText", func(t *testing.T) {
		assert.True(t, hasText(multimodal))
		assert.True(t, hasText(textOnly))
		assert.False(t, hasText(nil))
	})

	t.Run("hasVision", func(t *testing.T) {
		assert.True(t, hasVision(multimodal))
		assert.False(t, hasVision(textOnly))
		assert.False(t, hasVision(nil))
	})

	t.Run("hasAudio", func(t *testing.T) {
		assert.True(t, hasAudio(multimodal))
		assert.False(t, hasAudio(textOnly))
		assert.False(t, hasAudio(nil))
	})

	t.Run("hasVideo", func(t *testing.T) {
		assert.True(t, hasVideo(multimodal))
		assert.False(t, hasVideo(textOnly))
		assert.False(t, hasVideo(nil))
	})

	t.Run("hasPDF", func(t *testing.T) {
		assert.True(t, hasPDF(multimodal))
		assert.False(t, hasPDF(textOnly))
		assert.False(t, hasPDF(nil))
	})

}


func TestHasToolSupportModalities(t *testing.T) {
	tests := []struct {
		name     string
		features *catalogs.ModelFeatures
		expected bool
	}{
		{
			name:     "nil features",
			features: nil,
			expected: false,
		},
		{
			name: "has tools",
			features: &catalogs.ModelFeatures{
				Tools: true,
			},
			expected: true,
		},
		{
			name: "no tools",
			features: &catalogs.ModelFeatures{
				Tools: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasToolSupport(tt.features)
			assert.Equal(t, tt.expected, result)
		})
	}
}
