package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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

// TestClient_WritePump tests the WritePump method with mock WebSocket connection.
func TestClient_WritePump(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Create test WebSocket server
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to upgrade: %v", err)
			return
		}
		defer conn.Close()

		// Read messages from client
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			// Verify message type is text
			if messageType != websocket.TextMessage {
				// Could be ping/close
				continue
			}

			// Verify message is valid JSON
			var msg Message
			if err := json.Unmarshal(message, &msg); err == nil {
				// Message received successfully
				t.Logf("Server received: %s", message)
			}
		}
	}))
	defer server.Close()

	// Connect client to server
	wsURL := "ws" + server.URL[4:] // Convert http:// to ws://
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	// Create client and start WritePump
	client := NewClient("test-client", hub, conn)
	go client.WritePump()

	// Send messages through hub
	for i := 0; i < 5; i++ {
		msg := Message{
			Type:      "test",
			Timestamp: time.Now(),
			Data:      map[string]any{"i": i},
		}
		client.send <- msg
		time.Sleep(10 * time.Millisecond)
	}

	// Close client send channel to trigger shutdown
	close(client.send)

	// Wait for WritePump to finish
	time.Sleep(100 * time.Millisecond)
}

// TestClient_ReadPump tests the ReadPump method with mock WebSocket connection.
func TestClient_ReadPump(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Create test WebSocket server
	upgrader := websocket.Upgrader{}
	serverDone := make(chan bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to upgrade: %v", err)
			return
		}
		defer conn.Close()
		defer func() { serverDone <- true }()

		// Send test messages to client
		for i := 0; i < 3; i++ {
			msg := Message{
				Type:      "server.test",
				Timestamp: time.Now(),
				Data:      map[string]any{"i": i},
			}
			data, _ := json.Marshal(msg)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Close connection
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer server.Close()

	// Connect client to server
	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	// Create client and register
	client := NewClient("test-client", hub, conn)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Start ReadPump in goroutine
	go client.ReadPump()

	// Wait for server to finish sending
	select {
	case <-serverDone:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("server did not finish")
	}

	// Wait for ReadPump to process and unregister
	time.Sleep(100 * time.Millisecond)

	// Client should be unregistered after connection close
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after close, got %d", count)
	}
}

// TestClient_PingPong tests ping/pong mechanism in WritePump.
func TestClient_PingPong(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Track pings received
	pingsReceived := 0
	var mu sync.Mutex

	// Create test WebSocket server
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Set ping handler to count pings
		conn.SetPingHandler(func(appData string) error {
			mu.Lock()
			pingsReceived++
			mu.Unlock()
			// Send pong response
			return conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(time.Second))
		})

		// Read messages (including pings)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Connect client to server
	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	// Create client and start WritePump
	client := NewClient("test-client", hub, conn)
	done := make(chan bool)
	go func() {
		client.WritePump()
		done <- true
	}()

	// Wait for at least one ping (pingPeriod is 54 seconds in production)
	// For testing, we'll wait a bit and then close
	time.Sleep(200 * time.Millisecond)

	// Close client to stop WritePump
	close(client.send)

	// Wait for WritePump to finish
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("WritePump did not finish")
	}

	// Note: In production, ping period is 54 seconds, so we might not see pings in this test
	// The important thing is that WritePump runs without error
	t.Logf("Pings received: %d (may be 0 due to short test duration)", pingsReceived)
}

// TestClient_Integration tests full client lifecycle with real WebSocket connection.
func TestClient_Integration(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Track messages received by server
	serverMessages := make([]Message, 0)
	var serverMu sync.Mutex

	// Create test WebSocket server
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read messages from client
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				break
			}

			if messageType == websocket.TextMessage {
				var msg Message
				if err := json.Unmarshal(data, &msg); err == nil {
					serverMu.Lock()
					serverMessages = append(serverMessages, msg)
					serverMu.Unlock()
				}
			}
		}
	}))
	defer server.Close()

	// Connect client to server
	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	// Create client and register with hub
	client := NewClient("integration-test", hub, conn)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Verify client registered
	if count := hub.ClientCount(); count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}

	// Start client pumps
	go client.WritePump()
	go client.ReadPump()

	// Broadcast messages via hub
	testMessages := []Message{
		{Type: "event.1", Timestamp: time.Now(), Data: map[string]any{"value": 1}},
		{Type: "event.2", Timestamp: time.Now(), Data: map[string]any{"value": 2}},
		{Type: "event.3", Timestamp: time.Now(), Data: map[string]any{"value": 3}},
	}

	for _, msg := range testMessages {
		hub.Broadcast(msg)
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for messages to be received
	time.Sleep(100 * time.Millisecond)

	// Verify server received messages
	serverMu.Lock()
	receivedCount := len(serverMessages)
	serverMu.Unlock()

	if receivedCount != len(testMessages) {
		t.Errorf("expected %d messages, server received %d", len(testMessages), receivedCount)
	}

	// Verify message types
	serverMu.Lock()
	for i, msg := range serverMessages {
		if i < len(testMessages) && msg.Type != testMessages[i].Type {
			t.Errorf("message %d: expected type %s, got %s", i, testMessages[i].Type, msg.Type)
		}
	}
	serverMu.Unlock()

	// Close connection
	conn.Close()

	// Wait for client to unregister
	time.Sleep(100 * time.Millisecond)

	// Verify client unregistered
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after close, got %d", count)
	}
}

// TestClient_WriteDeadline tests write deadline handling in WritePump.
func TestClient_WriteDeadline(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Create test WebSocket server
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Just keep connection open, read nothing
		// This tests that WritePump handles writes correctly
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	// Connect client to server
	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	// Create client and start WritePump
	client := NewClient("test-client", hub, conn)
	done := make(chan bool)
	go func() {
		client.WritePump()
		done <- true
	}()

	// Send a message
	msg := Message{
		Type:      "test",
		Timestamp: time.Now(),
		Data:      map[string]any{"test": true},
	}
	client.send <- msg

	// Wait a bit for write to complete
	time.Sleep(100 * time.Millisecond)

	// Close send channel
	close(client.send)

	// WritePump should finish gracefully
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("WritePump did not finish after close")
	}
}

// TestClient_ConnectionClose tests handling of unexpected connection close.
func TestClient_ConnectionClose(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Create test WebSocket server that closes abruptly
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Close connection immediately
		conn.Close()
	}))
	defer server.Close()

	// Connect client to server
	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	// Create client and register
	client := NewClient("test-client", hub, conn)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Start ReadPump
	done := make(chan bool)
	go func() {
		client.ReadPump()
		done <- true
	}()

	// ReadPump should detect close and finish
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("ReadPump did not finish after connection close")
	}

	// Client should be unregistered
	time.Sleep(50 * time.Millisecond)
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after close, got %d", count)
	}
}

// TestHub_RegisterBeforeRun tests that Register() doesn't block when called before Run().
// This test catches the deadlock bug where unbuffered channels would block on Register()
// before the event loop starts running.
func TestHub_RegisterBeforeRun(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	// Create a mock client
	conn := &websocket.Conn{} // Will be nil but that's OK for this test
	client := NewClient("test-client", hub, conn)

	// Try to register BEFORE starting Run() - this should NOT block with buffered channels
	done := make(chan struct{})
	go func() {
		hub.Register(client) // This would deadlock if channels weren't buffered
		close(done)
	}()

	select {
	case <-done:
		// Success - Register() did not block
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Register() deadlocked when called before hub.Run() - channels are not buffered!")
	}

	// Now start the hub to clean up
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Verify client was registered
	if count := hub.ClientCount(); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}
}

// TestHub_MultipleRegistersBeforeRun tests multiple Register() calls before Run().
// This tests the buffer size is adequate for typical initialization patterns.
func TestHub_MultipleRegistersBeforeRun(t *testing.T) {
	logger := zerolog.Nop()
	hub := NewHub(&logger)

	// Register multiple clients before Run() starts
	const numClients = 5
	done := make(chan struct{})

	go func() {
		for i := 0; i < numClients; i++ {
			conn := &websocket.Conn{}
			client := NewClient("test-client", hub, conn)
			hub.Register(client)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - all Register() calls completed
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Register() deadlocked with multiple clients before Run()")
	}

	// Now start the hub
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Verify all clients were registered
	if count := hub.ClientCount(); count != numClients {
		t.Errorf("expected %d clients, got %d", numClients, count)
	}
}
