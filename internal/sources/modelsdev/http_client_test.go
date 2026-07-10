package modelsdev

import (
	"bytes"
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
	"github.com/agentstation/starmap/pkg/sources"
)

// mockAPI creates a valid models.dev-like JSON response
func mockAPI() map[string]any {
	return map[string]any{
		"openai": map[string]any{
			"id":   "openai",
			"name": "OpenAI",
			"models": map[string]any{
				"gpt-4": map[string]any{
					"id":   "gpt-4",
					"name": "GPT-4",
					"cost": map[string]any{
						"input":  0.03,
						"output": 0.06,
					},
				},
			},
		},
		"anthropic": map[string]any{
			"id":     "anthropic",
			"name":   "Anthropic",
			"models": map[string]any{},
		},
		"google": map[string]any{
			"id":     "google",
			"name":   "Google",
			"models": map[string]any{},
		},
		"deepseek": map[string]any{
			"id":     "deepseek",
			"name":   "DeepSeek",
			"models": map[string]any{},
		},
		"cerebras": map[string]any{
			"id":     "cerebras",
			"name":   "Cerebras",
			"models": map[string]any{},
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
		if openai, ok := api["openai"].(map[string]any); ok {
			if models, ok := openai["models"].(map[string]any); ok {
				models[modelID] = map[string]any{
					"id":   modelID,
					"name": fmt.Sprintf("Model %d", i),
					"cost": map[string]any{
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

func largeSchemaIncompatibleAPIJSON(t *testing.T) string {
	t.Helper()

	var api map[string]any
	if err := json.Unmarshal([]byte(largeMockAPIJSON()), &api); err != nil {
		t.Fatalf("unmarshal large mock API: %v", err)
	}
	openAI := api["openai"].(map[string]any)
	models := openAI["models"].(map[string]any)
	gpt4 := models["gpt-4"].(map[string]any)
	gpt4["limit"] = map[string]any{
		"context": map[string]any{"unexpected": true},
	}

	data, err := json.Marshal(api)
	if err != nil {
		t.Fatalf("marshal incompatible mock API: %v", err)
	}
	return string(data)
}

func semanticallyIncompleteAPIJSON(t *testing.T) string {
	t.Helper()

	var api map[string]any
	if err := json.Unmarshal([]byte(largeMockAPIJSON()), &api); err != nil {
		t.Fatalf("unmarshal large mock API: %v", err)
	}
	models := api["openai"].(map[string]any)["models"].(map[string]any)
	for index := 200; index < 1000; index++ {
		delete(models, fmt.Sprintf("model-%d", index))
	}
	models["gpt-4"].(map[string]any)["description"] = strings.Repeat("syntactically-valid-padding", 5000)

	data, err := json.Marshal(api)
	if err != nil {
		t.Fatalf("marshal semantically incomplete API: %v", err)
	}
	return string(data)
}

func TestHTTPClientSemanticPromotionPreservesLastKnownGoodOnCompletenessRegression(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(semanticallyIncompleteAPIJSON(t)))
	}))
	defer server.Close()

	now := time.Date(2026, time.July, 10, 20, 0, 0, 0, time.UTC)
	client := &HTTPClient{
		CacheDir: filepath.Join(t.TempDir(), "models.dev"), APIURL: server.URL,
		Client: server.Client(), nowFunc: func() time.Time { return now },
	}
	if err := os.MkdirAll(client.CacheDir, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	want := []byte(largeMockAPIJSON())
	apiPath := client.GetAPIPath()
	if err := os.WriteFile(apiPath, want, constants.FilePermissions); err != nil {
		t.Fatalf("write last-known-good cache: %v", err)
	}
	wantChecksum := checksumBytes(want)
	if err := client.writeCacheMetadata(apiPath, httpCacheMetadata{
		Version: httpCacheMetadataVersion, Origin: HTTPAcquisitionDownloaded,
		ContentChecksum: wantChecksum, ValidatedAt: now.Add(-2 * HTTPCacheTTL),
	}); err != nil {
		t.Fatalf("write cache metadata: %v", err)
	}

	result, err := client.AcquireAPI(context.Background())
	if err != nil {
		t.Fatalf("AcquireAPI: %v", err)
	}
	if result.Kind != HTTPAcquisitionStaleCache {
		t.Fatalf("acquisition = %q, want %q", result.Kind, HTTPAcquisitionStaleCache)
	}
	if len(result.Issues) != 1 || result.Issues[0].Code != sources.ObservationIssueCodeSchemaDrift {
		t.Fatalf("semantic rejection evidence = %#v", result.Issues)
	}
	got, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatalf("read retained cache: %v", err)
	}
	if !bytes.Equal(got, want) || checksumBytes(got) != wantChecksum {
		t.Fatal("semantically incomplete download replaced the last-known-good cache")
	}
}

func TestHTTPClient_EnsureAPI_DoesNotPromoteSchemaIncompatibleDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeSchemaIncompatibleAPIJSON(t)))
	}))
	defer server.Close()

	client := &HTTPClient{
		CacheDir: filepath.Join(t.TempDir(), "models.dev"),
		APIURL:   server.URL,
		Client:   &http.Client{Timeout: constants.DefaultHTTPTimeout},
	}
	if err := os.MkdirAll(client.CacheDir, constants.DirPermissions); err != nil {
		t.Fatalf("create cache directory: %v", err)
	}

	want := []byte(largeMockAPIJSON())
	apiPath := client.GetAPIPath()
	if err := os.WriteFile(apiPath, want, constants.FilePermissions); err != nil {
		t.Fatalf("write last-known-good cache: %v", err)
	}
	staleTime := time.Now().Add(-2 * HTTPCacheTTL)
	if err := os.Chtimes(apiPath, staleTime, staleTime); err != nil {
		t.Fatalf("age last-known-good cache: %v", err)
	}

	if err := client.EnsureAPI(context.Background()); err != nil {
		t.Fatalf("EnsureAPI should retain last-known-good cache: %v", err)
	}
	got, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatalf("read retained cache: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("schema-incompatible download replaced the last-known-good cache")
	}
	if _, err := ParseAPI(apiPath); err != nil {
		t.Fatalf("retained cache is not typed-parseable: %v", err)
	}
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
	acquisition, err := client.AcquireAPI(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if acquisition.Kind != HTTPAcquisitionDownloaded {
		t.Fatalf("acquisition = %q, want %q", acquisition.Kind, HTTPAcquisitionDownloaded)
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

	var parsed map[string]any
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
	cacheData, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}
	if err := client.writeCacheMetadata(apiPath, httpCacheMetadata{
		Version: httpCacheMetadataVersion, Origin: HTTPAcquisitionDownloaded,
		ContentChecksum: checksumBytes(cacheData), ValidatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Failed to write cache metadata: %v", err)
	}

	// Test that cached file is used
	ctx := context.Background()
	acquisition, err := client.AcquireAPI(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if acquisition.Kind != HTTPAcquisitionFreshCache {
		t.Fatalf("acquisition = %q, want %q", acquisition.Kind, HTTPAcquisitionFreshCache)
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

func TestHTTPClientConditionalRevalidationRetainsHTTPRevision(t *testing.T) {
	tests := []struct {
		name              string
		responseHeader    string
		responseValue     string
		conditionalHeader string
		wantRevisionKind  sources.RevisionKind
	}{
		{
			name: "etag", responseHeader: "ETag", responseValue: `"models-v1"`,
			conditionalHeader: "If-None-Match", wantRevisionKind: sources.RevisionKindETag,
		},
		{
			name: "last modified", responseHeader: "Last-Modified", responseValue: "Wed, 08 Jul 2026 18:00:00 GMT",
			conditionalHeader: "If-Modified-Since", wantRevisionKind: sources.RevisionKindLastModified,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := []byte(largeMockAPIJSON())
			requestCount := 0
			servedBytes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				requestCount++
				w.Header().Set(test.responseHeader, test.responseValue)
				if requestCount == 1 {
					servedBytes += len(body)
					_, _ = w.Write(body)
					return
				}
				if got := request.Header.Get(test.conditionalHeader); got != test.responseValue {
					t.Errorf("%s = %q, want %q", test.conditionalHeader, got, test.responseValue)
					w.WriteHeader(http.StatusPreconditionFailed)
					return
				}
				w.WriteHeader(http.StatusNotModified)
			}))
			defer server.Close()

			now := time.Date(2026, time.July, 10, 18, 0, 0, 0, time.UTC)
			client := &HTTPClient{
				CacheDir: filepath.Join(t.TempDir(), "models.dev"), APIURL: server.URL,
				Client: server.Client(), nowFunc: func() time.Time { return now },
			}
			first, err := client.AcquireAPI(context.Background())
			if err != nil {
				t.Fatalf("first AcquireAPI: %v", err)
			}
			if first.Kind != HTTPAcquisitionDownloaded || first.Revision.Kind != test.wantRevisionKind || first.Revision.Value != test.responseValue {
				t.Fatalf("first acquisition = %#v", first)
			}
			firstPayload, err := os.ReadFile(client.GetAPIPath())
			if err != nil {
				t.Fatalf("ReadFile first payload: %v", err)
			}

			now = now.Add(HTTPCacheTTL + time.Minute)
			second, err := client.AcquireAPI(context.Background())
			if err != nil {
				t.Fatalf("second AcquireAPI: %v", err)
			}
			if second.Kind != HTTPAcquisitionRevalidatedCache || second.Revision != first.Revision {
				t.Fatalf("second acquisition = %#v, want 304 revision %#v", second, first.Revision)
			}
			secondPayload, err := os.ReadFile(client.GetAPIPath())
			if err != nil {
				t.Fatalf("ReadFile second payload: %v", err)
			}
			if !bytes.Equal(secondPayload, firstPayload) {
				t.Fatal("304 revalidation changed the cached payload")
			}
			if requestCount != 2 || servedBytes != len(body) {
				t.Fatalf("requests/served bytes = %d/%d, want 2/%d", requestCount, servedBytes, len(body))
			}
		})
	}
}

func TestHTTPClientEmbeddedBootstrapCacheNeverBecomesFreshSuccess(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := &HTTPClient{
		CacheDir: filepath.Join(t.TempDir(), "models.dev"), APIURL: server.URL, Client: server.Client(),
	}

	first, err := client.AcquireAPI(context.Background())
	if err != nil {
		t.Fatalf("first AcquireAPI: %v", err)
	}
	second, err := client.AcquireAPI(context.Background())
	if err != nil {
		t.Fatalf("second AcquireAPI: %v", err)
	}
	if first.Kind != HTTPAcquisitionEmbeddedBootstrap || second.Kind != HTTPAcquisitionEmbeddedBootstrap {
		t.Fatalf("bootstrap acquisitions = (%q, %q), want embedded bootstrap twice", first.Kind, second.Kind)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want bootstrap cache reuse without relabeling", requestCount)
	}
}

func TestHTTPClientDoesNotSendValidatorFromMismatchedCacheMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("If-None-Match"); got != "" {
			t.Errorf("If-None-Match = %q, want empty for mismatched cache metadata", got)
		}
		w.Header().Set("ETag", `"models-v2"`)
		_, _ = w.Write([]byte(largeMockAPIJSON()))
	}))
	defer server.Close()
	client := &HTTPClient{
		CacheDir: filepath.Join(t.TempDir(), "models.dev"), APIURL: server.URL, Client: server.Client(),
	}
	if err := os.MkdirAll(client.CacheDir, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	apiPath := client.GetAPIPath()
	cache := []byte(largeMockAPIJSON())
	if err := os.WriteFile(apiPath, cache, constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := client.writeCacheMetadata(apiPath, httpCacheMetadata{
		Version: httpCacheMetadataVersion, Origin: HTTPAcquisitionDownloaded, ETag: `"models-v1"`,
		ContentChecksum: checksumBytes([]byte("different payload")), ValidatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeCacheMetadata: %v", err)
	}

	result, err := client.AcquireAPI(context.Background())
	if err != nil {
		t.Fatalf("AcquireAPI: %v", err)
	}
	if result.Kind != HTTPAcquisitionDownloaded || result.Revision != (sources.Revision{Kind: sources.RevisionKindETag, Value: `"models-v2"`}) {
		t.Fatalf("acquisition = %#v", result)
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
	acquisition, err := client.AcquireAPI(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if acquisition.Kind != HTTPAcquisitionStaleCache {
		t.Fatalf("acquisition = %q, want %q", acquisition.Kind, HTTPAcquisitionStaleCache)
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
	acquisition, err := client.AcquireAPI(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if acquisition.Kind != HTTPAcquisitionEmbeddedBootstrap {
		t.Fatalf("acquisition = %q, want %q", acquisition.Kind, HTTPAcquisitionEmbeddedBootstrap)
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

	var parsed map[string]any
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
