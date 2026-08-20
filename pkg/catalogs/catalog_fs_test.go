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

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestNewFromPathRejectsMissingAndQuarantinesCorruptRecords(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := NewFromPath(missing); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("NewFromPath missing error = %v, want errors.Is(os.ErrNotExist)", err)
	}

	corrupt := filepath.Join(t.TempDir(), "corrupt")
	if err := os.MkdirAll(corrupt, resourcepolicy.DirMode); err != nil {
		t.Fatalf("Mkdir corrupt: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(corrupt, "providers.yaml"),
		[]byte("- id: invalid\n  name: [unterminated\n"),
		resourcepolicy.FileMode,
	); err != nil {
		t.Fatalf("Write corrupt catalog: %v", err)
	}
	_, err := NewFromPath(corrupt)
	if err == nil {
		t.Fatal("corrupt configured catalog was treated as optional absence")
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("corrupt error = %T: %v, want *errors.ParseError", err, err)
	}

	corruptModel := filepath.Join(t.TempDir(), "corrupt-model")
	modelDir := filepath.Join(corruptModel, "providers", "test-provider", "models")
	if err := os.MkdirAll(modelDir, resourcepolicy.DirMode); err != nil {
		t.Fatalf("Mkdir corrupt model: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(corruptModel, "providers.yaml"),
		[]byte("- id: test-provider\n  name: Test Provider\n"),
		resourcepolicy.FileMode,
	); err != nil {
		t.Fatalf("Write provider index: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(modelDir, "broken.yaml"),
		[]byte("id: broken\nname: [unterminated\n"),
		resourcepolicy.FileMode,
	); err != nil {
		t.Fatalf("Write corrupt model: %v", err)
	}
	loaded, err := NewFromPath(corruptModel)
	if err != nil {
		t.Fatalf("corrupt model record should be quarantined: %v", err)
	}
	report := loaded.LoadReport()
	if report.Rejected != 1 || len(report.Issues) != 1 ||
		!stderrors.As(report.Issues[0].Err, &parseErr) {
		t.Fatalf("corrupt model report = %#v, want one *errors.ParseError issue", report)
	}
}

// testFS creates a test filesystem with sample catalog data.
func testFS() fs.FS {
	return fstest.MapFS{
		"providers.yaml": &fstest.MapFile{
			Data: []byte(`- id: openai
  name: OpenAI
  credentials:
    fields:
    - id: api-key
      kind: secret
      required: true
      environment: [OPENAI_API_KEY]
      pattern: "^sk-.*"
    profiles:
    - id: api-key
      primitive: api-key
      fields: [api-key]
      placements:
      - field: api-key
        kind: header
        name: Authorization
        scheme: bearer
    catalog_acquisition: {}
    inference:
      required: true
      alternatives: [api-key]
- id: anthropic
  name: Anthropic
  credentials:
    fields:
    - id: api-key
      kind: secret
      required: true
      environment: [ANTHROPIC_API_KEY]
      pattern: "^sk-ant-.*"
    profiles:
    - id: api-key
      primitive: api-key
      fields: [api-key]
      placements:
      - field: api-key
        kind: header
        name: x-api-key
        scheme: direct
    catalog_acquisition: {}
    inference:
      required: true
      alternatives: [api-key]
- id: groq
  name: Groq
  credentials:
    fields:
    - id: api-key
      kind: secret
      required: true
      environment: [GROQ_API_KEY]
      pattern: "^gsk_.*"
    profiles:
    - id: api-key
      primitive: api-key
      fields: [api-key]
      placements:
      - field: api-key
        kind: header
        name: Authorization
        scheme: bearer
    catalog_acquisition: {}
    inference:
      required: true
      alternatives: [api-key]
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
			assert.Equal(t, tt.wantModels, len(testBuilderModels(cat)))
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
`), resourcepolicy.FileMode))

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "providers", "test-provider", "models"), resourcepolicy.DirMode))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "providers", "test-provider", "models", "test-model.yaml"),
		[]byte(`id: test-model
name: Test Model
`), resourcepolicy.FileMode))

	// Create catalog from path
	cat, err := New(WithPath(tmpDir))
	require.NoError(t, err)
	assert.NotNil(t, cat)

	// Verify data loaded
	assert.Equal(t, 1, cat.Providers().Len())
	assert.Equal(t, 1, len(testBuilderModels(cat)))

	provider, err := cat.Provider("test-provider")
	assert.NoError(t, err)
	assert.Equal(t, "Test Provider", provider.Name)

	model, err := testBuilderFindModel(cat, "test-model")
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
	err = cat.SaveTo(tmpDir)
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
	assert.Equal(t, len(testBuilderModels(cat)), len(testBuilderModels(cat2)))
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
	}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := cat.SaveTo(tmpDir); err != nil {
		t.Fatalf("Save first generation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("unmanaged"), resourcepolicy.FileMode); err != nil {
		t.Fatalf("Write unmanaged root file: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(tmpDir, "providers", "replacement-provider", "logo.svg"),
		[]byte("<svg/>"),
		resourcepolicy.FileMode,
	); err != nil {
		t.Fatalf("Write unmanaged provider file: %v", err)
	}
	staleAuthorModels := filepath.Join(tmpDir, "authors", "replacement-author", "models")
	if err := os.MkdirAll(staleAuthorModels, resourcepolicy.DirMode); err != nil {
		t.Fatalf("Create obsolete author model tree: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(staleAuthorModels, "stale.yaml"),
		[]byte("id: stale\nname: Stale\n"),
		resourcepolicy.FileMode,
	); err != nil {
		t.Fatalf("Write obsolete author model: %v", err)
	}

	if err := cat.DeleteProviderModel(provider.ID, "stale-provider-model"); err != nil {
		t.Fatalf("DeleteProviderModel: %v", err)
	}
	if err := cat.SetProviderModel(provider.ID, Model{ID: "current-provider-model", Name: "Current"}); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	if err := cat.SaveTo(tmpDir); err != nil {
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
	if _, err := reloaded.Author("replacement-author"); err != nil {
		t.Fatalf("Author: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "authors", "replacement-author", "models")); !os.IsNotExist(err) {
		t.Fatalf("obsolete author model tree remains, error = %v", err)
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
	assert.Equal(t, 2, len(testBuilderModels(cat)))

	// Verify hierarchical IDs are preserved
	model1, err := testBuilderFindModel(cat, "meta-llama/llama-3.1/70b")
	assert.NoError(t, err)
	assert.Equal(t, "Llama 3.1 70B", model1.Name)

	model2, err := testBuilderFindModel(cat, "openai/gpt-3.5")
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
		for range 100 {
			_ = len(testBuilderModels(cat))
			_ = cat.Providers().Len()
		}
		done <- true
	}()

	// Reader 2
	go func() {
		for range 100 {
			_, _ = testBuilderFindModel(cat, "gpt-4")
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
		for i := range 100 {
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
	for range 3 {
		<-done
	}
}

// TestMemoryCatalog tests a pure memory catalog without filesystem.
func TestMemoryCatalog(t *testing.T) {
	cat := NewEmpty() // No options = memory catalog
	assert.NotNil(t, cat)

	// Should start empty
	assert.Equal(t, 0, len(testBuilderModels(cat)))
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
	assert.Equal(t, 1, len(testBuilderModels(cat)))
}

// TestCatalogCopy tests deep copying of catalogs.
func TestCatalogCopy(t *testing.T) {
	original, err := New(WithFS(testFS()))
	require.NoError(t, err)

	// Create a copy
	copied, err := original.Copy()
	require.NoError(t, err)

	// Verify copy has same data
	assert.Equal(t, len(testBuilderModels(original)), len(testBuilderModels(copied)))
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
	assert.Equal(t, len(testBuilderModels(original))-1, len(testBuilderModels(copied)))
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
		_ = len(testBuilderModels(cat))
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
