package server

import (
	"bufio"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/sse"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestPostCommitNotificationCorrespondsAcrossHTTPSSEAndCacheDespiteHookFaults(t *testing.T) {
	store := catalogstore.NewMemory()
	update := serverCatalogUpdate(func(candidate *catalogs.Builder) error {
		return candidate.SetProvider(catalogs.Provider{
			ID: "correspondence", Name: "Correspondence",
		})
	})
	client, err := starmap.New(starmap.WithCatalogStore(store))
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
	stream := openPublicationStream(t, server)

	if _, err := client.Update(context.Background(), update); err != nil {
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
	event := stream.wait(t)

	response, err := http.Get(stream.server.URL + "/api/v1/models?limit=1") //nolint:noctx
	if err != nil {
		t.Fatalf("GET models: %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("GET models status = %d: %s", response.StatusCode, body)
	}

	wantGeneration := published.Manifest.GenerationID
	state := client.CurrentCatalogState()
	if event.GenerationID != wantGeneration || event.Sequence != state.Sequence {
		t.Fatalf("SSE publication = %#v, state = %#v", event, state)
	}
	if event.ID != strconv.FormatUint(event.Sequence, 10) {
		t.Fatalf("SSE event ID = %q, sequence = %d", event.ID, event.Sequence)
	}
	if got := response.Header.Get("X-Starmap-Generation-ID"); got != wantGeneration {
		t.Fatalf("HTTP generation = %q, want %q", got, wantGeneration)
	}
	cacheState := server.Cache().GetStats()
	if cacheState.GenerationID != wantGeneration || cacheState.Sequence != state.Sequence {
		t.Fatalf("cache state = %#v, client = %#v", cacheState, state)
	}
	deadline := time.Now().Add(time.Second)
	for client.HookStats().Panics < 1 || client.HookStats().Failures < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("hook fault stats = %#v", client.HookStats())
		}
		time.Sleep(time.Millisecond)
	}
	releaseOnce.Do(func() { close(releaseSlow) })
}

func TestWebSocketRouteIsAbsent(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger := zerolog.Nop()
	server, err := New(&mockApplication{logger: &logger, sm: client}, Config{
		PathPrefix: "/api/v1", CacheTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/updates/ws", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("WebSocket route status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

type publicationStream struct {
	server       *httptest.Server
	publications chan streamedPublication
	errors       chan error
}

type streamedPublication struct {
	sse.Publication
	ID string
}

func openPublicationStream(t testing.TB, server *Server) *publicationStream {
	t.Helper()
	httpServer := httptest.NewServer(server.Handler())
	ctx, cancel := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		httpServer.URL+"/api/v1/updates/stream",
		nil,
	)
	if err != nil {
		cancel()
		httpServer.Close()
		t.Fatalf("New SSE request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		cancel()
		httpServer.Close()
		t.Fatalf("Connect SSE: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = response.Body.Close()
		httpServer.Close()
	})
	stream := &publicationStream{
		server:       httpServer,
		publications: make(chan streamedPublication, 16),
		errors:       make(chan error, 1),
	}
	go readPublicationSSE(response.Body, stream.publications, stream.errors)
	deadline := time.Now().Add(time.Second)
	for server.SSEBroadcaster().ClientCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if server.SSEBroadcaster().ClientCount() != 1 {
		t.Fatal("SSE connection did not register")
	}
	return stream
}

func (s *publicationStream) wait(t testing.TB) streamedPublication {
	t.Helper()
	select {
	case publication := <-s.publications:
		return publication
	case err := <-s.errors:
		t.Fatalf("read SSE publication: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE publication")
	}
	return streamedPublication{}
}

func (s *publicationStream) assertNone(t testing.TB, duration time.Duration) {
	t.Helper()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case publication := <-s.publications:
		t.Fatalf("unexpected SSE publication: %#v", publication)
	case err := <-s.errors:
		t.Fatalf("SSE stream failed: %v", err)
	case <-timer.C:
	}
}

func readPublicationSSE(
	body io.Reader,
	publications chan<- streamedPublication,
	errorsOut chan<- error,
) {
	reader := bufio.NewReader(body)
	for {
		eventType := ""
		eventID := ""
		var data []byte
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				errorsOut <- err
				return
			}
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			if line == "" {
				break
			}
			if after, ok := strings.CutPrefix(line, "event: "); ok {
				eventType = after
			}
			if after, ok := strings.CutPrefix(line, "id: "); ok {
				eventID = after
			}
			if after, ok := strings.CutPrefix(line, "data: "); ok {
				data = append(data[:0], after...)
			}
		}
		if eventType != sse.CatalogPublishedEvent {
			continue
		}
		var publication sse.Publication
		if err := json.Unmarshal(data, &publication); err != nil {
			errorsOut <- err
			return
		}
		publications <- streamedPublication{Publication: publication, ID: eventID}
	}
}
