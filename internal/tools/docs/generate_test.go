package docs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestWithOutputDir(t *testing.T) {
	gen := &Generator{}
	opt := WithOutputDir("/test/path")
	opt(gen)
	assert.Equal(t, "/test/path", gen.outputDir)
}

func TestWithVerbose(t *testing.T) {
	gen := &Generator{}
	opt := WithVerbose(true)
	opt(gen)
	assert.True(t, gen.verbose)

	opt = WithVerbose(false)
	opt(gen)
	assert.False(t, gen.verbose)
}

func TestNew(t *testing.T) {
	// Test with default options
	gen := New()
	assert.NotNil(t, gen)
	assert.Equal(t, "./docs", gen.outputDir)
	assert.False(t, gen.verbose)

	// Test with custom options
	gen = New(
		WithOutputDir("/custom/path"),
		WithVerbose(true),
	)
	assert.NotNil(t, gen)
	assert.Equal(t, "/custom/path", gen.outputDir)
	assert.True(t, gen.verbose)
}

func TestGenerateWithEmptyCatalog(t *testing.T) {
	tmpDir := t.TempDir()
	catalog, err := catalogs.New()
	require.NoError(t, err)

	gen := New(WithOutputDir(tmpDir))
	err = gen.Generate(context.Background(), catalog)
	assert.NoError(t, err)
}

func TestGenerateWithSimpleCatalog(t *testing.T) {
	tmpDir := t.TempDir()
	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add an author
	author := catalogs.Author{
		ID:   "openai",
		Name: "OpenAI",
	}
	err = catalog.SetAuthor(author)
	require.NoError(t, err)

	// Add a model
	model := catalogs.Model{
		ID:   "gpt-4",
		Name: "GPT-4",
		Authors: []catalogs.Author{
			{ID: "openai", Name: "OpenAI"},
		},
	}

	// Add a provider with the model
	provider := catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
		Models: map[string]*catalogs.Model{
			model.ID: &model,
		},
	}
	err = catalog.SetProvider(provider)
	require.NoError(t, err)

	gen := New(
		WithOutputDir(tmpDir),
		WithVerbose(false),
	)
	err = gen.Generate(context.Background(), catalog)
	assert.NoError(t, err)
}

func TestGenerateWithVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add test data
	provider := catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
	}
	err = catalog.SetProvider(provider)
	require.NoError(t, err)

	gen := New(
		WithOutputDir(tmpDir),
		WithVerbose(true), // Enable verbose mode
	)
	err = gen.Generate(context.Background(), catalog)
	assert.NoError(t, err)
}

// TestGenerateComprehensive tests the Generate function with various scenarios.
func TestGenerateComprehensive(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (catalogs.Catalog, error)
		verbose   bool
		wantErr   bool
		checkFunc func(t *testing.T, outputDir string)
	}{
		{
			name: "empty catalog",
			setupFunc: func() (catalogs.Catalog, error) {
				return catalogs.New()
			},
			verbose: false,
			wantErr: false,
			checkFunc: func(t *testing.T, outputDir string) {
				// Should create directories even for empty catalog
				assert.DirExists(t, filepath.Join(outputDir, "catalog"))
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "providers"))
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "authors"))
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "models"))
			},
		},
		{
			name: "catalog with providers only",
			setupFunc: func() (catalogs.Catalog, error) {
				catalog, err := catalogs.New()
				if err != nil {
					return nil, err
				}

				providers := []catalogs.Provider{
					{ID: catalogs.ProviderIDOpenAI, Name: "OpenAI"},
					{ID: catalogs.ProviderIDGroq, Name: "Groq"},
				}

				for _, p := range providers {
					if err := catalog.SetProvider(p); err != nil {
						return nil, err
					}
				}

				return catalog, nil
			},
			verbose: true, // Test verbose mode
			wantErr: false,
			checkFunc: func(t *testing.T, outputDir string) {
				// Check provider directories
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "providers", "openai"))
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "providers", "groq"))
			},
		},
		{
			name: "catalog with authors only",
			setupFunc: func() (catalogs.Catalog, error) {
				catalog, err := catalogs.New()
				if err != nil {
					return nil, err
				}

				authors := []catalogs.Author{
					{ID: "openai", Name: "OpenAI"},
					{ID: "anthropic", Name: "Anthropic"},
				}

				for _, a := range authors {
					if err := catalog.SetAuthor(a); err != nil {
						return nil, err
					}
				}

				return catalog, nil
			},
			verbose: false,
			wantErr: false,
			checkFunc: func(t *testing.T, outputDir string) {
				// Check author directories
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "authors", "openai"))
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "authors", "anthropic"))
			},
		},
		{
			name: "catalog with complete data",
			setupFunc: func() (catalogs.Catalog, error) {
				catalog, err := catalogs.New()
				if err != nil {
					return nil, err
				}

				// Add author
				author := catalogs.Author{
					ID:   "openai",
					Name: "OpenAI",
				}
				if err := catalog.SetAuthor(author); err != nil {
					return nil, err
				}

				// Add models
				models := []catalogs.Model{
					{
						ID:      "gpt-4",
						Name:    "GPT-4",
						Authors: []catalogs.Author{author},
					},
					{
						ID:      "gpt-3.5-turbo",
						Name:    "GPT-3.5 Turbo",
						Authors: []catalogs.Author{author},
					},
				}

				// Add provider with models
				provider := catalogs.Provider{
					ID:     catalogs.ProviderIDOpenAI,
					Name:   "OpenAI",
					Models: make(map[string]*catalogs.Model),
				}
				for _, m := range models {
					provider.Models[m.ID] = &m
				}
				if err := catalog.SetProvider(provider); err != nil {
					return nil, err
				}

				return catalog, nil
			},
			verbose: true,
			wantErr: false,
			checkFunc: func(t *testing.T, outputDir string) {
				// Check all components were generated
				assert.FileExists(t, filepath.Join(outputDir, "catalog", "README.md"))
				assert.FileExists(t, filepath.Join(outputDir, "catalog", "providers", "README.md"))
				assert.FileExists(t, filepath.Join(outputDir, "catalog", "authors", "README.md"))
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "providers", "openai"))
				assert.DirExists(t, filepath.Join(outputDir, "catalog", "authors", "openai"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			catalog, err := tt.setupFunc()
			require.NoError(t, err)

			gen := New(
				WithOutputDir(tmpDir),
				WithVerbose(tt.verbose),
			)

			err = gen.Generate(context.Background(), catalog)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, tmpDir)
				}
			}
		})
	}
}

// TestGenerateDirectoryCreationError tests error handling when directory creation fails.
func TestGenerateDirectoryCreationError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file where a directory should be
	blockerFile := filepath.Join(tmpDir, "catalog")
	err := os.WriteFile(blockerFile, []byte("blocking file"), 0644)
	require.NoError(t, err)

	catalog, err := catalogs.New()
	require.NoError(t, err)

	gen := New(WithOutputDir(tmpDir))

	err = gen.Generate(context.Background(), catalog)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating directory")
}

// TestGenerateCatalogIndexError tests error handling in catalog index generation.
func TestGenerateCatalogIndexError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create catalog directory as read-only after initial creation
	catalogDir := filepath.Join(tmpDir, "catalog")
	err := os.MkdirAll(catalogDir, 0755)
	require.NoError(t, err)

	// Create subdirectories
	for _, dir := range []string{"providers", "authors", "models"} {
		err := os.MkdirAll(filepath.Join(catalogDir, dir), 0755)
		require.NoError(t, err)
	}

	// Make catalog directory read-only to cause write error
	err = os.Chmod(catalogDir, 0555)
	require.NoError(t, err)
	defer os.Chmod(catalogDir, 0755) // Restore permissions for cleanup

	catalog, err := catalogs.New()
	require.NoError(t, err)

	gen := New(WithOutputDir(tmpDir))

	err = gen.Generate(context.Background(), catalog)
	assert.Error(t, err)
}

// TestGenerateWithLogoCopyFailure tests that logo copy failures don't fail generation.
func TestGenerateWithLogoCopyFailure(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add provider without logo
	provider := catalogs.Provider{
		ID:   "custom-provider",
		Name: "Custom Provider",
	}
	err = catalog.SetProvider(provider)
	require.NoError(t, err)

	// Add author without logo
	author := catalogs.Author{
		ID:   "custom-author",
		Name: "Custom Author",
	}
	err = catalog.SetAuthor(author)
	require.NoError(t, err)

	gen := New(
		WithOutputDir(tmpDir),
		WithVerbose(true), // Enable verbose to see warning
	)

	// Should succeed even though logos don't exist
	err = gen.Generate(context.Background(), catalog)
	require.NoError(t, err)

	// Verify documentation was still generated
	assert.DirExists(t, filepath.Join(tmpDir, "catalog", "providers", "custom-provider"))
	assert.DirExists(t, filepath.Join(tmpDir, "catalog", "authors", "custom-author"))
}

// TestGenerateModelDocsError tests error handling in model docs generation.
func TestGenerateModelDocsError(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add a model with invalid characters in ID that might cause file creation issues
	model := catalogs.Model{
		ID:   "model/with/slashes",
		Name: "Invalid Model",
	}

	// Add provider with the model
	provider := catalogs.Provider{
		ID:   catalogs.ProviderID("test-provider"),
		Name: "Test Provider",
		Models: map[string]*catalogs.Model{
			model.ID: &model,
		},
	}
	err = catalog.SetProvider(provider)
	require.NoError(t, err)

	// Create models directory as read-only
	modelsDir := filepath.Join(tmpDir, "catalog", "models")
	err = os.MkdirAll(modelsDir, 0755)
	require.NoError(t, err)

	// Create other required directories
	for _, dir := range []string{"providers", "authors"} {
		err := os.MkdirAll(filepath.Join(tmpDir, "catalog", dir), 0755)
		require.NoError(t, err)
	}

	gen := New(WithOutputDir(tmpDir))

	// This should handle the error gracefully
	err = gen.Generate(context.Background(), catalog)
	// The function may or may not error depending on OS handling of slashes
	// The important thing is it doesn't panic
	_ = err
}

// TestGenerateProviderDocsError tests error handling in provider docs generation.
func TestGenerateProviderDocsError(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add provider
	provider := catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
	}
	err = catalog.SetProvider(provider)
	require.NoError(t, err)

	// Create required directories
	catalogDir := filepath.Join(tmpDir, "catalog")
	err = os.MkdirAll(catalogDir, 0755)
	require.NoError(t, err)

	for _, dir := range []string{"authors", "models"} {
		err := os.MkdirAll(filepath.Join(catalogDir, dir), 0755)
		require.NoError(t, err)
	}

	// Create providers directory but make it read-only
	providersDir := filepath.Join(catalogDir, "providers")
	err = os.MkdirAll(providersDir, 0755)
	require.NoError(t, err)
	err = os.Chmod(providersDir, 0555)
	require.NoError(t, err)
	defer os.Chmod(providersDir, 0755)

	gen := New(WithOutputDir(tmpDir))

	err = gen.Generate(context.Background(), catalog)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generating provider docs")
}

// TestGenerateAuthorDocsError tests error handling in author docs generation.
func TestGenerateAuthorDocsError(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add author
	author := catalogs.Author{
		ID:   "openai",
		Name: "OpenAI",
	}
	err = catalog.SetAuthor(author)
	require.NoError(t, err)

	// Create required directories
	catalogDir := filepath.Join(tmpDir, "catalog")
	err = os.MkdirAll(catalogDir, 0755)
	require.NoError(t, err)

	for _, dir := range []string{"providers", "models"} {
		err := os.MkdirAll(filepath.Join(catalogDir, dir), 0755)
		require.NoError(t, err)
	}

	// Create authors directory but make it read-only
	authorsDir := filepath.Join(catalogDir, "authors")
	err = os.MkdirAll(authorsDir, 0755)
	require.NoError(t, err)
	err = os.Chmod(authorsDir, 0555)
	require.NoError(t, err)
	defer os.Chmod(authorsDir, 0755)

	gen := New(WithOutputDir(tmpDir))

	err = gen.Generate(context.Background(), catalog)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generating author docs")
}

// TestCopyLogosComprehensive tests the copyLogos function.
func TestCopyLogosComprehensive(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add providers and authors
	providers := []catalogs.Provider{
		{ID: catalogs.ProviderIDOpenAI, Name: "OpenAI"},
		{ID: catalogs.ProviderIDGroq, Name: "Groq"},
		{ID: "custom", Name: "Custom"}, // No logo exists
	}

	for _, p := range providers {
		err := catalog.SetProvider(p)
		require.NoError(t, err)
	}

	authors := []catalogs.Author{
		{ID: "openai", Name: "OpenAI"},
		{ID: "meta", Name: "Meta"},
		{ID: "custom", Name: "Custom"}, // No logo exists
	}

	for _, a := range authors {
		err := catalog.SetAuthor(a)
		require.NoError(t, err)
	}

	gen := New(
		WithOutputDir(tmpDir),
		WithVerbose(true),
	)

	// Create necessary directories
	logosDir := filepath.Join(tmpDir, "logos")
	err = os.MkdirAll(logosDir, 0755)
	require.NoError(t, err)

	// Call copyLogos - should handle missing logos gracefully
	err = gen.copyLogos(catalog)
	// Should not error even with missing logos
	assert.NoError(t, err)
}

// TestGenerateWithContextCancellation tests context cancellation handling.
func TestGenerateWithContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := catalogs.New()
	require.NoError(t, err)

	// Add lots of data to make generation take time
	for i := 0; i < 10; i++ {
		provider := catalogs.Provider{
			ID:   catalogs.ProviderID(fmt.Sprintf("provider-%d", i)),
			Name: fmt.Sprintf("Provider %d", i),
		}
		err := catalog.SetProvider(provider)
		require.NoError(t, err)

		author := catalogs.Author{
			ID:   catalogs.AuthorID(fmt.Sprintf("author-%d", i)),
			Name: fmt.Sprintf("Author %d", i),
		}
		err = catalog.SetAuthor(author)
		require.NoError(t, err)
	}

	gen := New(WithOutputDir(tmpDir))

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should complete even with cancelled context (current implementation doesn't check context)
	err = gen.Generate(ctx, catalog)
	// Current implementation doesn't check context, so it will succeed
	// This test documents current behavior
	_ = err
}
