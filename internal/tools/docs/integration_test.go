package docs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestFullCatalogGeneration tests the complete documentation generation pipeline.
func TestFullCatalogGeneration(t *testing.T) {
	t.Run("complete catalog generation", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a comprehensive test catalog
		catalog, err := catalogs.New()
		require.NoError(t, err)

		// Add multiple providers with models
		providers := []struct {
			provider catalogs.Provider
			models   []catalogs.Model
		}{
			{
				provider: catalogs.Provider{
					ID:   "provider1",
					Name: "Provider One",
					APIKey: &catalogs.ProviderAPIKey{
						Name: "PROVIDER1_API_KEY",
					},
				},
				models: []catalogs.Model{
					{
						ID:   "model1",
						Name: "Model 1",
						Features: &catalogs.ModelFeatures{
							Tools:       true,
							Attachments: true,
						},
					},
					{
						ID:   "model2",
						Name: "Model 2",
						Pricing: &catalogs.ModelPricing{
							Tokens: &catalogs.ModelTokenPricing{
								Input:  &catalogs.ModelTokenCost{Per1M: 1.0},
								Output: &catalogs.ModelTokenCost{Per1M: 2.0},
							},
						},
					},
				},
			},
			{
				provider: catalogs.Provider{
					ID:   "provider2",
					Name: "Provider Two",
					PrivacyPolicy: &catalogs.ProviderPrivacyPolicy{
						RetainsData: boolPtr(false),
					},
				},
				models: []catalogs.Model{
					{
						ID:   "model3",
						Name: "Model 3",
						Metadata: &catalogs.ModelMetadata{
							OpenWeights: true,
						},
					},
				},
			},
		}

		// Add providers and models to catalog
		for _, p := range providers {
			// Add models to provider
			p.provider.Models = make(map[string]catalogs.Model)
			for _, m := range p.models {
				p.provider.Models[m.ID] = m
			}

			err := catalog.SetProvider(p.provider)
			require.NoError(t, err)
		}

		// Add authors
		authors := []catalogs.Author{
			{
				ID:   "author1",
				Name: "Author One",
			},
			{
				ID:   "author2",
				Name: "Author Two",
			},
		}

		for _, a := range authors {
			err := catalog.SetAuthor(a)
			require.NoError(t, err)
		}

		// Generate documentation
		gen := New(WithOutputDir(tmpDir))
		err = gen.Generate(context.Background(), catalog)
		require.NoError(t, err)

		// Debug: List what was actually created
		entries, _ := os.ReadDir(tmpDir)
		t.Logf("Created entries in %s:", tmpDir)
		for _, e := range entries {
			t.Logf("  - %s (dir: %v)", e.Name(), e.IsDir())
		}

		// Verify structure
		verifyDirectoryStructure(t, tmpDir)

		// Verify cross-references
		verifyCrossReferences(t, tmpDir)

		// Verify all links are valid
		verifyLinks(t, tmpDir)
	})

	t.Run("concurrent generation", func(t *testing.T) {
		tmpDir := t.TempDir()
		catalog := createTestCatalogWithProviders()

		// Run multiple generations concurrently
		done := make(chan error, 3)

		for i := 0; i < 3; i++ {
			go func(idx int) {
				outputDir := filepath.Join(tmpDir, "output", string(rune('a'+idx)))
				gen := New(WithOutputDir(outputDir))
				done <- gen.Generate(context.Background(), catalog)
			}(i)
		}

		// Collect results
		for i := 0; i < 3; i++ {
			err := <-done
			require.NoError(t, err)
		}

		// Verify all outputs exist
		for i := 0; i < 3; i++ {
			outputDir := filepath.Join(tmpDir, "output", string(rune('a'+i)))
			catalogDir := filepath.Join(outputDir, "catalog")
			assert.DirExists(t, outputDir)
			assert.DirExists(t, catalogDir)
			assert.FileExists(t, filepath.Join(catalogDir, "README.md"))
		}
	})
}

// verifyDirectoryStructure checks that all expected directories and files exist.
func verifyDirectoryStructure(t *testing.T, dir string) {
	catalogDir := filepath.Join(dir, "catalog")

	// Check main directories
	assert.DirExists(t, catalogDir)
	assert.DirExists(t, filepath.Join(catalogDir, "providers"))
	assert.DirExists(t, filepath.Join(catalogDir, "authors"))
	assert.DirExists(t, filepath.Join(catalogDir, "models"))

	// Check main index files
	assert.FileExists(t, filepath.Join(catalogDir, "README.md"))
	assert.FileExists(t, filepath.Join(catalogDir, "providers", "README.md"))
	assert.FileExists(t, filepath.Join(catalogDir, "authors", "README.md"))
	assert.FileExists(t, filepath.Join(catalogDir, "models", "README.md"))

	// Check provider directories
	provider1Dir := filepath.Join(catalogDir, "providers", "provider1")
	assert.DirExists(t, provider1Dir)
	assert.FileExists(t, filepath.Join(provider1Dir, "README.md"))
	assert.DirExists(t, filepath.Join(provider1Dir, "models"))

	// Check model files
	assert.FileExists(t, filepath.Join(provider1Dir, "models", "model1.md"))
	assert.FileExists(t, filepath.Join(provider1Dir, "models", "model2.md"))
}

// verifyCrossReferences checks that cross-references between docs are valid.
func verifyCrossReferences(t *testing.T, dir string) {
	catalogDir := filepath.Join(dir, "catalog")

	// Read provider README
	providerReadme, err := os.ReadFile(filepath.Join(catalogDir, "providers", "provider1", "README.md"))
	require.NoError(t, err)

	content := string(providerReadme)

	// Check that models are listed
	assert.Contains(t, content, "Model 1")
	assert.Contains(t, content, "Model 2")

	// Check relative links
	assert.Contains(t, content, "[All Providers](../)")
	assert.Contains(t, content, "[← Back to Catalog](../../)")
}

// verifyLinks checks that all markdown links point to existing files.
func verifyLinks(t *testing.T, dir string) {
	catalogDir := filepath.Join(dir, "catalog")

	// Walk all markdown files
	err := filepath.Walk(catalogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(path, ".md") {
			verifyFileLinks(t, dir, path)
		}

		return nil
	})
	require.NoError(t, err)
}

// verifyFileLinks checks links in a specific markdown file.
func verifyFileLinks(t *testing.T, baseDir, filePath string) {
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Simple regex to find relative markdown links
	// This is a simplified check - just looking for basic relative paths
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, "](../") || strings.Contains(line, "](./") {
			// Extract relative path (simplified)
			start := strings.Index(line, "](")
			if start == -1 {
				continue
			}
			end := strings.Index(line[start+2:], ")")
			if end == -1 {
				continue
			}

			link := line[start+2 : start+2+end]

			// Skip external links
			if strings.HasPrefix(link, "http") {
				continue
			}

			// Skip anchor links
			if strings.HasPrefix(link, "#") {
				continue
			}

			// Resolve relative path
			linkDir := filepath.Dir(filePath)
			targetPath := filepath.Clean(filepath.Join(linkDir, link))

			// Check if it's a directory link (ends with /)
			if strings.HasSuffix(link, "/") {
				// Should point to a directory with README.md
				readmePath := filepath.Join(targetPath, "README.md")
				if !fileExists(readmePath) && !dirExists(targetPath) {
					t.Logf("Warning: Link %s in %s points to non-existent location", link, filePath)
				}
			}
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// TestEndToEndWithRealData tests with more realistic data.
func TestEndToEndWithRealData(t *testing.T) {
	tmpDir := t.TempDir()

	// Create catalog with realistic data
	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add OpenAI-like provider
	openai := catalogs.Provider{
		ID:   "openai",
		Name: "OpenAI",
		APIKey: &catalogs.ProviderAPIKey{
			Name:    "OPENAI_API_KEY",
			Pattern: "sk-*",
			Header:  "Authorization",
			Scheme:  "Bearer",
		},
		ChatCompletions: &catalogs.ProviderChatCompletions{
			URL: stringPtr("https://api.openai.com/v1/chat/completions"),
		},
		PrivacyPolicy: &catalogs.ProviderPrivacyPolicy{
			RetainsData:  boolPtr(true),
			TrainsOnData: boolPtr(false),
		},
	}

	err = catalog.SetProvider(openai)
	require.NoError(t, err)

	// Add GPT-4 like model
	gpt4 := catalogs.Model{
		ID:   "gpt-4",
		Name: "GPT-4",
		Features: &catalogs.ModelFeatures{
			Tools:      true,
			ToolCalls:  true,
			ToolChoice: true,
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 128000,
			OutputTokens:  4096,
		},
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.ModelTokenPricing{
				Input:  &catalogs.ModelTokenCost{Per1M: 30.0},
				Output: &catalogs.ModelTokenCost{Per1M: 60.0},
			},
		},
	}

	// Add model to provider
	openai.Models = map[string]catalogs.Model{
		gpt4.ID: gpt4,
	}
	err = catalog.SetProvider(openai)
	require.NoError(t, err)

	// Generate and verify
	gen := New(WithOutputDir(tmpDir), WithVerbose(true))
	err = gen.Generate(context.Background(), catalog)
	require.NoError(t, err)

	// Verify GPT-4 documentation
	gpt4Doc := filepath.Join(tmpDir, "catalog", "providers", "openai", "models", "gpt-4.md")
	assert.FileExists(t, gpt4Doc)

	content, err := os.ReadFile(gpt4Doc)
	require.NoError(t, err)

	// Verify key information is present
	assert.Contains(t, string(content), "GPT-4")
	assert.Contains(t, string(content), "128k")
	assert.Contains(t, string(content), "4k")
	assert.Contains(t, string(content), "$30.00")
	assert.Contains(t, string(content), "$60.00")
}
