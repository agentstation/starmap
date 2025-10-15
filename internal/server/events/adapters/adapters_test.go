package adapters

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/server/events"
	"github.com/agentstation/starmap/internal/server/sse"
	ws "github.com/agentstation/starmap/internal/server/websocket"
	"github.com/rs/zerolog"
)

// TestNewSSESubscriber tests SSE subscriber creation.
func TestNewSSESubscriber(t *testing.T) {
	logger := zerolog.Nop()
	broadcaster := sse.NewBroadcaster(&logger)

	sub := NewSSESubscriber(broadcaster)

	if sub == nil {
		t.Fatal("NewSSESubscriber returned nil")
	}

	if sub.broadcaster != broadcaster {
		t.Error("broadcaster not set correctly")
	}
}

// TestSSESubscriber_Send tests sending events via SSE adapter.
func TestSSESubscriber_Send(t *testing.T) {
	logger := zerolog.Nop()
	broadcaster := sse.NewBroadcaster(&logger)
	sub := NewSSESubscriber(broadcaster)

	// Test sending various event types
	testEvents := []events.Event{
		{Type: events.ModelAdded, Timestamp: time.Now(), Data: map[string]any{"model": "gpt-4"}},
		{Type: events.ModelUpdated, Timestamp: time.Now(), Data: map[string]any{"model": "claude-3"}},
		{Type: events.ModelDeleted, Timestamp: time.Now(), Data: map[string]any{"id": "gpt-3"}},
		{Type: events.SyncStarted, Timestamp: time.Now(), Data: map[string]any{"provider": "openai"}},
		{Type: events.SyncCompleted, Timestamp: time.Now(), Data: map[string]any{"count": 10}},
		{Type: events.ClientConnected, Timestamp: time.Now(), Data: map[string]any{"id": "client-1"}},
	}

	for i, event := range testEvents {
		err := sub.Send(event)
		if err != nil {
			t.Errorf("event %d: Send() returned error: %v", i, err)
		}
	}
}

// TestSSESubscriber_Send_WithNilData tests sending event with nil data.
func TestSSESubscriber_Send_WithNilData(t *testing.T) {
	logger := zerolog.Nop()
	broadcaster := sse.NewBroadcaster(&logger)
	sub := NewSSESubscriber(broadcaster)

	event := events.Event{
		Type:      events.ModelAdded,
		Timestamp: time.Now(),
		Data:      nil,
	}

	err := sub.Send(event)
	if err != nil {
		t.Errorf("Send() with nil data returned error: %v", err)
	}
}

// TestSSESubscriber_Send_WithComplexData tests sending event with complex data types.
func TestSSESubscriber_Send_WithComplexData(t *testing.T) {
	logger := zerolog.Nop()
	broadcaster := sse.NewBroadcaster(&logger)
	sub := NewSSESubscriber(broadcaster)

	complexData := map[string]any{
		"models": []string{"gpt-4", "claude-3", "gemini-pro"},
		"count":  100,
		"metadata": map[string]any{
			"provider": "openai",
			"version":  "v1",
		},
		"tags": []string{"production", "verified"},
	}

	event := events.Event{
		Type:      events.SyncCompleted,
		Timestamp: time.Now(),
		Data:      complexData,
	}

	err := sub.Send(event)
	if err != nil {
		t.Errorf("Send() with complex data returned error: %v", err)
	}
}

// TestSSESubscriber_Close tests closing SSE subscriber.
func TestSSESubscriber_Close(t *testing.T) {
	logger := zerolog.Nop()
	broadcaster := sse.NewBroadcaster(&logger)
	sub := NewSSESubscriber(broadcaster)

	// Close should be a no-op and not return error
	err := sub.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Should be able to call Close multiple times
	err = sub.Close()
	if err != nil {
		t.Errorf("second Close() returned error: %v", err)
	}

	// Should still be able to send after close (since Close is a no-op)
	event := events.Event{
		Type:      events.ModelAdded,
		Timestamp: time.Now(),
		Data:      map[string]any{"test": true},
	}

	err = sub.Send(event)
	if err != nil {
		t.Errorf("Send() after Close() returned error: %v", err)
	}
}

// TestNewWebSocketSubscriber tests WebSocket subscriber creation.
func TestNewWebSocketSubscriber(t *testing.T) {
	logger := zerolog.Nop()
	hub := ws.NewHub(&logger)

	sub := NewWebSocketSubscriber(hub)

	if sub == nil {
		t.Fatal("NewWebSocketSubscriber returned nil")
	}

	if sub.hub != hub {
		t.Error("hub not set correctly")
	}
}

// TestWebSocketSubscriber_Send tests sending events via WebSocket adapter.
func TestWebSocketSubscriber_Send(t *testing.T) {
	logger := zerolog.Nop()
	hub := ws.NewHub(&logger)
	sub := NewWebSocketSubscriber(hub)

	// Test sending various event types
	testEvents := []events.Event{
		{Type: events.ModelAdded, Timestamp: time.Now(), Data: map[string]any{"model": "gpt-4"}},
		{Type: events.ModelUpdated, Timestamp: time.Now(), Data: map[string]any{"model": "claude-3"}},
		{Type: events.ModelDeleted, Timestamp: time.Now(), Data: map[string]any{"id": "gpt-3"}},
		{Type: events.SyncStarted, Timestamp: time.Now(), Data: map[string]any{"provider": "openai"}},
		{Type: events.SyncCompleted, Timestamp: time.Now(), Data: map[string]any{"count": 50}},
		{Type: events.ClientConnected, Timestamp: time.Now(), Data: map[string]any{"id": "ws-1"}},
	}

	for i, event := range testEvents {
		err := sub.Send(event)
		if err != nil {
			t.Errorf("event %d: Send() returned error: %v", i, err)
		}
	}
}

// TestWebSocketSubscriber_Send_WithNilData tests sending event with nil data.
func TestWebSocketSubscriber_Send_WithNilData(t *testing.T) {
	logger := zerolog.Nop()
	hub := ws.NewHub(&logger)
	sub := NewWebSocketSubscriber(hub)

	event := events.Event{
		Type:      events.ModelAdded,
		Timestamp: time.Now(),
		Data:      nil,
	}

	err := sub.Send(event)
	if err != nil {
		t.Errorf("Send() with nil data returned error: %v", err)
	}
}

// TestWebSocketSubscriber_Send_WithComplexData tests sending event with complex data types.
func TestWebSocketSubscriber_Send_WithComplexData(t *testing.T) {
	logger := zerolog.Nop()
	hub := ws.NewHub(&logger)
	sub := NewWebSocketSubscriber(hub)

	complexData := map[string]any{
		"models": []string{"gpt-4", "claude-3", "gemini-pro"},
		"count":  100,
		"metadata": map[string]any{
			"provider": "openai",
			"version":  "v1",
		},
		"tags": []string{"production", "verified"},
	}

	event := events.Event{
		Type:      events.SyncCompleted,
		Timestamp: time.Now(),
		Data:      complexData,
	}

	err := sub.Send(event)
	if err != nil {
		t.Errorf("Send() with complex data returned error: %v", err)
	}
}

// TestWebSocketSubscriber_Close tests closing WebSocket subscriber.
func TestWebSocketSubscriber_Close(t *testing.T) {
	logger := zerolog.Nop()
	hub := ws.NewHub(&logger)
	sub := NewWebSocketSubscriber(hub)

	// Close should be a no-op and not return error
	err := sub.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Should be able to call Close multiple times
	err = sub.Close()
	if err != nil {
		t.Errorf("second Close() returned error: %v", err)
	}

	// Should still be able to send after close (since Close is a no-op)
	event := events.Event{
		Type:      events.ModelAdded,
		Timestamp: time.Now(),
		Data:      map[string]any{"test": true},
	}

	err = sub.Send(event)
	if err != nil {
		t.Errorf("Send() after Close() returned error: %v", err)
	}
}

// TestAdapters_EventTypeConversion tests that all event types are handled correctly.
func TestAdapters_EventTypeConversion(t *testing.T) {
	eventTypes := []events.EventType{
		events.ModelAdded,
		events.ModelUpdated,
		events.ModelDeleted,
		events.SyncStarted,
		events.SyncCompleted,
		events.ClientConnected,
	}

	logger := zerolog.Nop()

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			// Test SSE subscriber
			sseBroadcaster := sse.NewBroadcaster(&logger)
			sseSub := NewSSESubscriber(sseBroadcaster)

			sseEvent := events.Event{
				Type:      eventType,
				Timestamp: time.Now(),
				Data:      map[string]any{"test": true},
			}

			if err := sseSub.Send(sseEvent); err != nil {
				t.Errorf("SSE Send() failed: %v", err)
			}

			// Test WebSocket subscriber
			wsHub := ws.NewHub(&logger)
			wsSub := NewWebSocketSubscriber(wsHub)

			wsEvent := events.Event{
				Type:      eventType,
				Timestamp: time.Now(),
				Data:      map[string]any{"test": true},
			}

			if err := wsSub.Send(wsEvent); err != nil {
				t.Errorf("WebSocket Send() failed: %v", err)
			}
		})
	}
}

// TestAdapters_ConcurrentSend tests concurrent sending to ensure thread safety.
func TestAdapters_ConcurrentSend(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("SSE concurrent", func(t *testing.T) {
		broadcaster := sse.NewBroadcaster(&logger)
		sub := NewSSESubscriber(broadcaster)

		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(id int) {
				defer func() { done <- true }()
				for j := 0; j < 10; j++ {
					event := events.Event{
						Type:      events.ModelAdded,
						Timestamp: time.Now(),
						Data:      map[string]any{"id": id, "iteration": j},
					}
					if err := sub.Send(event); err != nil {
						t.Errorf("goroutine %d: Send() failed: %v", id, err)
					}
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("WebSocket concurrent", func(t *testing.T) {
		hub := ws.NewHub(&logger)
		sub := NewWebSocketSubscriber(hub)

		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(id int) {
				defer func() { done <- true }()
				for j := 0; j < 10; j++ {
					event := events.Event{
						Type:      events.ModelAdded,
						Timestamp: time.Now(),
						Data:      map[string]any{"id": id, "iteration": j},
					}
					if err := sub.Send(event); err != nil {
						t.Errorf("goroutine %d: Send() failed: %v", id, err)
					}
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
