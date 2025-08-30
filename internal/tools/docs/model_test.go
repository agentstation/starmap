package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateModelDocs(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create test catalog
	catalog := createTestCatalogForModels()

	// Create generator
	g := New(WithOutputDir(tempDir))

	// Generate model documentation
	err := g.generateModelDocs(filepath.Join(tempDir, "catalog", "models"), catalog)
	require.NoError(t, err)

	// Check that models directory was created
	assert.DirExists(t, filepath.Join(tempDir, "catalog", "models"))

	// Check that README index was created
	assert.FileExists(t, filepath.Join(tempDir, "catalog", "models", "README.md"))
}

func TestGenerateModelIndex(t *testing.T) {
	tempDir := t.TempDir()
	g := New()

	// Create test models with various features
	models := []*catalogs.Model{
		{
			ID:   "gpt-4",
			Name: "GPT-4",
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
				ToolCalls: true,
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 8192,
			},
			Pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.TokenPricing{
					Input: &catalogs.TokenCost{
						Per1M: 30.0,
					},
					Output: &catalogs.TokenCost{
						Per1M: 60.0,
					},
				},
			},
			Authors: []catalogs.Author{
				{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"},
			},
		},
		{
			ID:   "claude-3-opus",
			Name: "Claude 3 Opus",
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
				ToolCalls: true,
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 200000,
			},
			Pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.TokenPricing{
					Input: &catalogs.TokenCost{
						Per1M: 15.0,
					},
					Output: &catalogs.TokenCost{
						Per1M: 75.0,
					},
				},
			},
			Authors: []catalogs.Author{
				{ID: catalogs.AuthorIDAnthropic, Name: "Anthropic"},
			},
		},
		{
			ID:   "whisper-1",
			Name: "Whisper",
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityAudio},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			Authors: []catalogs.Author{
				{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"},
			},
		},
		{
			ID:   "gpt-3.5-turbo",
			Name: "GPT-3.5 Turbo",
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 4096,
			},
			Authors: []catalogs.Author{
				{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"},
			},
		},
		{
			ID:   "claude-3-sonnet",
			Name: "Claude 3 Sonnet",
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 200000,
			},
			Authors: []catalogs.Author{
				{ID: catalogs.AuthorIDAnthropic, Name: "Anthropic"},
			},
		},
	}

	err := g.generateModelIndex(tempDir, models)
	require.NoError(t, err)

	// Read the generated index
	indexPath := filepath.Join(tempDir, "README.md")
	content, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	contentStr := string(content)

	// Check header
	assert.Contains(t, contentStr, "# 🤖 All Models")
	assert.Contains(t, contentStr, "Complete listing of all 5 models")

	// Check Quick Stats section
	assert.Contains(t, contentStr, "## Quick Stats")
	assert.Contains(t, contentStr, "| Capability | Count | Percentage |")
	assert.Contains(t, contentStr, "📝 Text Generation")
	assert.Contains(t, contentStr, "👁️ Vision")
	assert.Contains(t, contentStr, "🎵 Audio")
	assert.Contains(t, contentStr, "🔧 Function Calling")

	// Check Model Families section
	assert.Contains(t, contentStr, "## Model Families")
	assert.Contains(t, contentStr, "### GPT")
	assert.Contains(t, contentStr, "### Claude")
	assert.Contains(t, contentStr, "| Model | Provider | Context | Pricing |")

	// Check model links
	assert.Contains(t, contentStr, "[GPT-4](../authors/openai/models/gpt-4.md)")
	assert.Contains(t, contentStr, "[Claude 3 Opus](../authors/anthropic/models/claude-3-opus.md)")

	// Check comparison sections
	assert.Contains(t, contentStr, "## 💰 Pricing Comparison")
	assert.Contains(t, contentStr, "## 📏 Context Window Comparison")
	assert.Contains(t, contentStr, "## 🎯 Feature Comparison")

	// Check footer
	assert.Contains(t, contentStr, "Back to Catalog")
	assert.Contains(t, contentStr, "Generated by [Starmap]")
}

func TestGenerateModelIndexWithNoModels(t *testing.T) {
	tempDir := t.TempDir()
	g := New()

	// Empty models list
	models := []*catalogs.Model{}

	err := g.generateModelIndex(tempDir, models)
	require.NoError(t, err)

	// Read the generated index
	indexPath := filepath.Join(tempDir, "README.md")
	content, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	contentStr := string(content)

	// Check that it handles empty list gracefully
	assert.Contains(t, contentStr, "Complete listing of all 0 models")
	assert.Contains(t, contentStr, "## Quick Stats")
}

func TestGenerateModelIndexWithMinimalModels(t *testing.T) {
	tempDir := t.TempDir()
	g := New()

	// Models with minimal features (no Features, Limits, or Pricing)
	models := []*catalogs.Model{
		{
			ID:   "minimal-1",
			Name: "Minimal Model 1",
		},
		{
			ID:   "minimal-2",
			Name: "Minimal Model 2",
		},
	}

	err := g.generateModelIndex(tempDir, models)
	require.NoError(t, err)

	// Read the generated index
	indexPath := filepath.Join(tempDir, "README.md")
	content, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	contentStr := string(content)

	// Check that it handles minimal models gracefully
	assert.Contains(t, contentStr, "Complete listing of all 2 models")

	// Stats should show 0 for all capabilities
	assert.Contains(t, contentStr, "| 📝 Text Generation | 0 | 0.0% |")
	assert.Contains(t, contentStr, "| 👁️ Vision | 0 | 0.0% |")
}

func TestGenerateModelIndexLargeFamilies(t *testing.T) {
	tempDir := t.TempDir()
	g := New()

	// Create a large family (more than 10 models)
	var models []*catalogs.Model
	for i := 0; i < 15; i++ {
		models = append(models, &catalogs.Model{
			ID:   fmt.Sprintf("gpt-variant-%d", i),
			Name: fmt.Sprintf("GPT Variant %d", i),
			Authors: []catalogs.Author{
				{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"},
			},
		})
	}

	err := g.generateModelIndex(tempDir, models)
	require.NoError(t, err)

	// Read the generated index
	indexPath := filepath.Join(tempDir, "README.md")
	content, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	contentStr := string(content)

	// Check that large families are truncated
	assert.Contains(t, contentStr, "### GPT (15 models)")
	assert.Contains(t, contentStr, "_...and 5 more_")
}

// TestDetectModelFamily is already defined in docs_test.go

// TestFormatModelID is already defined in utils_test.go

// TestFormatContext is already defined in docs_test.go

// Helper function to create test catalog with models
func createTestCatalogForModels() catalogs.Reader {
	catalog, _ := catalogs.New()

	// Add authors
	openai := catalogs.Author{
		ID:   catalogs.AuthorIDOpenAI,
		Name: "OpenAI",
	}
	anthropic := catalogs.Author{
		ID:   catalogs.AuthorIDAnthropic,
		Name: "Anthropic",
	}
	catalog.SetAuthor(openai)
	catalog.SetAuthor(anthropic)

	// Add providers
	provider := catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
	}
	catalog.SetProvider(provider)

	// Add models with various features
	gpt4 := catalogs.Model{
		ID:   "gpt-4",
		Name: "GPT-4",
		Authors: []catalogs.Author{
			{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"},
		},
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
			ToolCalls: true,
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 8192,
		},
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.TokenPricing{
				Input: &catalogs.TokenCost{
					Per1M: 30.0,
				},
				Output: &catalogs.TokenCost{
					Per1M: 60.0,
				},
			},
		},
	}

	claude := catalogs.Model{
		ID:   "claude-3-opus",
		Name: "Claude 3 Opus",
		Authors: []catalogs.Author{
			{ID: catalogs.AuthorIDAnthropic, Name: "Anthropic"},
		},
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
			ToolCalls: true,
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 200000,
		},
	}

	whisper := catalogs.Model{
		ID:   "whisper-1",
		Name: "Whisper",
		Authors: []catalogs.Author{
			{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"},
		},
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityAudio},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
		},
	}

	catalog.SetModel(gpt4)
	catalog.SetModel(claude)
	catalog.SetModel(whisper)

	return catalog
}
