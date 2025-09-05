package docs

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentstation/starmap/pkg/catalogs"
)

//go:embed testdata/*
//go:embed testdata/logos/*
//go:embed testdata/logos/providers/*
var testEmbedFS embed.FS

func TestNewLogoCopier(t *testing.T) {
	lc := NewLogoCopier(testEmbedFS, "testdata", "/tmp/target")

	assert.NotNil(t, lc)
	assert.Equal(t, "testdata", lc.sourceDir)
	assert.Equal(t, "/tmp/target", lc.targetDir)
}

func TestCopyProviderLogos(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test LogoCopier with the testEmbedFS
	lc := NewLogoCopier(testEmbedFS, "testdata", tempDir)

	providers := []*catalogs.Provider{
		{ID: catalogs.ProviderIDOpenAI, Name: "OpenAI"},
		{ID: catalogs.ProviderIDAnthropic, Name: "Anthropic"},
	}

	err := lc.CopyProviderLogos(providers)
	require.NoError(t, err)

	// Check that logo directory was created
	logoDir := filepath.Join(tempDir, "assets", "logos", "providers")
	assert.DirExists(t, logoDir)
}

func TestCopyAuthorLogos(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test LogoCopier with the testEmbedFS
	lc := NewLogoCopier(testEmbedFS, "testdata", tempDir)

	authors := []*catalogs.Author{
		{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"},
		{ID: catalogs.AuthorIDMeta, Name: "Meta"},
	}

	err := lc.CopyAuthorLogos(authors)
	require.NoError(t, err)

	// Check that logo directory was created
	logoDir := filepath.Join(tempDir, "assets", "logos", "authors")
	assert.DirExists(t, logoDir)
}

// TestCopyProviderLogo is omitted as it requires mocking embed.FS

// TestCopyAuthorLogo is omitted as it requires mocking embed.FS

// TestCopyLogo is omitted as it requires mocking embed.FS

// TestCopyAllLogos is omitted as it requires mocking embed.FS

func TestGetLogoPath(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		logoType string
		expected string
	}{
		{
			name:     "provider logo",
			id:       "openai",
			logoType: "providers",
			expected: "../../assets/logos/providers/openai.svg",
		},
		{
			name:     "author logo",
			id:       "meta",
			logoType: "authors",
			expected: "../../assets/logos/authors/meta.svg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLogoPath(tt.id, tt.logoType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetProviderLogoPath(t *testing.T) {
	providerID := catalogs.ProviderIDOpenAI
	expected := "../../assets/logos/providers/openai.svg"

	result := getProviderLogoPath(providerID)
	assert.Equal(t, expected, result)
}

func TestGetAuthorLogoPath(t *testing.T) {
	authorID := catalogs.AuthorIDAnthropic
	expected := "../../assets/logos/authors/anthropic.svg"

	result := getAuthorLogoPath(authorID)
	assert.Equal(t, expected, result)
}

func TestLogoHTML(t *testing.T) {
	tests := []struct {
		name     string
		logoPath string
		alt      string
		width    int
		height   int
		expected string
	}{
		{
			name:     "standard logo",
			logoPath: "/path/to/logo.svg",
			alt:      "Test Logo",
			width:    32,
			height:   32,
			expected: `<img src="/path/to/logo.svg" alt="Test Logo" width="32" height="32" style="vertical-align: middle;">`,
		},
		{
			name:     "large logo",
			logoPath: "/path/to/big.png",
			alt:      "Big Logo",
			width:    64,
			height:   64,
			expected: `<img src="/path/to/big.png" alt="Big Logo" width="64" height="64" style="vertical-align: middle;">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logoHTML(tt.logoPath, tt.alt, tt.width, tt.height)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProviderLogoHTML(t *testing.T) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
	}

	result := providerLogoHTML(provider)

	assert.Contains(t, result, "openai.svg")
	assert.Contains(t, result, "alt=\"OpenAI\"")
	assert.Contains(t, result, "width=\"32\"")
	assert.Contains(t, result, "height=\"32\"")
}

func TestAuthorLogoHTML(t *testing.T) {
	author := &catalogs.Author{
		ID:   catalogs.AuthorIDMeta,
		Name: "Meta",
	}

	result := authorLogoHTML(author)

	assert.Contains(t, result, "meta.svg")
	assert.Contains(t, result, "alt=\"Meta\"")
	assert.Contains(t, result, "width=\"32\"")
	assert.Contains(t, result, "height=\"32\"")
}

func TestOptimizeSVG(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "remove newlines and tabs",
			input: `<svg>
				<rect />
				<circle />
			</svg>`,
			expected: `<svg><rect /><circle /></svg>`,
		},
		{
			name:     "collapse multiple spaces",
			input:    `<svg>  <rect    width="10"  />  </svg>`,
			expected: `<svg><rect width="10" /></svg>`,
		},
		{
			name:     "remove spaces around tags",
			input:    `<svg> <rect /> <circle /> </svg>`,
			expected: `<svg><rect /><circle /></svg>`,
		},
		{
			name:     "handle carriage returns",
			input:    "<svg>\r\n<rect />\r\n</svg>",
			expected: "<svg><rect /></svg>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizeSVG([]byte(tt.input))
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestCreateFallbackLogo(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "fallback.svg")

	// Test with normal name
	err := createFallbackLogo("OpenAI", outputPath)
	require.NoError(t, err)

	// Check that file was created
	assert.FileExists(t, outputPath)

	// Read and check content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	svg := string(content)

	assert.Contains(t, svg, "<svg")
	assert.Contains(t, svg, ">O<") // First letter
	assert.Contains(t, svg, "width=\"32\"")
	assert.Contains(t, svg, "height=\"32\"")

	// Test with empty name
	outputPath2 := filepath.Join(tempDir, "fallback2.svg")
	err = createFallbackLogo("", outputPath2)
	require.NoError(t, err)

	content2, _ := os.ReadFile(outputPath2)
	assert.Contains(t, string(content2), ">?<") // Question mark for empty name
}

func TestEnsureLogosExist(t *testing.T) {
	tempDir := t.TempDir()

	providers := []*catalogs.Provider{
		{ID: catalogs.ProviderIDOpenAI, Name: "OpenAI"},
		{ID: catalogs.ProviderIDAnthropic, Name: "Anthropic"},
	}

	authors := []*catalogs.Author{
		{ID: catalogs.AuthorIDMeta, Name: "Meta"},
		{ID: catalogs.AuthorIDGoogle, Name: "Google"},
	}

	err := ensureLogosExist(providers, authors, tempDir)
	require.NoError(t, err)

	// Check that all fallback logos were created
	providerLogoDir := filepath.Join(tempDir, "assets", "logos", "providers")
	assert.FileExists(t, filepath.Join(providerLogoDir, "openai.svg"))
	assert.FileExists(t, filepath.Join(providerLogoDir, "anthropic.svg"))

	authorLogoDir := filepath.Join(tempDir, "assets", "logos", "authors")
	assert.FileExists(t, filepath.Join(authorLogoDir, "meta.svg"))
	assert.FileExists(t, filepath.Join(authorLogoDir, "google.svg"))

	// Run again - should not error even if files exist
	err = ensureLogosExist(providers, authors, tempDir)
	assert.NoError(t, err)
}

func TestEnsureLogosExistWithExistingLogos(t *testing.T) {
	tempDir := t.TempDir()

	// Pre-create one logo
	providerLogoDir := filepath.Join(tempDir, "assets", "logos", "providers")
	os.MkdirAll(providerLogoDir, 0755)
	existingLogo := filepath.Join(providerLogoDir, "openai.svg")
	os.WriteFile(existingLogo, []byte("<svg>Existing</svg>"), 0644)

	providers := []*catalogs.Provider{
		{ID: catalogs.ProviderIDOpenAI, Name: "OpenAI"},
		{ID: catalogs.ProviderIDAnthropic, Name: "Anthropic"},
	}

	authors := []*catalogs.Author{}

	err := ensureLogosExist(providers, authors, tempDir)
	require.NoError(t, err)

	// Check that existing logo was not overwritten
	content, _ := os.ReadFile(existingLogo)
	assert.Equal(t, "<svg>Existing</svg>", string(content))

	// Check that missing logo was created
	assert.FileExists(t, filepath.Join(providerLogoDir, "anthropic.svg"))
}

func TestCopyAllLogos(t *testing.T) {
	tempTargetDir := t.TempDir()

	// Create LogoCopier with the test embedded FS
	lc := &LogoCopier{
		embedFS:   testEmbedFS,
		sourceDir: "testdata",
		targetDir: tempTargetDir,
	}

	// Test successful copy
	err := lc.CopyAllLogos()
	require.NoError(t, err)

	// Verify all files were copied
	expectedTargetDir := filepath.Join(tempTargetDir, "assets", "logos")

	// Check root level logo
	testLogoPath := filepath.Join(expectedTargetDir, "test.svg")
	assert.FileExists(t, testLogoPath)
	content, err := os.ReadFile(testLogoPath)
	require.NoError(t, err)
	assert.Equal(t, "<svg>test logo</svg>", string(content))

	// Check subdirectory logos
	openaiPath := filepath.Join(expectedTargetDir, "providers", "openai.svg")
	assert.FileExists(t, openaiPath)
	content, err = os.ReadFile(openaiPath)
	require.NoError(t, err)
	assert.Equal(t, "<svg>openai logo</svg>", string(content))
}

func TestCopyAllLogosErrors(t *testing.T) {
	// Test with invalid source directory
	t.Run("non-existent source directory", func(t *testing.T) {
		tempDir := t.TempDir()
		lc := &LogoCopier{
			embedFS:   testEmbedFS,
			sourceDir: "nonexistent", // This directory doesn't exist in embedded FS
			targetDir: tempDir,
		}

		err := lc.CopyAllLogos()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "walking logos directory")
	})

	// Test with read-only target directory
	t.Run("target directory with no write permissions", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Cannot test permission errors as root")
		}

		tempDir := t.TempDir()
		readOnlyDir := filepath.Join(tempDir, "readonly", "nested")
		parentDir := filepath.Dir(readOnlyDir)
		require.NoError(t, os.MkdirAll(parentDir, 0755))
		require.NoError(t, os.Chmod(parentDir, 0555)) // Make parent read-only
		defer os.Chmod(parentDir, 0755)               // Restore permissions for cleanup

		lc := &LogoCopier{
			embedFS:   testEmbedFS,
			sourceDir: "testdata",
			targetDir: readOnlyDir,
		}

		err := lc.CopyAllLogos()
		assert.Error(t, err)
	})
}

func TestCopyLogoErrors(t *testing.T) {
	// Test with non-existent source file
	t.Run("source file does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		lc := &LogoCopier{
			embedFS:   testEmbedFS,
			sourceDir: "testdata",
			targetDir: tempDir,
		}

		err := lc.copyLogo("nonexistent.svg", tempDir, "logo.svg")
		assert.Error(t, err)
	})

	// Test with read-only target directory
	t.Run("target directory is read-only", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Cannot test permission errors as root")
		}

		tempDir := t.TempDir()
		readOnlyDir := filepath.Join(tempDir, "readonly")
		require.NoError(t, os.MkdirAll(readOnlyDir, 0555))

		lc := &LogoCopier{
			embedFS:   testEmbedFS,
			sourceDir: "testdata",
			targetDir: tempDir,
		}

		err := lc.copyLogo("testdata/logos/test.svg", readOnlyDir, "logo.svg")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "creating target file")
	})
}
