package sse

import (
	"context"
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
