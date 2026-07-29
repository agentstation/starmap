package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestRemoteCatalogClientAndServerShareVersionedManifestPayloadContract(t *testing.T) {
	app := newMockApplication()
	server, err := New(app, Config{PathPrefix: "/api/v1", CacheTTL: time.Minute})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	httpServer := httptest.NewServer(server.setupRouter())
	defer httpServer.Close()

	client, err := catalogremote.NewClient(
		httpServer.URL+"/api/v1", httpServer.Client(), catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	generation, err := client.FetchCurrent(context.Background())
	if err != nil {
		t.Fatalf("FetchCurrent: %v", err)
	}
	want, err := app.sm.CurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("CurrentGeneration: %v", err)
	}
	if generation.Manifest.GenerationID != want.Manifest.GenerationID ||
		generation.Manifest.Payload != want.Manifest.Payload ||
		string(generation.Payload) != string(want.Payload) {
		t.Fatalf("remote generation does not match server current generation")
	}
	addressed, err := client.FetchGeneration(
		context.Background(),
		want.Manifest.GenerationID,
	)
	if err != nil {
		t.Fatalf("FetchGeneration: %v", err)
	}
	if addressed.Manifest.GenerationID != want.Manifest.GenerationID ||
		addressed.Manifest.Payload != want.Manifest.Payload ||
		string(addressed.Payload) != string(want.Payload) {
		t.Fatal("addressed remote generation does not match retained server generation")
	}
}
