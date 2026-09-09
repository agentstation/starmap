package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/cli/app"
	"github.com/agentstation/starmap/internal/constants"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublisherStagesChangingProviderAndMetadataIngestion(t *testing.T) {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "STARMAP_") {
			t.Setenv(name, os.Getenv(name))
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
	root := t.TempDir()
	storePath := filepath.Join(root, "catalog-store")
	t.Setenv("STARMAP_HOME", filepath.Join(root, "product"))
	t.Setenv("STARMAP_CATALOG_STORE_PATH", storePath)
	t.Setenv("ACME_API_KEY", "fixture-key")
	var revision atomic.Int32
	var providerCalls, metadataCalls atomic.Int32
	revision.Store(1)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls.Add(1)
		if r.URL.Path != "/models" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("provider request lost its endpoint or credential")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []map[string]any{{"id": "known", "object": "model", "name": "API model", "context_window": int64(revision.Load()) * 131072}}}); err != nil {
			t.Error(err)
		}
	}))
	defer api.Close()
	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadataCalls.Add(1)
		if r.URL.Path != "/api.json" {
			t.Error("unexpected metadata endpoint")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(publisherMetadataFixture(revision.Load())); err != nil {
			t.Error(err)
		}
	}))
	defer metadata.Close()
	metadataURL, err := url.Parse(metadata.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = publisherFixtureTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Hostname() == "models.dev" {
			clone := request.Clone(request.Context())
			clone.URL.Scheme, clone.URL.Host = metadataURL.Scheme, metadataURL.Host
			return originalTransport.RoundTrip(clone)
		}
		if request.URL.Hostname() != "127.0.0.1" && request.URL.Hostname() != "::1" {
			return nil, fmt.Errorf("unexpected external host %s", request.URL.Hostname())
		}
		return originalTransport.RoundTrip(request)
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	workspace := catalogs.NewEmpty()
	if err := workspace.SetAuthor(catalogs.Author{ID: "acme", Name: "Acme"}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SetAuthorModel("acme", catalogs.Model{ID: "known", Name: "Known", Authors: []catalogs.Author{{ID: "acme", Name: "Acme"}}}); err != nil {
		t.Fatal(err)
	}
	provider := catalogs.Provider{ID: "acme", Name: "Acme", Credentials: testcatalog.APIKeyCredentials("ACME_API_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer), Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: api.URL + "/models", ProtocolOptions: testcatalog.OpenAIProtocolOptions(), FieldMappings: []catalogs.FieldMapping{{From: "context_window", To: "limits.context_window"}}}}, Models: map[string]*catalogs.Model{"known": {ID: "known", ModelRef: "acme/known", Name: "Known"}}}
	if err := workspace.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	workspacePath := filepath.Join(root, "workspace")
	if err := workspace.SaveTo(workspacePath); err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(root, "baseline.json")
	payload, err := catalogs.EncodeCatalogPayload(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baselinePath, payload, constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{catalogconfig.Source: "file", catalogconfig.SourceURL: baselinePath, catalogconfig.SourceStartupPolicy: "require_source", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"} {
		t.Setenv(name, value)
	}
	cacheRoot := filepath.Join(root, "cache")
	t.Setenv("STARMAP_CACHE_DIR", cacheRoot)
	store, err := storage.NewFilesystem(storePath)
	if err != nil {
		t.Fatal(err)
	}
	var previous catalogs.Generation
	for _, version := range []int32{1, 2} {
		revision.Store(version)
		if err := os.RemoveAll(filepath.Join(cacheRoot, "models.dev")); err != nil {
			t.Fatal(err)
		}
		application := app.NewForCommand("test", "test", "test", "test")
		t.Cleanup(func() {
			if err := application.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := application.Execute(t.Context(), []string{"update", "acme", "--yes", "--quiet", "--catalog-path", workspacePath, "--catalog-store-path", storePath}); err != nil {
			t.Fatal(err)
		}
		if err := application.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		committed, err := store.Current(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := run([]string{"--generation-store", storePath, "--output-dir", filepath.Join(root, "releases")}, &output); err != nil {
			t.Fatal(err)
		}
		var report releaseReport
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		archive, err := os.ReadFile(filepath.Join(report.Directory, artifact.Filename))
		if err != nil {
			t.Fatal(err)
		}
		statement, err := os.ReadFile(filepath.Join(report.Directory, artifact.AttestationFilename))
		if err != nil {
			t.Fatal(err)
		}
		staged, err := artifact.Open(archive, statement)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(staged.Manifest, committed.Manifest) || !bytes.Equal(staged.Payload, committed.Payload) {
			t.Fatal("publisher changed committed evidence")
		}
		catalog, err := catalogs.DecodeCatalogPayload(staged.Payload)
		if err != nil {
			t.Fatal(err)
		}
		if len(catalog.Providers().List()) != 1 {
			t.Fatal("metadata added unapproved canonical providers")
		}
		actual, err := catalog.Provider("acme")
		if err != nil {
			t.Fatal(err)
		}
		model := actual.Models["known"]
		if model == nil || model.Limits == nil || model.Limits.ContextWindow != int64(version)*131072 || model.Description != publisherDescription(version) {
			t.Fatalf("published model did not combine changed sources: %+v", model)
		}
		found := map[sources.ID]bool{}
		for _, link := range staged.Manifest.SourceObservations {
			found[link.Source] = link.ObservationID != "" && link.EvidenceChecksum != ""
		}
		if !found[sources.ProvidersID] || !found[sources.ModelsDevHTTPID] || !found[sources.LocalCatalogID] {
			t.Fatal("publisher omitted an acquired source receipt")
		}
		if version == 2 && (staged.Manifest.GenerationID == previous.Manifest.GenerationID || staged.Manifest.Payload.Checksum == previous.Manifest.Payload.Checksum) {
			t.Fatal("changed inputs kept the prior generation")
		}
		previous = staged
	}
	if providerCalls.Load() != 2 || metadataCalls.Load() != 2 {
		t.Fatal("publisher did not acquire both changing inputs twice")
	}
}

func publisherDescription(revision int32) string {
	return fmt.Sprintf("Metadata revision %d. ", revision) + strings.Repeat("Fixture metadata. ", 100)
}

func publisherMetadataFixture(revision int32) map[string]any {
	payload := make(map[string]any)
	for index := range 101 {
		id := fmt.Sprintf("fixture-%03d", index)
		if index == 0 {
			id = "acme"
		}
		payload[id] = map[string]any{"id": id, "name": "Fixture provider", "models": map[string]any{"known": map[string]any{"id": "known", "name": "Known", "description": publisherDescription(revision), "temperature": true, "tool_call": true, "modalities": map[string]any{"input": []string{"text"}, "output": []string{"text"}}, "cost": map[string]any{"input": 1, "output": 2}, "limit": map[string]any{"context": 32768, "output": 4096}}}}
	}
	return payload
}

type publisherFixtureTransport func(*http.Request) (*http.Response, error)

func (transport publisherFixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}
