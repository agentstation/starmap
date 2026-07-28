package catalogremote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestF017CharacterizationRemoteClientIsOneShotManifestAndPayloadFetch pins the
// current non-reactive contract. The client performs exactly one manifest GET
// and one immutable payload GET; it opens no event stream and owns no reconnect
// lifecycle. P7 must add an explicitly started, caller-context-owned
// subscriber around this verified fetch primitive.
func TestF017CharacterizationRemoteClientIsOneShotManifestAndPayloadFetch(t *testing.T) {
	generation := remoteTestGeneration(t, 1, catalogs.ConsumerCompatibility{
		MinSchemaVersion: 1,
		MaxSchemaVersion: 1,
	})
	manifest, err := MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatalf("MarshalManifest: %v", err)
	}
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		paths = append(paths, request.URL.Path)
		mu.Unlock()
		switch request.URL.Path {
		case ManifestPath:
			writer.Header().Set("Content-Type", ManifestMediaType)
			_, _ = writer.Write(manifest)
		case SnapshotPath(generation.Manifest.GenerationID):
			writer.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
			_, _ = writer.Write(generation.Payload)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchCurrent(context.Background()); err != nil {
		t.Fatalf("FetchCurrent: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), paths...)
	mu.Unlock()
	want := []string{ManifestPath, SnapshotPath(generation.Manifest.GenerationID)}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("remote request flow = %#v, want %#v and no stream/reconnect request", got, want)
	}
}
