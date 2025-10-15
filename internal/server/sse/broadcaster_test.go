package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestBroadcaster_NewBroadcaster tests broadcaster creation.
func TestBroadcaster_NewBroadcaster(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	if b == nil {
		t.Fatal("NewBroadcaster returned nil")
	}

	if b.clients == nil {
		t.Error("clients map not initialized")
	}

	if b.newClients == nil {
		t.Error("newClients channel not initialized")
	}

	if b.closed == nil {
		t.Error("closed channel not initialized")
	}

	if b.events == nil {
		t.Error("events channel not initialized")
	}
}

// TestBroadcaster_BasicOperation tests basic broadcaster operations.
func TestBroadcaster_BasicOperation(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register client
	client := make(chan Event, 256)
	b.newClients <- client
	time.Sleep(10 * time.Millisecond)

	if count := b.ClientCount(); count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}

	// Broadcast event
	event := Event{
		Event: "test",
		Data:  map[string]any{"test": true},
	}
	b.Broadcast(event)

	// Verify client received event
	select {
	case received := <-client:
		if received.Event != event.Event {
			t.Errorf("expected event %s, got %s", event.Event, received.Event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client did not receive event")
	}
}

// TestBroadcaster_Shutdown tests graceful shutdown.
func TestBroadcaster_Shutdown(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	ctx, cancel := context.WithCancel(context.Background())

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register clients
	client1 := make(chan Event, 256)
	client2 := make(chan Event, 256)
	b.newClients <- client1
	b.newClients <- client2
	time.Sleep(10 * time.Millisecond)

	if count := b.ClientCount(); count != 2 {
		t.Fatalf("expected 2 clients, got %d", count)
	}

	// Trigger shutdown
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Verify all clients disconnected
	if count := b.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", count)
	}
}

// TestBroadcaster_MultipleClients tests multiple concurrent SSE clients.
func TestBroadcaster_MultipleClients(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register multiple clients
	const numClients = 10
	clients := make([]chan Event, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = make(chan Event, 256)
		b.newClients <- clients[i]
	}
	time.Sleep(50 * time.Millisecond)

	// Verify all registered
	if count := b.ClientCount(); count != numClients {
		t.Fatalf("expected %d clients, got %d", numClients, count)
	}

	// Broadcast event
	testEvent := Event{
		Event: "test",
		ID:    "123",
		Data:  map[string]any{"message": "hello"},
	}
	b.Broadcast(testEvent)

	// Verify all clients received event
	for i, client := range clients {
		select {
		case event := <-client:
			if event.Event != testEvent.Event {
				t.Errorf("client %d: expected event %s, got %s", i, testEvent.Event, event.Event)
			}
			if event.ID != testEvent.ID {
				t.Errorf("client %d: expected ID %s, got %s", i, testEvent.ID, event.ID)
			}
		case <-time.After(200 * time.Millisecond):
			t.Errorf("client %d: did not receive event", i)
		}
	}
}

// TestBroadcaster_BroadcastChannelFull tests behavior when broadcast channel is full.
func TestBroadcaster_BroadcastChannelFull(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	// Don't start Run() so events won't be consumed
	// This will cause the channel to fill up

	// Fill the channel (capacity is 256)
	for i := 0; i < 256; i++ {
		b.Broadcast(Event{
			Event: "fill",
			Data:  map[string]any{"i": i},
		})
	}

	// Next broadcast should not block (should drop the event)
	done := make(chan bool, 1)
	go func() {
		b.Broadcast(Event{
			Event: "overflow",
			Data:  map[string]any{"test": true},
		})
		done <- true
	}()

	select {
	case <-done:
		// Success - broadcast didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("Broadcast blocked when channel was full")
	}
}

// TestBroadcaster_ClientDisconnect tests client disconnect handling.
func TestBroadcaster_ClientDisconnect(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register clients
	client1 := make(chan Event, 256)
	client2 := make(chan Event, 256)
	b.newClients <- client1
	b.newClients <- client2
	time.Sleep(10 * time.Millisecond)

	if count := b.ClientCount(); count != 2 {
		t.Fatalf("expected 2 clients, got %d", count)
	}

	// Disconnect client1
	b.closed <- client1
	time.Sleep(10 * time.Millisecond)

	if count := b.ClientCount(); count != 1 {
		t.Errorf("expected 1 client after disconnect, got %d", count)
	}

	// Broadcast event - only client2 should receive
	testEvent := Event{Event: "test", Data: map[string]any{"value": 42}}
	b.Broadcast(testEvent)

	// client2 should receive
	select {
	case <-client2:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("client2 did not receive event")
	}

	// client1 should be closed
	select {
	case _, ok := <-client1:
		if ok {
			t.Error("client1 channel should be closed")
		}
	default:
		t.Error("client1 channel not closed")
	}
}

// TestBroadcaster_ClientBufferFull tests behavior when client buffer is full.
func TestBroadcaster_ClientBufferFull(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register client with small buffer
	client := make(chan Event, 5)
	b.newClients <- client
	time.Sleep(10 * time.Millisecond)

	// Fill client buffer
	for i := 0; i < 5; i++ {
		b.Broadcast(Event{Event: "fill", Data: map[string]any{"i": i}})
		time.Sleep(5 * time.Millisecond)
	}

	// Broadcast more events - should skip when buffer full
	for i := 0; i < 5; i++ {
		b.Broadcast(Event{Event: "overflow", Data: map[string]any{"i": i}})
		time.Sleep(5 * time.Millisecond)
	}

	// Verify client still connected
	if count := b.ClientCount(); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}

	// Drain client buffer
	received := 0
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case <-client:
			received++
		case <-timeout:
			goto verify
		}
	}
verify:
	// Should have received at least the initial 5 events
	if received < 5 {
		t.Errorf("expected at least 5 events, got %d", received)
	}
}

// TestBroadcaster_ServeHTTP tests the SSE HTTP handler.
func TestBroadcaster_ServeHTTP(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Create request with cancellable context
	req := httptest.NewRequest("GET", "/events", nil)
	reqCtx, reqCancel := context.WithCancel(req.Context())
	req = req.WithContext(reqCtx)

	// Create response recorder
	w := httptest.NewRecorder()

	// Start ServeHTTP in goroutine
	done := make(chan bool)
	go func() {
		b.ServeHTTP(w, req)
		done <- true
	}()

	// Wait for client to register
	for i := 0; i < 100; i++ {
		if b.ClientCount() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify client registered
	if count := b.ClientCount(); count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}

	// Broadcast test event
	testEvent := Event{
		Event: "test.event",
		ID:    "evt-123",
		Data:  map[string]any{"message": "hello"},
	}
	b.Broadcast(testEvent)
	time.Sleep(100 * time.Millisecond)

	// Cancel request to stop ServeHTTP
	reqCancel()

	// Wait for handler to finish
	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("ServeHTTP did not finish after context cancel")
	}

	// Now it's safe to check headers and body since ServeHTTP has finished
	// Verify headers
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type=text/event-stream, got %s", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control=no-cache, got %s", cc)
	}
	if conn := w.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected Connection=keep-alive, got %s", conn)
	}

	// Verify response body contains SSE formatted data
	body := w.Body.String()

	// Should contain initial connection event
	if !strings.Contains(body, "event: connected") {
		t.Error("missing initial connection event")
	}
	if !strings.Contains(body, "Connected to Starmap updates stream") {
		t.Error("missing connection message")
	}

	// Should contain test event
	if !strings.Contains(body, "event: test.event") {
		t.Error("missing test event type")
	}
	if !strings.Contains(body, "id: evt-123") {
		t.Error("missing test event ID")
	}
	if !strings.Contains(body, `"message":"hello"`) {
		t.Error("missing test event data")
	}
}

// TestBroadcaster_ServeHTTP_NoFlusher tests ServeHTTP with non-flushing writer.
func TestBroadcaster_ServeHTTP_NoFlusher(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Create request
	req := httptest.NewRequest("GET", "/events", nil)

	// Create custom ResponseWriter that doesn't implement Flusher
	w := &nonFlushingWriter{
		header: make(http.Header),
		buffer: &strings.Builder{},
	}

	// ServeHTTP should detect lack of flusher and return error
	b.ServeHTTP(w, req)

	// Verify error response
	if w.statusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.statusCode)
	}
	if !strings.Contains(w.buffer.String(), "Streaming not supported") {
		t.Error("missing streaming not supported error")
	}
}

// TestBroadcaster_WriteEvent tests SSE event formatting.
func TestBroadcaster_WriteEvent(t *testing.T) {
	tests := []struct {
		name           string
		event          Event
		expectedOutput []string
	}{
		{
			name: "full event with type, ID, and data",
			event: Event{
				Event: "update",
				ID:    "123",
				Data:  map[string]any{"status": "ok"},
			},
			expectedOutput: []string{
				"event: update",
				"id: 123",
				`data: {"status":"ok"}`,
			},
		},
		{
			name: "event without type",
			event: Event{
				ID:   "456",
				Data: map[string]any{"value": 42},
			},
			expectedOutput: []string{
				"id: 456",
				`data: {"value":42}`,
			},
		},
		{
			name: "event without ID",
			event: Event{
				Event: "ping",
				Data:  map[string]any{"timestamp": 12345},
			},
			expectedOutput: []string{
				"event: ping",
				`data: {"timestamp":12345}`,
			},
		},
		{
			name: "event with only data",
			event: Event{
				Data: map[string]any{"test": true},
			},
			expectedOutput: []string{
				`data: {"test":true}`,
			},
		},
		{
			name: "event with string data",
			event: Event{
				Event: "message",
				Data:  "hello world",
			},
			expectedOutput: []string{
				"event: message",
				`data: "hello world"`,
			},
		},
		{
			name: "event with null data",
			event: Event{
				Event: "empty",
				Data:  nil,
			},
			expectedOutput: []string{
				"event: empty",
				"data: null",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			b := NewBroadcaster(&logger)

			w := httptest.NewRecorder()
			flusher := w

			b.writeEvent(w, flusher, tt.event)

			output := w.Body.String()

			// Verify all expected strings are present
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("output missing expected string %q\nGot: %s", expected, output)
				}
			}

			// Verify SSE format (ends with double newline)
			if !strings.HasSuffix(output, "\n\n") {
				t.Error("SSE event should end with double newline")
			}
		})
	}
}

// TestBroadcaster_ConcurrentBroadcast tests concurrent broadcasting.
func TestBroadcaster_ConcurrentBroadcast(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register client
	client := make(chan Event, 256)
	b.newClients <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast multiple events concurrently
	const numEvents = 50
	done := make(chan bool)
	go func() {
		for i := 0; i < numEvents; i++ {
			b.Broadcast(Event{
				Event: "concurrent",
				Data:  map[string]any{"i": i},
			})
		}
		done <- true
	}()

	// Wait for broadcasts
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("concurrent broadcast timeout")
	}

	// Drain and count messages
	time.Sleep(100 * time.Millisecond)
	count := 0
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case <-client:
			count++
		case <-timeout:
			goto verify
		}
	}
verify:
	if count != numEvents {
		t.Errorf("expected %d events, got %d", numEvents, count)
	}
}

// nonFlushingWriter is a ResponseWriter that doesn't implement Flusher.
type nonFlushingWriter struct {
	header     http.Header
	buffer     *strings.Builder
	statusCode int
}

func (w *nonFlushingWriter) Header() http.Header {
	return w.header
}

func (w *nonFlushingWriter) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *nonFlushingWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// TestBroadcaster_RegisterBeforeRun tests that registering clients before Run() doesn't block.
// This test catches the deadlock bug where unbuffered channels would block on client registration
// before the event loop starts running.
func TestBroadcaster_RegisterBeforeRun(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	// Try to register a client BEFORE starting Run() - this should NOT block with buffered channels
	done := make(chan struct{})
	go func() {
		client := make(chan Event, 256)
		b.newClients <- client // This would deadlock if channels weren't buffered
		close(done)
	}()

	select {
	case <-done:
		// Success - registering client did not block
	case <-time.After(2 * time.Second):
		t.Fatal("broadcaster client registration deadlocked when called before Run() - channels are not buffered!")
	}

	// Now start the broadcaster to clean up
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Verify client was registered
	if count := b.ClientCount(); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}
}

// TestBroadcaster_MultipleRegistersBeforeRun tests multiple client registrations before Run().
// This tests the buffer size is adequate for typical initialization patterns.
func TestBroadcaster_MultipleRegistersBeforeRun(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroadcaster(&logger)

	// Register multiple clients before Run() starts
	const numClients = 5
	done := make(chan struct{})

	go func() {
		for i := 0; i < numClients; i++ {
			client := make(chan Event, 256)
			b.newClients <- client
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - all client registrations completed
	case <-time.After(2 * time.Second):
		t.Fatal("broadcaster client registration deadlocked with multiple clients before Run()")
	}

	// Now start the broadcaster
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go b.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Verify all clients were registered
	if count := b.ClientCount(); count != numClients {
		t.Errorf("expected %d clients, got %d", numClients, count)
	}
}
