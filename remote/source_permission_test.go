package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
)

func TestSourcePermissionReadDoesNotStartCatalogSynchronization(t *testing.T) {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	envelope := catalogs.CatalogPermissionEnvelope{Version: catalogs.CatalogPermissionEnvelopeVersion,
		Head: catalogs.CatalogAuthorityHead{AuthorityID: "enterprise", PolicyID: "production", Sequence: 3,
			GenerationID: "future-catalog", PayloadChecksum: "sha256:" + strings.Repeat("a", 64),
			RequiredPermissionRevision: "sha256:" + strings.Repeat("b", 64), PermissionSchemaVersion: catalogs.CatalogPermissionSchemaVersion + 1},
		IssuedAt: at, ValidUntil: at.Add(time.Minute)}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/api/v1"+protocol.PermissionEnvelopePath {
			t.Errorf("permission read requested %s", r.URL.Path)
			http.Error(w, "unsupported catalog", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", protocol.PermissionEnvelopeMediaType)
		_ = json.NewEncoder(w).Encode(envelope)
	}))
	defer server.Close()
	source, err := NewSource(t.Context(), SourceConfig{Subscriber: Config{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			t.Error(err)
		}
	}()
	if requests.Load() != 0 {
		t.Fatal("constructor fetched permission")
	}
	received, err := source.ReadPermission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if received.Head != envelope.Head || received.Head.SupportsPermissions() {
		t.Fatal("permission read lost the unknown mandatory revision")
	}
	source.startMu.Lock()
	started := source.started
	source.startMu.Unlock()
	if started || requests.Load() != 1 {
		t.Fatalf("permission read started catalog work: started=%v, requests=%d", started, requests.Load())
	}
}
