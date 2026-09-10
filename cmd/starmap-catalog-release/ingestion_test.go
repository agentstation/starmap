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
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/cli/app"
	"github.com/agentstation/starmap/internal/constants"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/internal/test/gitfixture"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublisherStagesChangingProviderAndMetadataIngestion(t *testing.T) {
	publisherStagesChangingIngestion(t, []string{"acme"}, sources.ModelsDevHTTPID)
}

func TestGitPublisherStagesChangingProviderAndMetadataIngestion(t *testing.T) {
	publisherStagesChangingIngestion(t, []string{"acme"}, sources.ModelsDevGitID)
}

func TestPublisherStagesAllProviderAndMetadataIngestion(t *testing.T) {
	publisherAllProviderIngestion(t, sources.ModelsDevHTTPID)
}

func TestGitPublisherStagesAllProviderAndMetadataIngestion(t *testing.T) {
	publisherAllProviderIngestion(t, sources.ModelsDevGitID)
}

func publisherAllProviderIngestion(t *testing.T, metadataSource sources.ID) {
	const childMarker = "STARMAP_TEST_PUBLISHER_CHILD"
	if os.Getenv(childMarker) == "1" {
		publisherStagesChangingIngestion(t, nil, metadataSource)
		return
	}
	// Start a separate process before net/http caches the proxy environment.
	// The child receives fixture credentials and private home and working directories.
	var blocked atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blocked.Add(1)
		http.Error(w, "External fixture request refused.", http.StatusBadGateway)
	}))
	defer proxy.Close()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	arguments := []string{"-test.run=^" + t.Name() + "$", "-test.count=1", "-test.v"}
	if deadline, present := t.Deadline(); present {
		arguments = append(arguments, "-test.timeout="+time.Until(deadline).String())
	}
	command := exec.CommandContext(t.Context(), executable, arguments...)
	command.Dir = root
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "PATH", "SYSTEMROOT", "WINDIR", "TMP", "TEMP", "TMPDIR", "CATALOG_GIT_FIXTURE_REQUIRED":
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env,
		childMarker+"=1", "HOME="+root, "USERPROFILE="+root,
		"APPDATA="+root, "LOCALAPPDATA="+root, "XDG_CONFIG_HOME="+root,
		"HTTP_PROXY="+proxy.URL, "HTTPS_PROXY="+proxy.URL,
		"http_proxy="+proxy.URL, "https_proxy="+proxy.URL, "NO_PROXY=", "no_proxy=",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated publisher fixture: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "--- SKIP:") {
		if os.Getenv("CATALOG_GIT_FIXTURE_REQUIRED") == "1" {
			t.Fatalf("required publisher fixture skipped: %s", output)
		}
		t.Skip("publisher Git fixture requires Git and Bun 1.3.12")
	}
	t.Logf("The local proxy refused %d external catalog requests.", blocked.Load())
}

func publisherStagesChangingIngestion(t *testing.T, providers []string, metadataSource sources.ID) {
	t.Helper()
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
	var git *gitfixture.Fixture
	var metadataURL *url.URL
	if metadataSource == sources.ModelsDevGitID {
		var payloads [][]byte
		for _, version := range []int32{1, 2} {
			payload, err := json.Marshal(publisherMetadataFixture(version))
			if err != nil {
				t.Fatal(err)
			}
			payloads = append(payloads, payload)
		}
		git = gitfixture.New(t, payloads...)
	} else {
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
		var err error
		metadataURL, err = url.Parse(metadata.URL)
		if err != nil {
			t.Fatal(err)
		}
	}
	countMetadata := func() int32 {
		if git != nil {
			return int32(git.BuildCount(t))
		}
		return metadataCalls.Load()
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = publisherFixtureTransport(func(request *http.Request) (*http.Response, error) {
		if metadataURL != nil && request.URL.Hostname() == "models.dev" {
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
		if git != nil {
			t.Setenv(catalogconfig.ModelsDevGitCommit, git.Commits[version-1])
			t.Setenv(catalogconfig.AcquisitionSources, "providers,local_catalog,"+string(metadataSource))
		}
		if err := os.RemoveAll(filepath.Join(cacheRoot, "models.dev")); err != nil {
			t.Fatal(err)
		}
		application := app.NewForCommand("test", "test", "test", "test")
		t.Cleanup(func() {
			if err := application.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		arguments := append([]string{"update"}, providers...)
		arguments = append(arguments, "--yes", "--quiet", "--catalog-path", workspacePath, "--catalog-store-path", storePath)
		if err := application.Execute(t.Context(), arguments); err != nil {
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
		metadataEvidence := catalog.Provenance().FindModelField("acme", "known", "Description")
		if len(metadataEvidence) != 1 || metadataEvidence[0].Source != metadataSource {
			t.Fatal("published description lost its source provenance")
		}
		boundMetadata := false
		found := map[sources.ID]bool{}
		for _, link := range staged.Manifest.SourceObservations {
			found[link.Source] = link.ObservationID != "" && link.EvidenceChecksum != ""
			if link.Source == metadataSource && link.ObservationID == metadataEvidence[0].ObservationID && link.EvidenceChecksum == metadataEvidence[0].EvidenceChecksum {
				boundMetadata = true
				if git != nil && (link.Revision.Kind != sources.RevisionKindGitCommit || link.Revision.Value != git.Commits[version-1] || link.Revision.InputName != "bun.lock" || link.Revision.InputChecksum != git.LockfileChecksum) {
					t.Fatalf("published description lost pinned Git inputs: %+v", link.Revision)
				}
			}

			if link.Source != sources.ProvidersID && link.Source != metadataSource && link.Source != sources.LocalCatalogID {
				continue
			}
			wantStatus, wantCompleteness := sources.ObservationStatusSucceeded, sources.ObservationCompletenessComplete
			if len(providers) == 0 && link.Source == sources.ProvidersID {
				wantStatus, wantCompleteness = sources.ObservationStatusDegraded, sources.ObservationCompletenessPartial
			}
			if link.Status != wantStatus || link.Completeness != wantCompleteness {
				t.Fatalf("published source %s health = %s/%s, want %s/%s", link.Source, link.Status, link.Completeness, wantStatus, wantCompleteness)
			}
		}
		if !boundMetadata {
			t.Fatal("published description has no matching immutable receipt")
		}
		if !found[sources.ProvidersID] || !found[metadataSource] || !found[sources.LocalCatalogID] {
			t.Fatal("publisher omitted an acquired source receipt")
		}
		if version == 2 && (staged.Manifest.GenerationID == previous.Manifest.GenerationID || staged.Manifest.Payload.Checksum == previous.Manifest.Payload.Checksum) {
			t.Fatal("changed inputs kept the prior generation")
		}
		previous = staged
	}
	if providerCalls.Load() != 2 || countMetadata() != 2 {
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
