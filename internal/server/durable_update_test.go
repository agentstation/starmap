package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/operations"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestDurableServerUpdatePublishesSameGenerationAfterProcessRestart(t *testing.T) {
	catalogPath := t.TempDir()
	storePath := t.TempDir()
	local := catalogs.NewEmpty()
	if err := local.SetProvider(catalogs.Provider{ID: "before", Name: "Before"}); err != nil {
		t.Fatalf("Set initial provider: %v", err)
	}
	if err := local.SaveTo(catalogPath); err != nil {
		t.Fatalf("Save initial catalog: %v", err)
	}
	store, err := storage.NewFilesystem(storePath)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	client, err := starmap.New(starmap.WithCatalogStore(store), starmap.WithCatalogPath(catalogPath))
	if err != nil {
		t.Fatalf("New client: %v", err)
	}

	if err := local.SetProvider(catalogs.Provider{ID: "after-restart", Name: "After Restart"}); err != nil {
		t.Fatalf("Set updated provider: %v", err)
	}
	if err := local.SaveTo(catalogPath); err != nil {
		t.Fatalf("Save updated catalog: %v", err)
	}
	logger := zerolog.Nop()
	server, err := New(&mockApplication{logger: &logger, sm: client}, Config{
		PathPrefix: "/api/v1", CacheTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	handler := server.Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost, "/api/v1/update?source=local_catalog", nil,
	))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	operation := decodeOperationStatus(t, recorder.Body.Bytes())
	<-server.operations.Done(operation.ID)
	final := readOperationStatus(t, handler, operation.ID)
	if final.State != operations.StateSucceeded {
		t.Fatalf("operation state = %q, want %q: reason %q",
			final.State, operations.StateSucceeded, final.Reason)
	}

	published, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current after update: %v", err)
	}
	if client.CurrentGenerationID() != published.Manifest.GenerationID {
		t.Fatalf("client generation = %q, store = %q", client.CurrentGenerationID(), published.Manifest.GenerationID)
	}

	reopened, err := storage.NewFilesystem(storePath)
	if err != nil {
		t.Fatalf("Reopen filesystem store: %v", err)
	}
	restarted, err := starmap.New(starmap.WithCatalogStore(reopened), starmap.WithCatalogPath(catalogPath))
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

// readOperationStatus reads one operation status through the registered route.
func readOperationStatus(
	t *testing.T,
	handler http.Handler,
	id string,
) operations.Status {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet, "/api/v1/updates/"+id, nil,
	))
	if recorder.Code != http.StatusOK {
		t.Fatalf("operation status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	return decodeOperationStatus(t, recorder.Body.Bytes())
}

// decodeOperationStatus reads one operation out of a response envelope.
func decodeOperationStatus(t *testing.T, body []byte) operations.Status {
	t.Helper()
	var envelope struct {
		Data operations.Status `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("Unmarshal operation: %v", err)
	}
	return envelope.Data
}
