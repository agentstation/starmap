package sse

import (
	"bufio"
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewBroadcasterDefaultsAndValidation(t *testing.T) {
	logger := zerolog.Nop()
	broadcaster, err := NewBroadcaster(Config{}, &logger)
	if err != nil {
		t.Fatalf("NewBroadcaster: %v", err)
	}
	if broadcaster.config.HeartbeatInterval != DefaultHeartbeatInterval {
		t.Fatalf(
			"heartbeat interval = %s, want %s",
			broadcaster.config.HeartbeatInterval,
			DefaultHeartbeatInterval,
		)
	}
	if broadcaster.config.WriteTimeout != DefaultWriteTimeout {
		t.Fatalf(
			"write timeout = %s, want %s",
			broadcaster.config.WriteTimeout,
			DefaultWriteTimeout,
		)
	}

	for _, test := range []struct {
		name   string
		config Config
		logger *zerolog.Logger
	}{
		{name: "nil logger", logger: nil},
		{
			name:   "negative heartbeat",
			config: Config{HeartbeatInterval: -time.Second},
			logger: &logger,
		},
		{
			name:   "negative write timeout",
			config: Config{WriteTimeout: -time.Second},
			logger: &logger,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewBroadcaster(test.config, test.logger)
			if got != nil || err == nil {
				t.Fatalf("NewBroadcaster() = (%#v, %v), want nil error result", got, err)
			}
		})
	}
}

func TestPublishRejectsInvalidIdentity(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{})
	for _, publication := range []Publication{
		{Sequence: 1},
		{GenerationID: "generation"},
	} {
		if err := broadcaster.Publish(publication); err == nil {
			t.Fatalf("Publish(%#v) succeeded", publication)
		}
	}
}

func TestPublicationBackpressureTerminatesConnection(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{})
	connection := newClient()
	if broadcaster.register(connection) != admissionGranted {
		t.Fatal("register rejected active broadcaster")
	}

	if err := broadcaster.Publish(Publication{
		GenerationID: "generation-1", Sequence: 1,
	}); err != nil {
		t.Fatalf("Publish generation 1: %v", err)
	}
	if err := broadcaster.Publish(Publication{
		GenerationID: "generation-2", Sequence: 2,
	}); err != nil {
		t.Fatalf("Publish generation 2: %v", err)
	}

	select {
	case <-connection.done:
	case <-time.After(time.Second):
		t.Fatal("backpressured connection remained healthy")
	}
	if count := broadcaster.ClientCount(); count != 0 {
		t.Fatalf("client count = %d, want 0", count)
	}
	stats := broadcaster.Stats()
	if stats.Published != 2 || stats.BackpressureTerminated != 1 ||
		stats.Disconnected != 1 {
		t.Fatalf("delivery stats = %#v", stats)
	}
	health := broadcaster.Health()
	if health.State != StreamStateIdle || health.Clients != 0 ||
		health.LastError == nil || health.LastError.Kind != "backpressure" {
		t.Fatalf("backpressure health = %#v", health)
	}
}

func TestStreamFlushesHeartbeatAndPublicationOnOneWriter(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{
		HeartbeatInterval: 10 * time.Millisecond,
		WriteTimeout:      100 * time.Millisecond,
	})
	server := httptest.NewServer(broadcaster)
	t.Cleanup(server.Close)

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		server.URL,
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("connect SSE: %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if contentType := response.Header.Get("Content-Type"); contentType != "text/event-stream" {
		t.Fatalf("Content-Type = %q", contentType)
	}

	reader := bufio.NewReader(response.Body)
	if frame := readFrame(t, reader); frame != ": connected\n\n" {
		t.Fatalf("initial frame = %q", frame)
	}
	heartbeat := readFrame(t, reader)
	if heartbeat != ": heartbeat\n\n" {
		t.Fatalf("heartbeat frame = %q", heartbeat)
	}
	if strings.Contains(heartbeat, "id:") || strings.Contains(heartbeat, "event:") {
		t.Fatalf("heartbeat advanced event state: %q", heartbeat)
	}

	if err := broadcaster.Publish(Publication{
		GenerationID: "generation-7", Sequence: 7,
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	var publication string
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		frame := readFrame(t, reader)
		if strings.Contains(frame, "event: "+CatalogPublishedEvent) {
			publication = frame
			break
		}
	}
	for _, part := range []string{
		"id: 7\n",
		"event: catalog.published\n",
		`data: {"generation_id":"generation-7","sequence":7}` + "\n",
	} {
		if !strings.Contains(publication, part) {
			t.Fatalf("publication frame %q does not contain %q", publication, part)
		}
	}
	deadline = time.Now().Add(time.Second)
	for {
		stats := broadcaster.Stats()
		health := broadcaster.Health()
		if stats.Sent == 1 && stats.Heartbeats > 0 && stats.Failed == 0 &&
			health.State == StreamStateStreaming && health.Clients == 1 &&
			!health.LastHeartbeatAt.IsZero() && !health.LastEventAt.IsZero() &&
			health.LastGenerationID == "generation-7" &&
			health.LastSequence == 7 && health.LastError == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("delivery stats = %#v; stream health = %#v", stats, health)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestWriteFailureUsesDeadlineAndCleansUpConnection(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{
		HeartbeatInterval: time.Hour,
		WriteTimeout:      50 * time.Millisecond,
	})
	writer := newFailingWriter(1)
	request := httptest.NewRequest(http.MethodGet, "/updates/stream", nil)
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		broadcaster.ServeHTTP(writer, request)
	}()

	select {
	case <-writer.firstWrite:
	case <-time.After(time.Second):
		t.Fatal("initial SSE frame was not written")
	}
	if err := broadcaster.Publish(Publication{
		GenerationID: "generation-2", Sequence: 2,
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("failed SSE writer was not cleaned up")
	}

	if count := broadcaster.ClientCount(); count != 0 {
		t.Fatalf("client count = %d, want 0", count)
	}
	if stats := broadcaster.Stats(); stats.Failed != 1 || stats.Sent != 0 {
		t.Fatalf("delivery stats = %#v", stats)
	}
	if health := broadcaster.Health(); health.LastError == nil ||
		health.LastError.Kind != "write_failed" {
		t.Fatalf("write failure health = %#v", health)
	}
	if deadlines := writer.deadlineCount(); deadlines < 2 {
		t.Fatalf("write deadlines = %d, want at least 2", deadlines)
	}
}

func TestServeHTTPRequiresStreamingDeadlines(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{})
	request := httptest.NewRequest(http.MethodGet, "/updates/stream", nil)
	recorder := httptest.NewRecorder()
	broadcaster.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestServeHTTPRejectsUnsupportedRequests(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{})
	for _, test := range []struct {
		name       string
		method     string
		writer     http.ResponseWriter
		wantStatus int
	}{
		{
			name:       "wrong method",
			method:     http.MethodPost,
			writer:     newBasicWriter(),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "no streaming flush",
			method:     http.MethodGet,
			writer:     newBasicWriter(),
			wantStatus: http.StatusInternalServerError,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "/updates/stream", nil)
			broadcaster.ServeHTTP(test.writer, request)
			if status := test.writer.(*basicWriter).statusOrOK(); status != test.wantStatus {
				t.Fatalf("status = %d, want %d", status, test.wantStatus)
			}
		})
	}
}

func TestServeHTTPRejectsNewConnectionAfterClose(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{})
	broadcaster.Close()
	writer := newFailingWriter(10)
	request := httptest.NewRequest(http.MethodGet, "/updates/stream", nil)
	broadcaster.ServeHTTP(writer, request)
	if writer.statusCode() != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", writer.statusCode(), http.StatusServiceUnavailable)
	}
}

func TestServeHTTPReturnsOnCallerCancellation(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{HeartbeatInterval: time.Hour})
	writer := newFailingWriter(10)
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/updates/stream", nil).WithContext(ctx)
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		broadcaster.ServeHTTP(writer, request)
	}()
	select {
	case <-writer.firstWrite:
	case <-time.After(time.Second):
		t.Fatal("initial SSE frame was not written")
	}
	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("SSE handler did not return after caller cancellation")
	}
	if count := broadcaster.ClientCount(); count != 0 {
		t.Fatalf("client count = %d, want 0", count)
	}
}

func TestHeartbeatWriteFailureCleansUpConnection(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{
		HeartbeatInterval: time.Millisecond,
		WriteTimeout:      50 * time.Millisecond,
	})
	writer := newFailingWriter(1)
	request := httptest.NewRequest(http.MethodGet, "/updates/stream", nil)
	broadcaster.ServeHTTP(writer, request)
	stats := broadcaster.Stats()
	if stats.Failed != 1 || stats.Heartbeats != 0 || stats.Disconnected != 1 {
		t.Fatalf("delivery stats = %#v", stats)
	}
}

func TestCloseTerminatesConnectionsAndRejectsNewOnes(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{})
	connection := newClient()
	if broadcaster.register(connection) != admissionGranted {
		t.Fatal("register rejected active broadcaster")
	}
	broadcaster.Close()
	broadcaster.Close()
	select {
	case <-connection.done:
	case <-time.After(time.Second):
		t.Fatal("Close did not terminate connection")
	}
	if broadcaster.register(newClient()) == admissionGranted {
		t.Fatal("closed broadcaster accepted a connection")
	}
	if err := broadcaster.Publish(Publication{
		GenerationID: "generation-after-close", Sequence: 1,
	}); err != nil {
		t.Fatalf("Publish after close: %v", err)
	}
	if health := broadcaster.Health(); health.State != StreamStateStopped ||
		health.Clients != 0 {
		t.Fatalf("closed broadcaster health = %#v", health)
	}
}

func newTestBroadcaster(t testing.TB, config Config) *Broadcaster {
	t.Helper()
	logger := zerolog.Nop()
	broadcaster, err := NewBroadcaster(config, &logger)
	if err != nil {
		t.Fatalf("NewBroadcaster: %v", err)
	}
	return broadcaster
}

func readFrame(t testing.TB, reader *bufio.Reader) string {
	t.Helper()
	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE frame: %v", err)
		}
		frame.WriteString(line)
		if line == "\n" {
			return frame.String()
		}
	}
}

type failingWriter struct {
	mu         sync.Mutex
	header     http.Header
	status     int
	writes     int
	failAfter  int
	deadlines  int
	deadlineAt []time.Time
	firstWrite chan struct{}
	firstOnce  sync.Once
}

func newFailingWriter(failAfter int) *failingWriter {
	return &failingWriter{
		header:     make(http.Header),
		failAfter:  failAfter,
		firstWrite: make(chan struct{}),
	}
}

func (w *failingWriter) Header() http.Header { return w.header }

func (w *failingWriter) WriteHeader(status int) {
	w.mu.Lock()
	w.status = status
	w.mu.Unlock()
}

func (w *failingWriter) Write(payload []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes++
	w.firstOnce.Do(func() { close(w.firstWrite) })
	if w.writes > w.failAfter {
		return 0, stderrors.New("injected write failure")
	}
	return len(payload), nil
}

func (*failingWriter) Flush() {}

func (w *failingWriter) SetWriteDeadline(deadline time.Time) error {
	w.mu.Lock()
	w.deadlines++
	w.deadlineAt = append(w.deadlineAt, deadline)
	w.mu.Unlock()
	return nil
}

// deadlineValues returns every write deadline the handler set, oldest first.
func (w *failingWriter) deadlineValues() []time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.deadlineAt)
}

func (w *failingWriter) deadlineCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.deadlines
}

func (w *failingWriter) statusCode() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

type basicWriter struct {
	header http.Header
	status int
}

func newBasicWriter() *basicWriter {
	return &basicWriter{header: make(http.Header)}
}

func (w *basicWriter) Header() http.Header { return w.header }

func (w *basicWriter) Write(payload []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return len(payload), nil
}

func (w *basicWriter) WriteHeader(status int) { w.status = status }

func (w *basicWriter) statusOrOK() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

var _ http.ResponseWriter = (*failingWriter)(nil)
var _ http.Flusher = (*failingWriter)(nil)
var _ interface{ SetWriteDeadline(time.Time) error } = (*failingWriter)(nil)
var _ http.ResponseWriter = (*basicWriter)(nil)

// TestSourceAdmissionReturnsRetryAfter proves the bounded admission of the
// source server. A subscriber that arrives at capacity receives 503 with a
// Retry-After delay. A refused fleet therefore returns on a spread schedule
// instead of one synchronized retry.
func TestSourceAdmissionReturnsRetryAfter(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{
		MaxClients:          1,
		AdmissionRetryAfter: 10 * time.Second,
	})
	t.Cleanup(broadcaster.Close)

	if admitted := broadcaster.register(newClient()); admitted != admissionGranted {
		t.Fatalf("first admission = %v, want granted", admitted)
	}

	writer := newFailingWriter(10)
	request := httptest.NewRequest(http.MethodGet, "/updates/stream", nil)
	broadcaster.ServeHTTP(writer, request)

	if status := writer.statusCode(); status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", status, http.StatusServiceUnavailable)
	}
	header := writer.Header().Get("Retry-After")
	if header == "" {
		t.Fatal("refusal carried no Retry-After header")
	}
	seconds, err := strconv.Atoi(header)
	if err != nil {
		t.Fatalf("Retry-After = %q, want whole seconds", header)
	}
	if seconds < 10 || seconds > 20 {
		t.Fatalf("Retry-After = %d seconds, want the 10s to 20s window", seconds)
	}
	if count := broadcaster.ClientCount(); count != 1 {
		t.Fatalf("client count = %d, want the one admitted subscriber", count)
	}
	if stats := broadcaster.Stats(); stats.Refused != 1 {
		t.Fatalf("refused = %d, want 1", stats.Refused)
	}
	if health := broadcaster.Health(); health.MaxClients != 1 {
		t.Fatalf("health max clients = %d, want 1", health.MaxClients)
	}
}

// TestAdmissionRetryAfterStaysJitteredAndWhole proves that every refusal names
// a whole second inside the jitter window. A refusal that names one fixed delay
// returns the whole fleet at one instant.
func TestAdmissionRetryAfterStaysJitteredAndWhole(t *testing.T) {
	broadcaster := newTestBroadcaster(t, Config{
		AdmissionRetryAfter: time.Second,
	})
	t.Cleanup(broadcaster.Close)
	for range 200 {
		seconds := broadcaster.retryAfterSeconds()
		if seconds < 1 || seconds > 2 {
			t.Fatalf("Retry-After = %d seconds, want 1 or 2", seconds)
		}
	}
}
