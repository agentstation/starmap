package modelsdev

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	cacheDir := filepath.Join(outputDir, "models.dev-cache")
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
		return errors.WrapResource("create", "request", c.APIURL, err)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return &errors.APIError{
			Provider: "models.dev",
			Endpoint: c.APIURL,
			Message:  "failed to download api.json",
			Err:      err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return &errors.APIError{
			Provider:   "models.dev",
			Endpoint:   c.APIURL,
			StatusCode: resp.StatusCode,
			Message:    resp.Status,
		}
	}

	// Create temporary file
	tempFile, err := os.CreateTemp(c.CacheDir, "api_*.json")
	if err != nil {
		return errors.WrapIO("create", "temp file", err)
	}
	defer func() { _ = tempFile.Close() }()
	tempPath := tempFile.Name()

	// Copy response to temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		_ = os.Remove(tempPath)
		return errors.WrapIO("write", "api.json", err)
	}

	// Atomically move temp file to final location
	if err := os.Rename(tempPath, apiPath); err != nil {
		_ = os.Remove(tempPath)
		return errors.WrapIO("move", "api.json", err)
	}

	fmt.Printf("  ✅ Downloaded api.json successfully\n")
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
