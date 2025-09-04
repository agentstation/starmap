package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/stretchr/testify/assert"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration *time.Duration
		expected string
	}{
		{
			name:     "nil duration",
			duration: nil,
			expected: "Not specified",
		},
		{
			name:     "zero duration",
			duration: durationPtr(0),
			expected: "Immediate deletion",
		},
		{
			name:     "one hour",
			duration: durationPtr(time.Hour),
			expected: "1 hour",
		},
		{
			name:     "two hours",
			duration: durationPtr(2 * time.Hour),
			expected: "2 hours",
		},
		{
			name:     "23 hours",
			duration: durationPtr(23 * time.Hour),
			expected: "23 hours",
		},
		{
			name:     "one day",
			duration: durationPtr(24 * time.Hour),
			expected: "1 day",
		},
		{
			name:     "two days",
			duration: durationPtr(48 * time.Hour),
			expected: "2 days",
		},
		{
			name:     "seven days (one week)",
			duration: durationPtr(7 * 24 * time.Hour),
			expected: "7 days",
		},
		{
			name:     "30 days",
			duration: durationPtr(30 * 24 * time.Hour),
			expected: "30 days",
		},
		{
			name:     "90 days",
			duration: durationPtr(90 * 24 * time.Hour),
			expected: "90 days",
		},
		{
			name:     "365 days (one year)",
			duration: durationPtr(365 * 24 * time.Hour),
			expected: "365 days",
		},
		{
			name:     "uneven hours (25 hours)",
			duration: durationPtr(25 * time.Hour),
			expected: "25 hours",
		},
		{
			name:     "uneven hours (49 hours)",
			duration: durationPtr(49 * time.Hour),
			expected: "49 hours",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		number   int
		expected string
	}{
		{0, "0"},
		{100, "100"},
		{1000, "1k"},
		{1500, "1k"}, // formatNumber truncates to int for k values
		{10000, "10k"},
		{100000, "100k"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{10000000, "10.0M"},
		{100000000, "100.0M"},
		{1000000000, "1.0B"},
		{999, "999"},
		{1001, "1k"},
		{999999, "999k"}, // 999999/1000 = 999
		{1000001, "1.0M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatNumber(tt.number)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatContextVariations(t *testing.T) {
	tests := []struct {
		context  int64
		expected string
	}{
		{0, "0"},
		{100, "100"},
		{1000, "1k"},
		{1024, "1.0k"},
		{2048, "2.0k"},
		{4096, "4.1k"},
		{8192, "8.2k"},
		{16384, "16.4k"},
		{32768, "32.8k"}, // 32768/1000 = 32.8
		{65536, "65.5k"}, // 65536/1000 = 65.5
		{128000, "128k"},
		{131072, "131.1k"},
		{1000000, "1.0M"},
		{2000000, "2.0M"},
		{2097152, "2.1M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatContext(tt.context)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a        int
		b        int
		expected int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{0, 0, 0},
		{-1, 1, -1},
		{-5, -3, -5},
		{100, 100, 100},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		assert.Equal(t, tt.expected, result)
	}
}

func TestFormatModelID(t *testing.T) {
	tests := []struct {
		id       string
		expected string
	}{
		{"gpt-4", "gpt-4"},
		{"claude-3-opus", "claude-3-opus"},
		{"model/with/slash", "model-with-slash"},
		{"model@version", "model-at-version"},
		{"model:variant", "model-variant"},
		{"model with spaces", "model-with-spaces"},
		{"UPPERCASE", "uppercase"},
		{"MixedCase", "mixedcase"},
		{"model/with@special:chars and spaces", "model-with-at-special-chars-and-spaces"},
		{"", ""},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := formatModelID(tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompactFeatures(t *testing.T) {
	tests := []struct {
		name     string
		model    catalogs.Model
		expected string
	}{
		{
			name: "no features",
			model: catalogs.Model{
				Features: nil,
			},
			expected: "—",
		},
		{
			name: "text only",
			model: catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
				},
			},
			expected: "📝",
		},
		{
			name: "multimodal",
			model: catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input: []catalogs.ModelModality{
							catalogs.ModelModalityText,
							catalogs.ModelModalityImage,
							catalogs.ModelModalityAudio,
						},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
				},
			},
			expected: "📝 👁️ 🎵",
		},
		{
			name: "with tools and streaming",
			model: catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
					Tools:     true,
					Streaming: true,
				},
			},
			expected: "📝 🔧 ⚡",
		},
		{
			name: "video input",
			model: catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityVideo},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
				},
			},
			expected: "📝 🎬",
		},
		{
			name: "embedding model",
			model: catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: []catalogs.ModelModality{catalogs.ModelModalityEmbedding},
					},
				},
			},
			expected: "📝",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compactFeatures(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to create duration pointers
func durationPtrUtil(d time.Duration) *time.Duration {
	return &d
}

// Tests for getProviderBadge and getAuthorBadge are already in catalog_test.go

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		{0.0, "0"},
		{1.0, "1"},
		{10.0, "10"},
		{100.0, "100"},
		{1.5, "1.50"},
		{1.55, "1.55"},
		{1.555, "1.55"}, // Go truncates, doesn't round 0.5 up
		{1.234, "1.23"},
		{-1.0, "-1"},
		{-1.5, "-1.50"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.3f", tt.value), func(t *testing.T) {
			result := formatFloat(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetModelFilePath(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name            string
		modelID         string
		expectedPath    string
		shouldCreateDir bool
	}{
		{
			name:            "simple model ID",
			modelID:         "gpt-4",
			expectedPath:    "gpt-4.md",
			shouldCreateDir: false,
		},
		{
			name:            "model with subdirectory",
			modelID:         "openai/gpt-oss-120b",
			expectedPath:    "openai/gpt-oss-120b.md",
			shouldCreateDir: true,
		},
		{
			name:            "model with multiple subdirectories",
			modelID:         "meta/llama/llama-3-70b",
			expectedPath:    "meta/llama/llama-3-70b.md",
			shouldCreateDir: true,
		},
		{
			name:            "model with special characters",
			modelID:         "model:variant",
			expectedPath:    "model-variant.md",
			shouldCreateDir: false,
		},
		{
			name:            "model with @ character",
			modelID:         "model@version",
			expectedPath:    "model-at-version.md",
			shouldCreateDir: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getModelFilePath(tempDir, tt.modelID)
			assert.NoError(t, err)

			expected := filepath.Join(tempDir, tt.expectedPath)
			assert.Equal(t, expected, result)

			if tt.shouldCreateDir {
				// Check that directory was created
				dir := filepath.Dir(result)
				info, err := os.Stat(dir)
				assert.NoError(t, err)
				assert.True(t, info.IsDir())
			}
		})
	}
}

func TestFormatFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "gpt-4", "gpt-4"},
		{"with colon", "model:variant", "model-variant"},
		{"with space", "model name", "model-name"},
		{"with at", "model@version", "model-at-version"},
		{"uppercase", "GPT-4", "gpt-4"},
		{"mixed case", "ClaudeModel", "claudemodel"},
		{"multiple special", "model:variant@v1 test", "model-variant-at-v1-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
