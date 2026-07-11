package catalogs

import (
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
)

func TestNewLocalDistinguishesMissingOptionalPathFromCorruptCatalog(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := NewLocal(missing); err != nil {
		t.Fatalf("missing optional path: %v", err)
	}
	if _, err := NewFromPath(missing); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("NewFromPath missing error = %v, want errors.Is(os.ErrNotExist)", err)
	}

	corrupt := filepath.Join(t.TempDir(), "corrupt")
	if err := os.MkdirAll(corrupt, constants.DirPermissions); err != nil {
		t.Fatalf("Mkdir corrupt: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(corrupt, "providers.yaml"),
		[]byte("- id: invalid\n  name: [unterminated\n"),
		constants.FilePermissions,
	); err != nil {
		t.Fatalf("Write corrupt catalog: %v", err)
	}
	_, err := NewLocal(corrupt)
	if err == nil {
		t.Fatal("corrupt configured catalog was treated as optional absence")
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("corrupt error = %T: %v, want *errors.ParseError", err, err)
	}

	corruptModel := filepath.Join(t.TempDir(), "corrupt-model")
	modelDir := filepath.Join(corruptModel, "providers", "test-provider", "models")
	if err := os.MkdirAll(modelDir, constants.DirPermissions); err != nil {
		t.Fatalf("Mkdir corrupt model: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(corruptModel, "providers.yaml"),
		[]byte("- id: test-provider\n  name: Test Provider\n"),
		constants.FilePermissions,
	); err != nil {
		t.Fatalf("Write provider index: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(modelDir, "broken.yaml"),
		[]byte("id: broken\nname: [unterminated\n"),
		constants.FilePermissions,
	); err != nil {
		t.Fatalf("Write corrupt model: %v", err)
	}
	_, err = NewLocal(corruptModel)
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("corrupt model error = %T: %v, want *errors.ParseError", err, err)
	}
}

// testFS creates a test filesystem with sample catalog data.
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

// TestCatalogWithFS tests creating a catalog with a custom fs.FS.
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
			assert.Equal(t, tt.wantModels, len(cat.Models().List()))
		})
	}
}

// TestCatalogWithPath tests creating a catalog from a directory path.
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
	assert.Equal(t, 1, len(cat.Models().List()))

	provider, err := cat.Provider("test-provider")
	assert.NoError(t, err)
	assert.Equal(t, "Test Provider", provider.Name)

	model, err := cat.FindModel("test-model")
	assert.NoError(t, err)
	assert.Equal(t, "Test Model", model.Name)
}

// TestCatalogWrite tests writing a catalog to disk.
func TestCatalogWrite(t *testing.T) {
	// Create a catalog with test data
	cat, err := New(WithFS(testFS()))
	require.NoError(t, err)

	// Write to temporary directory
	tmpDir := t.TempDir()
	err = cat.Save(save.WithPath(tmpDir))
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
	assert.Equal(t, len(cat.Models().List()), len(cat2.Models().List()))
}

func TestStaleCatalogRecordsDoNotReappearAfterSaveReload(t *testing.T) {
	tmpDir := t.TempDir()
	cat := NewEmpty()
	provider := Provider{
		ID:   "replacement-provider",
		Name: "Replacement Provider",
		Models: map[string]*Model{
			"stale-provider-model": {ID: "stale-provider-model", Name: "Stale"},
		},
	}
	if err := cat.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := cat.SetAuthor(Author{
		ID:   "replacement-author",
		Name: "Replacement Author",
		Models: map[string]*Model{
			"stale-author-model": {ID: "stale-author-model", Name: "Stale Author Model"},
		},
	}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := cat.Save(save.WithPath(tmpDir)); err != nil {
		t.Fatalf("Save first generation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("unmanaged"), constants.FilePermissions); err != nil {
		t.Fatalf("Write unmanaged root file: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(tmpDir, "providers", "replacement-provider", "logo.svg"),
		[]byte("<svg/>"),
		constants.FilePermissions,
	); err != nil {
		t.Fatalf("Write unmanaged provider file: %v", err)
	}

	if err := cat.DeleteProviderModel(provider.ID, "stale-provider-model"); err != nil {
		t.Fatalf("DeleteProviderModel: %v", err)
	}
	if err := cat.SetProviderModel(provider.ID, Model{ID: "current-provider-model", Name: "Current"}); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	if err := cat.SetAuthor(Author{
		ID:   "replacement-author",
		Name: "Replacement Author",
		Models: map[string]*Model{
			"current-author-model": {ID: "current-author-model", Name: "Current Author Model"},
		},
	}); err != nil {
		t.Fatalf("replace author: %v", err)
	}
	if err := cat.Save(); err != nil {
		t.Fatalf("Save replacement generation: %v", err)
	}

	reloaded, err := New(WithPath(tmpDir))
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if _, err := reloaded.ProviderModel(provider.ID, "stale-provider-model"); !pkgerrors.IsNotFound(err) {
		t.Fatalf("stale provider model reappeared, error = %v", err)
	}
	if _, err := reloaded.ProviderModel(provider.ID, "current-provider-model"); err != nil {
		t.Fatalf("current provider model missing: %v", err)
	}
	author, err := reloaded.Author("replacement-author")
	if err != nil {
		t.Fatalf("Author: %v", err)
	}
	if _, found := author.Models["stale-author-model"]; found {
		t.Fatal("stale author model reappeared")
	}
	if _, found := author.Models["current-author-model"]; !found {
		t.Fatal("current author model missing")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "notes.txt")); err != nil {
		t.Fatalf("unmanaged root file was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "providers", "replacement-provider", "logo.svg")); err != nil {
		t.Fatalf("unmanaged provider file was removed: %v", err)
	}
}

// TestCatalogLoadMalformed tests handling of malformed YAML.
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

// TestCatalogNestedModels tests loading models from nested directories.
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
	assert.Equal(t, 2, len(cat.Models().List()))

	// Verify hierarchical IDs are preserved
	model1, err := cat.FindModel("meta-llama/llama-3.1/70b")
	assert.NoError(t, err)
	assert.Equal(t, "Llama 3.1 70B", model1.Name)

	model2, err := cat.FindModel("openai/gpt-3.5")
	assert.NoError(t, err)
	assert.Equal(t, "GPT-3.5 on Groq", model2.Name)
}

// TestCatalogConcurrentAccess tests thread-safe access to catalog.
func TestCatalogConcurrentAccess(t *testing.T) {
	cat, err := New(WithFS(testFS()))
	require.NoError(t, err)

	// Run concurrent operations
	done := make(chan bool, 3)

	// Reader 1
	go func() {
		for i := 0; i < 100; i++ {
			_ = len(cat.Models().List())
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
				Models: make(map[string]*Model),
			}
		}
		for i := 0; i < 100; i++ {
			model := &Model{
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

// TestMemoryCatalog tests a pure memory catalog without filesystem.
func TestMemoryCatalog(t *testing.T) {
	cat := NewEmpty() // No options = memory catalog
	assert.NotNil(t, cat)

	// Should start empty
	assert.Equal(t, 0, len(cat.Models().List()))
	assert.Equal(t, 0, cat.Providers().Len())

	// Add data programmatically
	provider := Provider{
		ID:   "test",
		Name: "Test Provider",
		Models: map[string]*Model{
			"test-model": {
				ID:   "test-model",
				Name: "Test Model",
			},
		},
	}
	err := cat.SetProvider(provider)
	assert.NoError(t, err)

	// Verify data
	assert.Equal(t, 1, cat.Providers().Len())
	assert.Equal(t, 1, len(cat.Models().List()))
}

// TestCatalogCopy tests deep copying of catalogs.
func TestCatalogCopy(t *testing.T) {
	original, err := New(WithFS(testFS()))
	require.NoError(t, err)

	// Create a copy
	copied, err := original.Copy()
	require.NoError(t, err)

	// Verify copy has same data
	assert.Equal(t, len(original.Models().List()), len(copied.Models().List()))
	assert.Equal(t, original.Providers().Len(), copied.Providers().Len())

	// Modify original by adding a model to an existing provider
	provider, err := original.Provider("openai")
	assert.NoError(t, err)
	if provider.Models == nil {
		provider.Models = make(map[string]*Model)
	}
	provider.Models["new-model"] = &Model{
		ID:   "new-model",
		Name: "New Model",
	}
	err = original.SetProvider(provider)
	assert.NoError(t, err)

	// Copy should not be affected
	assert.Equal(t, len(original.Models().List())-1, len(copied.Models().List()))
}

// BenchmarkCatalogLoad benchmarks loading catalogs.
func BenchmarkCatalogLoad(b *testing.B) {
	testData := testFS()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = New(WithFS(testData))
	}
}

// BenchmarkCatalogWalk benchmarks walking catalog files.
func BenchmarkCatalogWalk(b *testing.B) {
	cat, _ := New(WithFS(testFS()))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = len(cat.Models().List())
	}
}

// BenchmarkCatalogCopy benchmarks copying catalogs.
func BenchmarkCatalogCopy(b *testing.B) {
	cat, _ := New(WithFS(testFS()))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = cat.Copy()
	}
}
