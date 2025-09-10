package modelsdev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
)

// mockAPI creates a valid models.dev-like JSON response
func mockAPI() map[string]interface{} {
	return map[string]interface{}{
		"openai": map[string]interface{}{
			"id":   "openai",
			"name": "OpenAI",
			"models": map[string]interface{}{
				"gpt-4": map[string]interface{}{
					"id":   "gpt-4",
					"name": "GPT-4",
					"cost": map[string]interface{}{
						"input":  0.03,
						"output": 0.06,
					},
				},
			},
		},
		"anthropic": map[string]interface{}{
			"id":   "anthropic",
			"name": "Anthropic",
		},
		"google": map[string]interface{}{
			"id":   "google",
			"name": "Google",
		},
		"deepseek": map[string]interface{}{
			"id":   "deepseek",
			"name": "DeepSeek",
		},
		"cerebras": map[string]interface{}{
			"id":   "cerebras",
			"name": "Cerebras",
		},
	}
}

// mockAPIJSON returns a JSON string of valid API data
func mockAPIJSON() string {
	api := mockAPI()
	data, _ := json.Marshal(api)
	return string(data)
}

// largeMockAPIJSON returns a large JSON string (>100KB)
func largeMockAPIJSON() string {
	api := mockAPI()

	// Add many models to reach >100KB
	for i := 0; i < 1000; i++ {
		modelID := fmt.Sprintf("model-%d", i)
		if openai, ok := api["openai"].(map[string]interface{}); ok {
			if models, ok := openai["models"].(map[string]interface{}); ok {
				models[modelID] = map[string]interface{}{
					"id":   modelID,
					"name": fmt.Sprintf("Model %d", i),
					"cost": map[string]interface{}{
						"input":  0.001,
						"output": 0.002,
					},
					"description": strings.Repeat("x", 100), // Add bulk
				}
			}
		}
	}

	data, _ := json.Marshal(api)
	return string(data)
}

func TestHTTPClient_EnsureAPI_SuccessfulDownload(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeMockAPIJSON()))
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()

	// Create client with mock server URL
	client := &HTTPClient{
		CacheDir: filepath.Join(tempDir, "models.dev"),
		APIURL:   server.URL,
		Client:   &http.Client{Timeout: constants.DefaultHTTPTimeout},
	}

	// Test successful download
	ctx := context.Background()
	err := client.EnsureAPI(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify file was created
	apiPath := client.GetAPIPath()
	if _, err := os.Stat(apiPath); err != nil {
		t.Fatalf("API file not created: %v", err)
	}

	// Verify content is correct
	data, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatalf("Failed to read API file: %v", err)
	}

	if len(data) < 100000 {
		t.Errorf("Expected file size > 100KB, got %d bytes", len(data))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Downloaded data is not valid JSON: %v", err)
	}

	if len(parsed) < 5 {
		t.Errorf("Expected at least 5 providers, got %d", len(parsed))
	}
}

func TestHTTPClient_EnsureAPI_CachedFile(t *testing.T) {
	tempDir := t.TempDir()

	client := &HTTPClient{
		CacheDir: filepath.Join(tempDir, "models.dev"),
		APIURL:   "https://models.dev/api.json",
		Client:   &http.Client{Timeout: constants.DefaultHTTPTimeout},
	}

	// Create cache directory
	err := os.MkdirAll(client.CacheDir, constants.DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	// Create fresh cache file
	apiPath := client.GetAPIPath()
	err = os.WriteFile(apiPath, []byte(largeMockAPIJSON()), constants.FilePermissions)
	if err != nil {
		t.Fatalf("Failed to create cache file: %v", err)
	}

	// Test that cached file is used
	ctx := context.Background()
	err = client.EnsureAPI(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify cache file still exists and wasn't modified
	info, err := os.Stat(apiPath)
	if err != nil {
		t.Fatalf("Cache file disappeared: %v", err)
	}

	// File should be recent (just created)
	if time.Since(info.ModTime()) > time.Minute {
		t.Error("Cache file should be recent")
	}
}

func TestHTTPClient_EnsureAPI_HTTPFailureWithCache(t *testing.T) {
	// Create mock server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	tempDir := t.TempDir()

	client := &HTTPClient{
		CacheDir: filepath.Join(tempDir, "models.dev"),
		APIURL:   server.URL,
		Client:   &http.Client{Timeout: constants.DefaultHTTPTimeout},
	}

	// Create cache directory and stale cache file
	err := os.MkdirAll(client.CacheDir, constants.DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	apiPath := client.GetAPIPath()
	err = os.WriteFile(apiPath, []byte(largeMockAPIJSON()), constants.FilePermissions)
	if err != nil {
		t.Fatalf("Failed to create cache file: %v", err)
	}

	// Make cache file stale
	staleTime := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(apiPath, staleTime, staleTime)
	if err != nil {
		t.Fatalf("Failed to make cache file stale: %v", err)
	}

	// Test that stale cache is used when HTTP fails
	ctx := context.Background()
	err = client.EnsureAPI(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify cache file is still there
	if _, err := os.Stat(apiPath); err != nil {
		t.Fatalf("Cache file should still exist: %v", err)
	}
}

func TestHTTPClient_EnsureAPI_HTTPFailureWithEmbeddedFallback(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	tempDir := t.TempDir()

	client := &HTTPClient{
		CacheDir: filepath.Join(tempDir, "models.dev"),
		APIURL:   server.URL,
		Client:   &http.Client{Timeout: constants.DefaultHTTPTimeout},
	}

	// No cache file exists - should use embedded
	ctx := context.Background()
	err := client.EnsureAPI(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify embedded data was copied to cache
	apiPath := client.GetAPIPath()
	if _, err := os.Stat(apiPath); err != nil {
		t.Fatalf("Embedded data not copied to cache: %v", err)
	}

	// Verify it's valid JSON
	data, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Cache data is not valid JSON: %v", err)
	}
}

func TestHTTPClient_EnsureAPI_InvalidResponses(t *testing.T) {
	tests := []struct {
		name     string
		response string
		status   int
	}{
		{
			name:     "Too small response",
			response: `{"openai": {"id": "openai"}}`,
			status:   http.StatusOK,
		},
		{
			name:     "Invalid JSON",
			response: `{"invalid": json}`,
			status:   http.StatusOK,
		},
		{
			name:     "Too few providers",
			response: `{"openai": {"id": "openai"}, "anthropic": {"id": "anthropic"}}`,
			status:   http.StatusOK,
		},
		{
			name:     "HTTP 500 error",
			response: `Internal Server Error`,
			status:   http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			tempDir := t.TempDir()

			client := &HTTPClient{
				CacheDir: filepath.Join(tempDir, "models.dev"),
				APIURL:   server.URL,
				Client:   &http.Client{Timeout: constants.DefaultHTTPTimeout},
			}

			// Should fall back to embedded when HTTP response is invalid
			ctx := context.Background()
			err := client.EnsureAPI(ctx)
			if err != nil {
				t.Fatalf("Expected no error (should use embedded fallback), got: %v", err)
			}

			// Verify embedded data was used
			apiPath := client.GetAPIPath()
			if _, err := os.Stat(apiPath); err != nil {
				t.Fatalf("Expected fallback data to be written: %v", err)
			}
		})
	}
}

func TestHTTPClient_isCacheValid(t *testing.T) {
	tempDir := t.TempDir()

	client := &HTTPClient{
		CacheDir: filepath.Join(tempDir, "models.dev"),
	}

	apiPath := filepath.Join(tempDir, "api.json")

	// Non-existent file
	if client.isCacheValid(apiPath) {
		t.Error("Non-existent file should not be valid")
	}

	// Create fresh file
	err := os.WriteFile(apiPath, []byte("test"), constants.FilePermissions)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Fresh file should be valid
	if !client.isCacheValid(apiPath) {
		t.Error("Fresh file should be valid")
	}

	// Make file stale
	staleTime := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(apiPath, staleTime, staleTime)
	if err != nil {
		t.Fatalf("Failed to make file stale: %v", err)
	}

	// Stale file should not be valid
	if client.isCacheValid(apiPath) {
		t.Error("Stale file should not be valid")
	}
}
