package server

import (
	"bufio"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/events"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestPostCommitNotificationCorrespondsAcrossHTTPWebSocketSSEAndCacheDespiteHookFaults(t *testing.T) {
	store := catalogstore.NewMemory()
	client, err := starmap.New(
		starmap.WithCatalogStore(store),
		starmap.WithUpdateFunc(func(_ context.Context, candidate *catalogs.Builder) (*catalogs.Builder, error) {
			if err := candidate.SetProvider(catalogs.Provider{ID: "correspondence", Name: "Correspondence"}); err != nil {
				return nil, err
			}
			return candidate, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseSlow) }) })
	client.OnCatalogPublished(func(starmap.CatalogPublishedEvent) error {
		close(slowStarted)
		<-releaseSlow
		return nil
	})
	client.OnCatalogPublished(func(starmap.CatalogPublishedEvent) error {
		panic("injected publication hook panic")
	})
	client.OnCatalogPublished(func(starmap.CatalogPublishedEvent) error {
		return stderrors.New("injected publication hook failure")
	})

	logger := zerolog.Nop()
	server, err := New(&mockApplication{logger: &logger, sm: client}, Config{
		PathPrefix: "/api/v1", CacheTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	server.Start()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	sseRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, httpServer.URL+"/api/v1/updates/stream", nil)
	if err != nil {
		t.Fatalf("New SSE request: %v", err)
	}
	sseResponse, err := http.DefaultClient.Do(sseRequest)
	if err != nil {
		t.Fatalf("Connect SSE: %v", err)
	}
	t.Cleanup(func() { _ = sseResponse.Body.Close() })
	sseEvents := make(chan map[string]any, 1)
	sseErrors := make(chan error, 1)
	go readPublicationSSE(sseResponse.Body, sseEvents, sseErrors)

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/updates/ws"
	wsConnection, wsResponse, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		if wsResponse != nil {
			body, _ := io.ReadAll(wsResponse.Body)
			t.Fatalf("Connect WebSocket: %v, status=%d body=%s", err, wsResponse.StatusCode, body)
		}
		t.Fatalf("Connect WebSocket: %v", err)
	}
	t.Cleanup(func() { _ = wsConnection.Close() })
	if err := wsConnection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for (server.SSEBroadcaster().ClientCount() != 1 || server.WSHub().ClientCount() != 1) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if server.SSEBroadcaster().ClientCount() != 1 || server.WSHub().ClientCount() != 1 {
		t.Fatalf("transport clients not registered: sse=%d ws=%d", server.SSEBroadcaster().ClientCount(), server.WSHub().ClientCount())
	}

	if err := client.Update(context.Background()); err != nil {
		t.Fatalf("Update: %v", err)
	}
	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		t.Fatal("slow hook did not start")
	}
	published, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}

	var websocketEvent struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	for websocketEvent.Type != string(events.CatalogPublished) {
		if err := wsConnection.ReadJSON(&websocketEvent); err != nil {
			t.Fatalf("Read WebSocket publication: %v", err)
		}
	}
	var sseData map[string]any
	select {
	case sseData = <-sseEvents:
	case err := <-sseErrors:
		t.Fatalf("Read SSE publication: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("SSE publication was blocked by an unrelated slow hook")
	}

	response, err := http.Get(httpServer.URL + "/api/v1/models?limit=1") //nolint:noctx
	if err != nil {
		t.Fatalf("GET models: %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("GET models status = %d: %s", response.StatusCode, body)
	}
	wantGeneration := published.Manifest.GenerationID
	wantSyncRun := published.Manifest.SyncRunID
	assertPublicationIdentity(t, "websocket", websocketEvent.Data, wantGeneration, wantSyncRun)
	assertPublicationIdentity(t, "sse", sseData, wantGeneration, wantSyncRun)
	if got := response.Header.Get("X-Starmap-Generation-ID"); got != wantGeneration {
		t.Fatalf("HTTP generation = %q, want %q", got, wantGeneration)
	}
	cacheState := server.Cache().GetStats()
	if cacheState.GenerationID != wantGeneration || cacheState.Sequence != client.CurrentCatalogState().Sequence {
		t.Fatalf("cache state = %#v, client = %#v", cacheState, client.CurrentCatalogState())
	}
	deadline = time.Now().Add(time.Second)
	for client.HookStats().Panics < 1 || client.HookStats().Failures < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("hook fault stats = %#v", client.HookStats())
		}
		time.Sleep(time.Millisecond)
	}
	releaseOnce.Do(func() { close(releaseSlow) })
}

func readPublicationSSE(body io.Reader, eventsOut chan<- map[string]any, errorsOut chan<- error) {
	reader := bufio.NewReader(body)
	for {
		eventType := ""
		var data map[string]any
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				errorsOut <- err
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data); err != nil {
					errorsOut <- err
					return
				}
			}
		}
		if eventType == string(events.CatalogPublished) {
			eventsOut <- data
			return
		}
	}
}

func assertPublicationIdentity(t testing.TB, channel string, data map[string]any, generationID, syncRunID string) {
	t.Helper()
	if data["generation_id"] != generationID || data["sync_run_id"] != syncRunID {
		t.Fatalf("%s publication = %#v, want generation=%q sync_run=%q", channel, data, generationID, syncRunID)
	}
}
