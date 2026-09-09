package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/server/operations"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/server"
)

func TestCLIRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testApplicationMetadataIngestion(t, "cli")
}

func TestHTTPRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testApplicationMetadataIngestion(t, "http")
}

func testApplicationMetadataIngestion(t *testing.T, mode string) {
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
	defer metadataServer.Close()
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
	defer api.Close()
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
		application, err := New("test", "test", "test", "test", WithConfig(&Config{Quiet: true, CatalogPath: path, CatalogValues: map[string]string{catalogconfig.Source: "file", catalogconfig.SourceURL: baselinePath, catalogconfig.SourceStartupPolicy: "require_source", catalogconfig.AcquisitionEnabled: "false", catalogconfig.SourcePollInterval: "0s"}}))
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
	startHTTP := func() (*server.Server, *httptest.Server) {
		t.Helper()
		connectedServer, err := application.Runtime(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		syncer, err := application.CatalogAcquisition(connectedServer.Client())
		if err != nil {
			t.Fatal(err)
		}
		srv, err := server.New(connectedServer.Client(), server.Config{PathPrefix: "/api/v1"}, server.WithRuntime(connectedServer), server.WithSyncer(syncer))
		if err != nil {
			t.Fatal(err)
		}
		if err := srv.Start(); err != nil {
			t.Fatal(err)
		}
		return srv, httptest.NewServer(srv.Handler())
	}
	srv, httpServer := startHTTP()
	closeServer := func() {
		httpServer.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(closeServer)
	client := httpServer.Client()
	client.Timeout = 2 * time.Minute
	readStatus := func(method, target string, expected int) operations.Status {
		t.Helper()
		request, err := http.NewRequestWithContext(t.Context(), method, httpServer.URL+target, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != expected {
			t.Fatalf("%s %s: status %d, want %d", method, target, response.StatusCode, expected)
		}
		var envelope struct {
			Data operations.Status `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}
	runOperation := func(source string, fresh bool) operations.Status {
		t.Helper()
		if mode == "cli" {
			command := application.NewUpdateCommand()
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			args := []string{"acme", "--source", source, "--yes", "--catalog-path", path}
			if fresh {
				args = append(args, "--fresh")
			}
			command.SetArgs(args)
			if err := command.ExecuteContext(t.Context()); err != nil {
				t.Logf("CLI acquisition returned %v", err)
				return operations.Status{State: operations.StateFailed}
			}
			return operations.Status{State: operations.StateSucceeded}
		}

		values := url.Values{"provider": {"acme"}, "source": {source}}
		if fresh {
			values.Set("fresh", "true")
		}
		state := readStatus(http.MethodPost, "/api/v1/update?"+values.Encode(), http.StatusAccepted)
		if state.ID == "" {
			t.Fatal("operation ID is missing")
		}
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for !state.State.Terminal() {
			select {
			case <-ctx.Done():
				t.Fatalf("operation %s timed out: %v", state.ID, ctx.Err())
			case <-ticker.C:
				state = readStatus(http.MethodGet, "/api/v1/updates/"+state.ID, http.StatusOK)
			}
		}
		return state
	}
	run := func(source string) {
		t.Helper()
		if state := runOperation(source, false); state.State != operations.StateSucceeded {
			t.Fatalf("operation failed: %+v", state)
		}
	}
	assertModel := func(want int64, description string) {
		t.Helper()
		connected, err := application.Runtime(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		provider, err := connected.State().Catalog.Provider("acme")
		if err != nil {
			t.Fatal(err)
		}
		model := provider.Models["known"]
		if model == nil || model.Limits == nil || model.Limits.ContextWindow != want || model.Description != description {
			t.Fatalf("catalog model=%+v; want context %d and description %q", model, want, description)
		}
		for range 2 {
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, httpServer.URL+"/api/v1/models?provider=acme&id=known&limit=1", nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Data struct {
					Models []struct {
						ID          string `json:"id"`
						ProviderID  string `json:"provider_id"`
						Description string `json:"description"`
						Limits      struct {
							ContextWindow int64 `json:"context_window"`
						} `json:"limits"`
					} `json:"models"`
				} `json:"data"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&envelope)
			closeErr := response.Body.Close()
			if decodeErr != nil || closeErr != nil {
				t.Fatalf("model response: decode=%v close=%v", decodeErr, closeErr)
			}
			if response.StatusCode != http.StatusOK || response.Header.Get("X-Starmap-Generation-ID") != connected.State().GenerationID {
				t.Fatalf("model response status=%d generation=%s", response.StatusCode, response.Header.Get("X-Starmap-Generation-ID"))
			}
			if len(envelope.Data.Models) != 1 {
				t.Fatalf("model response=%+v", envelope.Data.Models)
			}
			item := envelope.Data.Models[0]
			if item.ID != "known" || item.ProviderID != "acme" || item.Description != description || item.Limits.ContextWindow != want {
				t.Fatalf("HTTP catalog item=%+v", item)
			}
		}

	}
	run(string(sources.LocalCatalogID))
	run(string(sources.ProvidersID))
	assertModel(131072, "")
	authored, err := catalogs.New(catalogs.WithPath(path))
	if err != nil {
		t.Fatal(err)
	}
	edited, err := authored.Provider("acme")
	if err != nil {
		t.Fatal(err)
	}
	edited.Models["known"].Description = "Operator fixture"
	if err := authored.SetProvider(edited); err != nil {
		t.Fatal(err)
	}
	if err := authored.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	run(string(sources.LocalCatalogID))
	if calls.Load() != before {
		t.Fatal("local refresh contacted the provider")
	}
	assertModel(131072, "Operator fixture")
	metadataObservations := make(map[int32]string)
	assertMetadata := func(want bool) {
		t.Helper()
		connected, err := application.Runtime(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		provider, err := connected.State().Catalog.Provider("acme")
		if err != nil {
			t.Fatal(err)
		}
		model := provider.Models["known"]
		if model.Features == nil || model.Features.Temperature != want {
			t.Fatalf("temperature=%+v, want %v", model.Features, want)
		}
		receipts := connected.State().Catalog.Provenance().FindModelField("acme", "known", "Features.temperature")
		if len(receipts) != 1 || receipts[0].Source != sources.ModelsDevHTTPID {
			t.Fatalf("metadata receipts=%+v", receipts)
		}
		storePath, err := application.catalogStatePath()
		if err != nil {
			t.Fatal(err)
		}
		store, err := storage.NewFilesystem(storePath)
		if err != nil {
			t.Fatal(err)
		}
		current, err := store.Current(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		bound := false
		for _, link := range current.Manifest.SourceObservations {
			if link.Source == sources.ModelsDevHTTPID && link.ObservationID == receipts[0].ObservationID && link.EvidenceChecksum == receipts[0].EvidenceChecksum && link.EvidenceChecksum != "" {
				bound = true
			}
		}
		if !bound {
			t.Fatal("metadata receipt has no matching immutable manifest link")
		}
		count := metadataCalls.Load()
		if count == 2 && receipts[0].ObservationID == metadataObservations[1] {
			t.Fatal("changed metadata retained the first response identity")
		}
		if prior := metadataObservations[count]; prior != "" && prior != receipts[0].ObservationID {
			t.Fatal("provider refresh or restart changed the retained metadata identity")
		}
		metadataObservations[count] = receipts[0].ObservationID

	}
	beforeMetadata := calls.Load()
	run(string(sources.ModelsDevHTTPID))
	assertMetadata(false)
	assertModel(131072, "Metadata fixture")
	if metadataCalls.Load() != 1 || calls.Load() != beforeMetadata {
		t.Fatal("metadata acquisition did not isolate the intended HTTP source")
	}
	temperature.Store(true)
	directories, err := application.SourceDirectories()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(directories.Cache, "models.dev")); err != nil {
		t.Fatal(err)
	}
	run(string(sources.ModelsDevHTTPID))
	assertMetadata(true)
	assertModel(131072, "Metadata fixture")
	if metadataCalls.Load() != 2 || calls.Load() != beforeMetadata {
		t.Fatal("changed metadata did not come from a second HTTP response")
	}
	limit.Store(262144)
	partial.Store(true)
	run(string(sources.ProvidersID))
	assertModel(262144, "Metadata fixture")
	assertMetadata(true)
	storePath, err := application.catalogStatePath()
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewFilesystem(storePath)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !accepted.Manifest.Degraded {
		t.Fatal("partial provider reply was reported as healthy")
	}
	connected, err := application.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	receipts := connected.State().Catalog.Provenance().FindModelField("acme", "known", "limits.context_window")
	if len(receipts) != 1 {
		t.Fatalf("limit receipts=%+v", receipts)
	}
	bound := false
	for _, link := range accepted.Manifest.SourceObservations {
		if string(link.Source) == string(sources.ProvidersID) && link.ObservationID == receipts[0].ObservationID && link.Status == sources.ObservationStatusDegraded && link.Completeness == sources.ObservationCompletenessPartial {
			bound = true
		}
	}
	if !bound {
		t.Fatal("partial value has no matching immutable source receipt")
	}
	failed.Store(true)
	if state := runOperation(string(sources.ProvidersID), true); state.State != operations.StateFailed {
		t.Fatalf("fresh provider failure state=%+v", state)
	}
	retained, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if retained.Manifest.GenerationID != accepted.Manifest.GenerationID {
		t.Fatal("failed acquisition replaced the accepted generation")
	}
	closeServer()
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	application = open()
	srv, httpServer = startHTTP()
	client = httpServer.Client()
	client.Timeout = 2 * time.Minute
	assertModel(262144, "Metadata fixture")
	assertMetadata(true)
	restarted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Manifest.GenerationID != accepted.Manifest.GenerationID || restarted.Manifest.Payload.Checksum != accepted.Manifest.Payload.Checksum || !restarted.Manifest.Degraded {
		t.Fatal("restart changed accepted data or partial-evidence health")
	}
}

type ingestionFixtureTransport func(*http.Request) (*http.Response, error)

func (transport ingestionFixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func ingestionMetadataPayload(t *testing.T) []byte {
	t.Helper()
	payload := make(map[string]any)
	for _, id := range []string{"acme", "fixture-b", "fixture-c", "fixture-d", "fixture-e"} {
		models := make(map[string]any)
		for index := range 20 {
			modelID := fmt.Sprintf("model-%02d", index)
			if index == 0 {
				modelID = "known"
			}
			models[modelID] = map[string]any{
				"id": modelID, "name": "Fixture model", "description": strings.Repeat("Fixture metadata. ", 100),
				"temperature": true, "tool_call": true,
				"modalities": map[string]any{"input": []string{"text"}, "output": []string{"text"}},
				"cost":       map[string]any{"input": 1, "output": 2},
				"limit":      map[string]any{"context": 32768, "output": 4096},
			}
		}
		payload[id] = map[string]any{"id": id, "name": "Fixture provider", "models": models}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
