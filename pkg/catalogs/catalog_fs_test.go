package catalogs

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFS creates a test filesystem with sample catalog data
func testFS() fs.FS {
	return fstest.MapFS{
		"providers.yaml": &fstest.MapFile{
			Data: []byte(`- id: openai
  name: OpenAI
  api_key:
    name: OPENAI_API_KEY
    pattern: "sk-.*"
    header: "Authorization"
    scheme: "Bearer"
- id: anthropic
  name: Anthropic
  api_key:
    name: ANTHROPIC_API_KEY
    pattern: "sk-ant-.*"
    header: "x-api-key"
- id: groq
  name: Groq
  api_key:
    name: GROQ_API_KEY
    pattern: "gsk_.*"
    header: "Authorization"
    scheme: "Bearer"
`),
		},
		"authors.yaml": &fstest.MapFile{
			Data: []byte(`- id: openai
  name: OpenAI Inc.
  url: https://openai.com
- id: anthropic
  name: Anthropic
  url: https://anthropic.com
`),
		},
		"providers/openai/models/gpt-4.yaml": &fstest.MapFile{
			Data: []byte(`id: gpt-4
name: GPT-4
description: "Most capable GPT-4 model"
`),
		},
		"providers/anthropic/models/claude-3-opus.yaml": &fstest.MapFile{
			Data: []byte(`id: claude-3-opus
name: Claude 3 Opus
description: "Most capable Claude model"
`),
		},
		"providers/groq/models/meta-llama/llama-3.yaml": &fstest.MapFile{
			Data: []byte(`id: meta-llama/llama-3
name: Llama 3
description: "Open source LLM"
`),
		},
	}
}

// TestCatalogWithFS tests creating a catalog with a custom fs.FS
func TestCatalogWithFS(t *testing.T) {
	tests := []struct {
		name          string
		fs            fs.FS
		wantProviders int
		wantAuthors   int
		wantModels    int
		wantError     bool
	}{
		{
			name:          "valid test filesystem",
			fs:            testFS(),
			wantProviders: 3,
			wantAuthors:   2,
			wantModels:    3,
		},
		{
			name:          "empty filesystem",
			fs:            fstest.MapFS{},
			wantProviders: 0,
			wantAuthors:   0,
			wantModels:    0,
		},
		{
			name:          "nil filesystem (memory catalog)",
			fs:            nil,
			wantProviders: 0,
			wantAuthors:   0,
			wantModels:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, err := New(WithFS(tt.fs))
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, cat)

			// Check loaded data
			assert.Equal(t, tt.wantProviders, cat.Providers().Len())
			assert.Equal(t, tt.wantAuthors, cat.Authors().Len())
			assert.Equal(t, tt.wantModels, len(cat.GetAllModels()))
		})
	}
}

// TestCatalogWithPath tests creating a catalog from a directory path
func TestCatalogWithPath(t *testing.T) {
	// Create a temporary directory with test data
	tmpDir := t.TempDir()

	// Write test files
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "providers.yaml"),
		[]byte(`- id: test-provider
  name: Test Provider
`), constants.FilePermissions))

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "providers", "test-provider", "models"), constants.DirPermissions))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "providers", "test-provider", "models", "test-model.yaml"),
		[]byte(`id: test-model
name: Test Model
`), constants.FilePermissions))

	// Create catalog from path
	cat, err := New(WithPath(tmpDir))
	require.NoError(t, err)
	assert.NotNil(t, cat)

	// Verify data loaded
	assert.Equal(t, 1, cat.Providers().Len())
	assert.Equal(t, 1, len(cat.GetAllModels()))

	provider, err := cat.Provider("test-provider")
	assert.NoError(t, err)
	assert.Equal(t, "Test Provider", provider.Name)

	model, err := cat.FindModel("test-model")
	assert.NoError(t, err)
	assert.Equal(t, "Test Model", model.Name)
}

// TestCatalogWrite tests writing a catalog to disk
func TestCatalogWrite(t *testing.T) {
	// Create a catalog with test data
	cat, err := New(WithFS(testFS()))
	require.NoError(t, err)

	// Write to temporary directory
	tmpDir := t.TempDir()
	err = cat.(*catalog).Write(tmpDir)
	require.NoError(t, err)

	// Verify files were written
	assert.FileExists(t, filepath.Join(tmpDir, "providers.yaml"))
	assert.FileExists(t, filepath.Join(tmpDir, "authors.yaml"))
	assert.DirExists(t, filepath.Join(tmpDir, "providers"))

	// Load the written catalog and compare
	cat2, err := New(WithPath(tmpDir))
	require.NoError(t, err)

	assert.Equal(t, cat.Providers().Len(), cat2.Providers().Len())
	assert.Equal(t, cat.Authors().Len(), cat2.Authors().Len())
	assert.Equal(t, len(cat.GetAllModels()), len(cat2.GetAllModels()))
}

// TestCatalogLoadMalformed tests handling of malformed YAML
func TestCatalogLoadMalformed(t *testing.T) {
	malformedFS := fstest.MapFS{
		"providers.yaml": &fstest.MapFile{
			Data: []byte(`- id: test
  name: [this is invalid yaml
`),
		},
	}

	cat, err := New(WithFS(malformedFS))
	// Should handle malformed YAML gracefully
	assert.Error(t, err)
	assert.Nil(t, cat)
}

// TestCatalogNestedModels tests loading models from nested directories
func TestCatalogNestedModels(t *testing.T) {
	nestedFS := fstest.MapFS{
		"providers.yaml": &fstest.MapFile{
			Data: []byte(`- id: groq
  name: Groq
`),
		},
		"providers/groq/models/meta-llama/llama-3.1/70b.yaml": &fstest.MapFile{
			Data: []byte(`id: meta-llama/llama-3.1/70b
name: Llama 3.1 70B
`),
		},
		"providers/groq/models/openai/gpt-3.5.yaml": &fstest.MapFile{
			Data: []byte(`id: openai/gpt-3.5
name: GPT-3.5 on Groq
`),
		},
	}

	cat, err := New(WithFS(nestedFS))
	require.NoError(t, err)
	assert.Equal(t, 2, len(cat.GetAllModels()))

	// Verify hierarchical IDs are preserved
	model1, err := cat.FindModel("meta-llama/llama-3.1/70b")
	assert.NoError(t, err)
	assert.Equal(t, "Llama 3.1 70B", model1.Name)

	model2, err := cat.FindModel("openai/gpt-3.5")
	assert.NoError(t, err)
	assert.Equal(t, "GPT-3.5 on Groq", model2.Name)
}

// TestCatalogConcurrentAccess tests thread-safe access to catalog
func TestCatalogConcurrentAccess(t *testing.T) {
	cat, err := New(WithFS(testFS()))
	require.NoError(t, err)

	// Run concurrent operations
	done := make(chan bool, 3)

	// Reader 1
	go func() {
		for i := 0; i < 100; i++ {
			_ = len(cat.GetAllModels())
			_ = cat.Providers().Len()
		}
		done <- true
	}()

	// Reader 2
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = cat.FindModel("gpt-4")
			_, _ = cat.Provider("openai")
		}
		done <- true
	}()

	// Writer
	go func() {
		// Get or create a test provider to hold models
		provider, err := cat.Provider("test-provider")
		if err != nil {
			provider = Provider{
				ID:     "test-provider",
				Name:   "Test Provider",
				Models: make(map[string]Model),
			}
		}
		for i := 0; i < 100; i++ {
			model := Model{
				ID:   "test-" + string(rune(i)),
				Name: "Test Model",
			}
			provider.Models[model.ID] = model
		}
		_ = cat.SetProvider(provider)
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
}

// TestMemoryCatalog tests a pure memory catalog without filesystem
func TestMemoryCatalog(t *testing.T) {
	cat, err := New() // No options = memory catalog
	require.NoError(t, err)
	assert.NotNil(t, cat)

	// Should start empty
	assert.Equal(t, 0, len(cat.GetAllModels()))
	assert.Equal(t, 0, cat.Providers().Len())

	// Add data programmatically
	provider := Provider{
		ID:   "test",
		Name: "Test Provider",
		Models: map[string]Model{
			"test-model": {
				ID:   "test-model",
				Name: "Test Model",
			},
		},
	}
	err = cat.SetProvider(provider)
	assert.NoError(t, err)

	// Verify data
	assert.Equal(t, 1, cat.Providers().Len())
	assert.Equal(t, 1, len(cat.GetAllModels()))

	// Memory catalog should not support Save
	if persistable, ok := cat.(Persistable); ok {
		err = persistable.Save()
		assert.Error(t, err, "memory catalog should not support Save")
	}
}

// TestCatalogCopy tests deep copying of catalogs
func TestCatalogCopy(t *testing.T) {
	original, err := New(WithFS(testFS()))
	require.NoError(t, err)

	// Create a copy
	copied, err := original.Copy()
	require.NoError(t, err)

	// Verify copy has same data
	assert.Equal(t, len(original.GetAllModels()), len(copied.GetAllModels()))
	assert.Equal(t, original.Providers().Len(), copied.Providers().Len())

	// Modify original by adding a model to an existing provider
	provider, err := original.Provider("openai")
	assert.NoError(t, err)
	if provider.Models == nil {
		provider.Models = make(map[string]Model)
	}
	provider.Models["new-model"] = Model{
		ID:   "new-model",
		Name: "New Model",
	}
	err = original.SetProvider(provider)
	assert.NoError(t, err)

	// Copy should not be affected
	assert.Equal(t, len(original.GetAllModels())-1, len(copied.GetAllModels()))
}

// BenchmarkCatalogLoad benchmarks loading catalogs
func BenchmarkCatalogLoad(b *testing.B) {
	testData := testFS()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = New(WithFS(testData))
	}
}

// BenchmarkCatalogWalk benchmarks walking catalog files
func BenchmarkCatalogWalk(b *testing.B) {
	cat, _ := New(WithFS(testFS()))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = len(cat.GetAllModels())
	}
}

// BenchmarkCatalogCopy benchmarks copying catalogs
func BenchmarkCatalogCopy(b *testing.B) {
	cat, _ := New(WithFS(testFS()))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = cat.Copy()
	}
}
