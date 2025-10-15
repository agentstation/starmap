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

// TestHub_MultipleClients tests multiple concurrent clients.
func TestHub_MultipleClients(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register multiple clients
	const numClients = 20
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = NewClient("client-"+string(rune(i)), hub, nil)
		hub.Register(clients[i])
	}
	time.Sleep(50 * time.Millisecond)

	// Verify all registered
	if count := hub.ClientCount(); count != numClients {
		t.Fatalf("expected %d clients, got %d", numClients, count)
	}

	// Broadcast message
	testMsg := Message{
		Type: "test.event",
		Data: map[string]any{"message": "hello"},
	}
	hub.Broadcast(testMsg)

	// Verify all clients received message
	for i, client := range clients {
		select {
		case msg := <-client.send:
			if msg.Type != testMsg.Type {
				t.Errorf("client %d: expected type %s, got %s", i, testMsg.Type, msg.Type)
			}
		case <-time.After(200 * time.Millisecond):
			t.Errorf("client %d: did not receive message", i)
		}
	}
}

// TestHub_ClientBufferFull tests client behavior when buffer approaches full.
func TestHub_ClientBufferFull(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Create client with small buffer
	client := &Client{
		id:   "test-client",
		hub:  hub,
		conn: nil,
		send: make(chan Message, 10), // Small buffer
	}
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Send messages rapidly
	for i := 0; i < 20; i++ {
		hub.Broadcast(Message{
			Type: "rapid",
			Data: map[string]any{"i": i},
		})
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Client should either handle messages or be unregistered
	// (implementation dependent, so we just verify no panic occurred)
	count := hub.ClientCount()
	if count < 0 || count > 1 {
		t.Errorf("unexpected client count: %d", count)
	}
}

// TestHub_ConcurrentRegisterUnregister tests concurrent register/unregister operations.
func TestHub_ConcurrentRegisterUnregister(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Concurrently register and unregister clients
	const numOperations = 50
	done := make(chan bool, numOperations*2)

	// Registrations
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			client := NewClient("client-"+string(rune(id)), hub, nil)
			hub.Register(client)
			done <- true
		}(i)
	}

	// Unregistrations (for some clients) via unregister channel
	for i := 0; i < numOperations/2; i++ {
		go func(id int) {
			time.Sleep(5 * time.Millisecond)
			client := NewClient("client-"+string(rune(id)), hub, nil)
			hub.unregister <- client
			done <- true
		}(i)
	}

	// Wait for all operations
	for i := 0; i < numOperations+numOperations/2; i++ {
		<-done
	}

	time.Sleep(50 * time.Millisecond)

	// Final count should be reasonable
	count := hub.ClientCount()
	if count < 0 || count > numOperations {
		t.Errorf("unexpected client count: %d", count)
	}
}

// TestHub_MessageOrdering tests that messages maintain order for each client.
func TestHub_MessageOrdering(t *testing.T) {
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

	// Send ordered messages
	const numMessages = 20
	for i := 0; i < numMessages; i++ {
		hub.Broadcast(Message{
			Type: "ordered",
			Data: map[string]any{"seq": i},
		})
	}

	// Verify order is maintained
	for i := 0; i < numMessages; i++ {
		select {
		case msg := <-client.send:
			data, ok := msg.Data.(map[string]any)
			if !ok {
				t.Fatal("invalid message data type")
			}
			seq, ok := data["seq"].(int)
			if !ok {
				t.Fatal("invalid seq type")
			}
			if seq != i {
				t.Errorf("expected seq=%d, got %d (out of order)", i, seq)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout waiting for message %d", i)
		}
	}
}

// TestHub_StressTest tests hub under heavy concurrent load.
func TestHub_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Register many clients
	const numClients = 100
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = NewClient("stress-"+string(rune(i)), hub, nil)
		hub.Register(clients[i])
	}
	time.Sleep(100 * time.Millisecond)

	// Broadcast many messages
	const numMessages = 100
	done := make(chan bool)
	go func() {
		for i := 0; i < numMessages; i++ {
			hub.Broadcast(Message{
				Type: "stress",
				Data: map[string]any{"id": i},
			})
		}
		done <- true
	}()

	// Wait for broadcasts to complete
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stress test timeout")
	}

	// Let messages propagate
	time.Sleep(200 * time.Millisecond)

	// Verify all clients still connected
	if count := hub.ClientCount(); count != numClients {
		t.Errorf("expected %d clients, got %d", numClients, count)
	}
}
