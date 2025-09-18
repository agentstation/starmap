package modelsdev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// ModelsDevAPIURL is the URL for the models.dev API.
	ModelsDevAPIURL = "https://models.dev/api.json"
	// HTTPCacheTTL is the cache time-to-live for HTTP responses.
	HTTPCacheTTL = 1 * time.Hour
)

// HTTPClient handles HTTP downloading of models.dev api.json.
type HTTPClient struct {
	CacheDir string
	APIURL   string
	Client   *http.Client
}

// NewHTTPClient creates a new models.dev HTTP client.
func NewHTTPClient(outputDir string) *HTTPClient {
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}
	cacheDir := filepath.Join(outputDir, "models.dev")
	return &HTTPClient{
		CacheDir: cacheDir,
		APIURL:   ModelsDevAPIURL,
		Client:   &http.Client{Timeout: constants.DefaultHTTPTimeout},
	}
}

// EnsureAPI ensures the api.json is available and up to date.
func (c *HTTPClient) EnsureAPI(ctx context.Context) error {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(c.CacheDir, constants.DirPermissions); err != nil {
		return errors.WrapIO("create", "cache directory", err)
	}

	apiPath := c.GetAPIPath()

	// Check if cached file exists and is recent
	if c.isCacheValid(apiPath) {
		fmt.Printf("  ✅ Using cached api.json\n")
		return nil
	}

	// Download fresh api.json
	fmt.Printf("  📥 Downloading api.json from %s...\n", c.APIURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIURL, nil)
	if err != nil {
		return c.useCacheFallback(apiPath)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		fmt.Printf("  ⚠️  HTTP request failed: %v\n", err)
		return c.useCacheFallback(apiPath)
	}
	defer func() { _ = resp.Body.Close() }()

	// Only accept HTTP 200 OK
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  ⚠️  HTTP %d: %s\n", resp.StatusCode, resp.Status)
		return c.useCacheFallback(apiPath)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("  ⚠️  Failed to read response: %v\n", err)
		return c.useCacheFallback(apiPath)
	}

	// Validate minimum size (round number, ~1/3 of typical ~267KB)
	const minValidSize = 100000
	if len(data) < minValidSize {
		fmt.Printf("  ⚠️  Response too small (%d bytes, expected > %d)\n", len(data), minValidSize)
		return c.useCacheFallback(apiPath)
	}

	// Validate JSON structure
	var testParse map[string]any
	if err := json.Unmarshal(data, &testParse); err != nil {
		fmt.Printf("  ⚠️  Invalid JSON response: %v\n", err)
		return c.useCacheFallback(apiPath)
	}

	// Check for expected structure (should have provider keys)
	if len(testParse) < 5 {
		fmt.Printf("  ⚠️  JSON missing expected providers (found %d)\n", len(testParse))
		return c.useCacheFallback(apiPath)
	}

	// Write validated data to file
	if err := os.WriteFile(apiPath, data, constants.FilePermissions); err != nil {
		return errors.WrapIO("write", "api.json", err)
	}

	fmt.Printf("  ✅ Downloaded api.json successfully (%d KB)\n", len(data)/1024)
	return nil
}

// GetAPIPath returns the path to the cached api.json file.
func (c *HTTPClient) GetAPIPath() string {
	return filepath.Join(c.CacheDir, "api.json")
}

// Cleanup removes the cache directory.
func (c *HTTPClient) Cleanup() error {
	if _, err := os.Stat(c.CacheDir); os.IsNotExist(err) {
		return nil // Already cleaned up
	}
	return os.RemoveAll(c.CacheDir)
}

// isCacheValid checks if the cached api.json is recent enough.
func (c *HTTPClient) isCacheValid(apiPath string) bool {
	info, err := os.Stat(apiPath)
	if err != nil {
		return false // File doesn't exist
	}

	// Check if file is recent enough
	return time.Since(info.ModTime()) < HTTPCacheTTL
}

// useCacheFallback tries cached data first, then embedded as final fallback.
func (c *HTTPClient) useCacheFallback(apiPath string) error {
	// Try existing cache file (even if stale)
	if _, err := os.Stat(apiPath); err == nil {
		fmt.Printf("  📄 Using cached api.json (network unavailable)\n")
		return nil
	}

	// Fall back to embedded data as last resort
	return c.useEmbeddedFallback(apiPath)
}

// useEmbeddedFallback copies embedded api.json to cache when all else fails.
func (c *HTTPClient) useEmbeddedFallback(apiPath string) error {
	fmt.Printf("  📦 Using embedded api.json fallback...\n")

	// Read embedded api.json
	embeddedData, err := embedded.FS.ReadFile("sources/models.dev/api.json")
	if err != nil {
		return errors.WrapResource("read", "embedded api.json", "", err)
	}

	// Write to cache location
	if err := os.WriteFile(apiPath, embeddedData, constants.FilePermissions); err != nil {
		return errors.WrapIO("write", "api.json fallback", err)
	}

	fmt.Printf("  ✅ Using embedded api.json (all other sources failed)\n")
	return nil
}
