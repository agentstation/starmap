package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

func TestConnectedRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testConnectedSourceIngestion(t, false)
}

func TestScheduledRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testConnectedSourceIngestion(t, true)
}

func testConnectedSourceIngestion(t *testing.T, automatic bool) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	modelsPayload := ingestionMetadataPayload(t)
	var temperature atomic.Bool
	var metadataCalls atomic.Int32
	metadataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadataCalls.Add(1)
		if r.URL.Path != "/api.json" {
			t.Errorf("metadata path %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.Unmarshal(modelsPayload, &payload); err != nil {
			t.Error(err)
			return
		}
		provider, ok := payload["acme"].(map[string]any)
		if !ok {
			t.Error("fixture provider absent")
			return
		}
		models := provider["models"].(map[string]any)
		model, ok := models["known"].(map[string]any)
		if !ok {
			t.Error("fixture model absent")
			return
		}
		model["temperature"] = temperature.Load()
		model["description"] = "Metadata fixture"
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(metadataServer.Close)
	metadataURL, err := url.Parse(metadataServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = ingestionFixtureTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Hostname() == "models.dev" {
			copy := request.Clone(request.Context())
			copy.URL.Scheme, copy.URL.Host = metadataURL.Scheme, metadataURL.Host
			return originalTransport.RoundTrip(copy)
		}
		if request.URL.Hostname() != "127.0.0.1" && request.URL.Hostname() != "::1" {
			return nil, fmt.Errorf("unexpected external host %s", request.URL.Hostname())
		}
		return originalTransport.RoundTrip(request)
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	var limit atomic.Int64
	var partial atomic.Bool
	var failed atomic.Bool
	var calls atomic.Int32
	limit.Store(131072)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if failed.Load() {
			http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path != "/models" {
			t.Errorf("unexpected catalog path %s", r.URL.Path)
		}
		records := []map[string]any{{"id": "known", "object": "model", "name": "API name", "context_window": limit.Load()}}
		if partial.Load() {
			records = append(records, map[string]any{"object": "model"})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": records}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(api.Close)
	workspace := catalogs.NewEmpty()
	if err := workspace.SetAuthor(catalogs.Author{ID: "acme", Name: "Acme"}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SetAuthorModel("acme", catalogs.Model{ID: "known", Name: "Known", Authors: []catalogs.Author{{ID: "acme", Name: "Acme"}}}); err != nil {
		t.Fatal(err)
	}
	credentials := testcatalog.APIKeyCredentials("ACME_API_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer)
	provider := catalogs.Provider{ID: "acme", Name: "Acme", Credentials: credentials, Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: api.URL + "/models", ProtocolOptions: testcatalog.OpenAIProtocolOptions(), FieldMappings: []catalogs.FieldMapping{{From: "context_window", To: "limits.context_window"}}}}, Models: map[string]*catalogs.Model{"known": {ID: "known", ModelRef: "acme/known", Name: "Reviewed offering"}}}
	if err := workspace.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "workspace")
	if err := workspace.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	payload, err := catalogs.EncodeCatalogPayload(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baselinePath, payload, constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	open := func() *App {
		application, err := New("test", "test", "test", "test", WithConfig(&Config{Quiet: true, CatalogPath: path, CatalogValues: map[string]string{catalogconfig.Source: "file", catalogconfig.SourceURL: baselinePath, catalogconfig.SourceStartupPolicy: "require_source", catalogconfig.AcquisitionEnabled: strconv.FormatBool(automatic), catalogconfig.AcquisitionInterval: "0s", catalogconfig.StartupSpread: "0s", catalogconfig.SourcePollInterval: "0s"}}))
		if err != nil {
			t.Fatal(err)
		}
		application.credentialResolver = sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
			return testcatalog.APIKeyMaterial(p.Credentials, "fixture-key"), nil
		})
		t.Cleanup(func() {
			if err := application.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		return application
	}
	application := open()
	connected, err := application.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if automatic {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		timer := time.NewTicker(10 * time.Millisecond)
		defer timer.Stop()
		for connected.Status().AcquisitionHealth == runtime.HealthUnknown {
			select {
			case <-ctx.Done():
				t.Fatal("automatic source acquisition did not finish")
			case <-timer.C:
			}
		}
	} else {
		report, err := connected.Sync(t.Context(), "acme")
		if err != nil {
			t.Fatal(err)
		}
		if report.Succeeded != 1 {
			t.Fatalf("provider acquisition report=%+v", report)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", calls.Load())
	}
	if metadataCalls.Load() != 1 {
		t.Fatalf("connected acquisition made %d metadata requests, want 1", metadataCalls.Load())
	}
	observed, err := connected.State().Catalog.Provider("acme")
	if err != nil {
		t.Fatal(err)
	}
	model := observed.Models["known"]
	if model == nil || model.Description != "Metadata fixture" || model.Limits == nil || model.Limits.ContextWindow != 131072 {
		t.Fatalf("connected catalog did not combine provider and metadata facts: %+v", model)
	}
}
