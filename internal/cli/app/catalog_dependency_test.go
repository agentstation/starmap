package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/server/operations"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/server"
)

func TestHTTPMissingGitDependencyNamesSource(t *testing.T) {
	t.Run("refresh", func(t *testing.T) { testHTTPMissingGitDependency(t, false) })
	t.Run("fresh", func(t *testing.T) { testHTTPMissingGitDependency(t, true) })
}

func testHTTPMissingGitDependency(t *testing.T, fresh bool) {
	application := newMissingDependencyApplication(t)
	connected, err := application.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	before := connected.State().GenerationID
	syncer, err := application.CatalogAcquisition(connected.Client())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(connected.Client(), server.Config{PathPrefix: "/api/v1"}, server.WithRuntime(connected), server.WithSyncer(syncer))
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	httpServer := httptest.NewServer(srv.Handler())
	defer httpServer.Close()
	client := httpServer.Client()
	client.Timeout = 30 * time.Second
	read := func(method, path string) (operations.Status, []byte) {
		t.Helper()
		request, err := http.NewRequestWithContext(t.Context(), method, httpServer.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusAccepted && response.StatusCode != http.StatusOK {
			t.Fatalf("HTTP %d: %s", response.StatusCode, raw)
		}
		var envelope struct {
			Data operations.Status `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data, raw
	}
	path := "/api/v1/update?source=models_dev_git"
	if fresh {
		path += "&fresh=true"
	}
	status, raw := read(http.MethodPost, path)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !status.State.Terminal() {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
			status, raw = read(http.MethodGet, "/api/v1/updates/"+status.ID)
		}
	}
	t.Logf("terminal operator response: %s", raw)
	if status.Reason != sources.ProviderReasonDependencyUnavailable {
		t.Errorf("missing dependency reason=%s", status.Reason)
	}
	if status.State != operations.StateFailed {
		t.Fatalf("missing dependency state=%s", status.State)
	}
	if connected.State().GenerationID != before {
		t.Fatal("failed dependency replaced the accepted baseline")
	}
	if !bytes.Contains(raw, []byte("models_dev_git")) {
		t.Fatal("operator result does not identify the source with missing dependencies")
	}
}

func newMissingDependencyApplication(t *testing.T) *App {
	t.Helper()
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	workspace := catalogs.NewEmpty()
	if err := workspace.SetAuthor(catalogs.Author{ID: "acme", Name: "Acme"}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SetAuthorModel("acme", catalogs.Model{ID: "known", Name: "Known", Authors: []catalogs.Author{{ID: "acme", Name: "Acme"}}}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SetProvider(catalogs.Provider{ID: "acme", Name: "Acme", Models: map[string]*catalogs.Model{"known": {ID: "known", ModelRef: "acme/known", Name: "Known"}}}); err != nil {
		t.Fatal(err)
	}
	workspacePath := filepath.Join(t.TempDir(), "workspace")
	if err := workspace.SaveTo(workspacePath); err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(workspace)
	if err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(baselinePath, payload, constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	application, err := New("test", "test", "test", "test", WithConfig(&Config{Quiet: true, CatalogPath: workspacePath, CatalogValues: map[string]string{
		catalogconfig.Source: "file", catalogconfig.SourceURL: baselinePath, catalogconfig.SourceStartupPolicy: "require_source", catalogconfig.AcquisitionEnabled: "false", catalogconfig.SourcePollInterval: "0s",
		catalogconfig.AcquisitionSources: "models_dev_git", catalogconfig.ModelsDevGitCommit: strings.Repeat("a", 40),
	}}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := application.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return application
}

func TestCLIMissingGitDependencyNamesSource(t *testing.T) {
	t.Run("refresh", func(t *testing.T) { testCLIMissingGitDependency(t, false) })
	t.Run("fresh", func(t *testing.T) { testCLIMissingGitDependency(t, true) })
}

func testCLIMissingGitDependency(t *testing.T, fresh bool) {
	application := newMissingDependencyApplication(t)
	connected, err := application.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	before := connected.State().GenerationID
	command := application.NewUpdateCommand()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	args := []string{"--source", "models.dev-git", "--yes", "--skip-dep-prompts", "--catalog-path", application.config.CatalogPath}
	if fresh {
		args = append(args, "--fresh")
	}
	command.SetArgs(args)
	err = command.ExecuteContext(t.Context())
	if err == nil || !strings.Contains(err.Error(), "models_dev_git") {
		t.Fatalf("missing dependency error=%v", err)
	}
	if sources.ClassifyProviderReason(err) != sources.ProviderReasonDependencyUnavailable {
		t.Fatalf("missing dependency reason=%s", sources.ClassifyProviderReason(err))
	}
	if connected.State().GenerationID != before {
		t.Fatal("missing dependencies replaced the accepted catalog")
	}
}
