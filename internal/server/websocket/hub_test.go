package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestHub_NewHub tests hub creation.
func TestHub_NewHub(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map not initialized")
	}

	if hub.broadcast == nil {
		t.Error("broadcast channel not initialized")
	}

	if hub.register == nil {
		t.Error("register channel not initialized")
	}

	if hub.unregister == nil {
		t.Error("unregister channel not initialized")
	}
}

// TestHub_BasicOperation tests basic hub operations with proper cleanup.
func TestHub_BasicOperation(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	// Create context with timeout for this test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start hub
	go hub.Run(ctx)

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Create and register client
	client := NewClient("test-1", hub, nil)
	hub.Register(client)

	// Wait for registration
	time.Sleep(10 * time.Millisecond)

	// Verify client count
	if count := hub.ClientCount(); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}

	// Broadcast message
	msg := Message{
		Type:      "test.event",
		Timestamp: time.Now(),
		Data:      map[string]any{"test": true},
	}
	hub.Broadcast(msg)

	// Verify client received message
	select {
	case received := <-client.send:
		if received.Type != msg.Type {
			t.Errorf("expected type %s, got %s", msg.Type, received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client did not receive message")
	}

	// Test passes - context cleanup happens automatically
}

// TestHub_Shutdown tests graceful shutdown.
func TestHub_Shutdown(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithCancel(context.Background())

	// Start hub
	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register clients
	client1 := NewClient("test-1", hub, nil)
	client2 := NewClient("test-2", hub, nil)
	hub.Register(client1)
	hub.Register(client2)

	time.Sleep(10 * time.Millisecond)

	if count := hub.ClientCount(); count != 2 {
		t.Fatalf("expected 2 clients, got %d", count)
	}

	// Trigger shutdown
	cancel()

	// Wait for shutdown
	time.Sleep(50 * time.Millisecond)

	// Verify all clients disconnected
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", count)
	}
}

// TestHub_ConcurrentBroadcast tests concurrent broadcasting.
func TestHub_ConcurrentBroadcast(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register client
	client := NewClient("test", hub, nil)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Broadcast multiple messages concurrently
	done := make(chan bool)
	go func() {
		for i := 0; i < 10; i++ {
			hub.Broadcast(Message{
				Type: "test",
				Data: map[string]any{"i": i},
			})
		}
		done <- true
	}()

	// Wait for broadcasts
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("broadcast timeout")
	}

	// Drain messages
	count := 0
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case <-client.send:
			count++
		case <-timeout:
			goto verify
		}
	}
verify:
	if count != 10 {
		t.Errorf("expected 10 messages, got %d", count)
	}
}
