package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/save"
)

func TestDurableServerUpdatePublishesSameGenerationAfterProcessRestart(t *testing.T) {
	catalogPath := t.TempDir()
	storePath := t.TempDir()
	local := catalogs.NewEmpty()
	if err := local.SetProvider(catalogs.Provider{ID: "before", Name: "Before"}); err != nil {
		t.Fatalf("Set initial provider: %v", err)
	}
	if err := local.Save(save.WithPath(catalogPath)); err != nil {
		t.Fatalf("Save initial catalog: %v", err)
	}
	store, err := catalogstore.NewFilesystem(storePath)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	client, err := starmap.New(starmap.WithCatalogStore(store), starmap.WithLocalPath(catalogPath))
	if err != nil {
		t.Fatalf("New client: %v", err)
	}

	if err := local.SetProvider(catalogs.Provider{ID: "after-restart", Name: "After Restart"}); err != nil {
		t.Fatalf("Set updated provider: %v", err)
	}
	if err := local.Save(save.WithPath(catalogPath)); err != nil {
		t.Fatalf("Save updated catalog: %v", err)
	}
	logger := zerolog.Nop()
	server, err := New(&mockApplication{logger: &logger, sm: client}, Config{
		PathPrefix: "/api/v1", CacheTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/update?source=local_catalog", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	published, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current after update: %v", err)
	}
	if client.CurrentGenerationID() != published.Manifest.GenerationID {
		t.Fatalf("client generation = %q, store = %q", client.CurrentGenerationID(), published.Manifest.GenerationID)
	}

	reopened, err := catalogstore.NewFilesystem(storePath)
	if err != nil {
		t.Fatalf("Reopen filesystem store: %v", err)
	}
	restarted, err := starmap.New(starmap.WithCatalogStore(reopened), starmap.WithLocalPath(catalogPath))
	if err != nil {
		t.Fatalf("Restart client: %v", err)
	}
	if restarted.CurrentGenerationID() != published.Manifest.GenerationID {
		t.Fatalf("restarted generation = %q, want %q", restarted.CurrentGenerationID(), published.Manifest.GenerationID)
	}
	if _, err := restarted.Catalog().Provider("after-restart"); err != nil {
		t.Fatalf("restarted catalog does not serve committed provider: %v", err)
	}
	restartedGeneration, err := restarted.CurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("CurrentGeneration after restart: %v", err)
	}
	if !bytes.Equal(restartedGeneration.Payload, published.Payload) ||
		!reflect.DeepEqual(restartedGeneration.Manifest, published.Manifest) {
		t.Fatal("restarted process did not publish the exact committed generation")
	}
}
