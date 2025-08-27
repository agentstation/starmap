package modelsdev

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	ModelsDevAPIURL = "https://models.dev/api.json"
	HTTPCacheTTL    = 1 * time.Hour
)

// HTTPClient handles HTTP downloading of models.dev api.json
type HTTPClient struct {
	CacheDir string
	APIURL   string
	Client   *http.Client
}

// NewHTTPClient creates a new models.dev HTTP client
func NewHTTPClient(outputDir string) *HTTPClient {
	cacheDir := filepath.Join(outputDir, "models.dev-cache")
	return &HTTPClient{
		CacheDir: cacheDir,
		APIURL:   ModelsDevAPIURL,
		Client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// EnsureAPI ensures the api.json is available and up to date
func (c *HTTPClient) EnsureAPI() error {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(c.CacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	apiPath := c.GetAPIPath()

	// Check if cached file exists and is recent
	if c.isCacheValid(apiPath) {
		fmt.Printf("  ✅ Using cached api.json\n")
		return nil
	}

	// Download fresh api.json
	fmt.Printf("  📥 Downloading api.json from %s...\n", c.APIURL)

	resp, err := c.Client.Get(c.APIURL)
	if err != nil {
		return fmt.Errorf("downloading api.json: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error downloading api.json: %d %s", resp.StatusCode, resp.Status)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp(c.CacheDir, "api_*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer tempFile.Close()
	tempPath := tempFile.Name()

	// Copy response to temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("writing api.json: %w", err)
	}

	// Atomically move temp file to final location
	if err := os.Rename(tempPath, apiPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("moving api.json to final location: %w", err)
	}

	fmt.Printf("  ✅ Downloaded api.json successfully\n")
	return nil
}

// GetAPIPath returns the path to the cached api.json file
func (c *HTTPClient) GetAPIPath() string {
	return filepath.Join(c.CacheDir, "api.json")
}

// Cleanup removes the cache directory
func (c *HTTPClient) Cleanup() error {
	if _, err := os.Stat(c.CacheDir); os.IsNotExist(err) {
		return nil // Already cleaned up
	}
	return os.RemoveAll(c.CacheDir)
}

// isCacheValid checks if the cached api.json is recent enough
func (c *HTTPClient) isCacheValid(apiPath string) bool {
	info, err := os.Stat(apiPath)
	if err != nil {
		return false // File doesn't exist
	}

	// Check if file is recent enough
	return time.Since(info.ModTime()) < HTTPCacheTTL
}
