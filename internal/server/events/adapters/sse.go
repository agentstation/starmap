package adapters

import (
	"fmt"

	"github.com/agentstation/starmap/internal/server/events"
	"github.com/agentstation/starmap/internal/server/sse"
)

// SSESubscriber adapts the SSE broadcaster to the Subscriber interface.
type SSESubscriber struct {
	broadcaster *sse.Broadcaster
}

// NewSSESubscriber creates a new SSE subscriber.
func NewSSESubscriber(broadcaster *sse.Broadcaster) *SSESubscriber {
	return &SSESubscriber{broadcaster: broadcaster}
}

// Send delivers an event to all SSE clients.
func (s *SSESubscriber) Send(event events.Event) error {
	s.broadcaster.Broadcast(sse.Event{
		Event: string(event.Type),
		ID:    fmt.Sprintf("%d", event.Timestamp.Unix()),
		Data:  event.Data,
	})
	return nil
}

// Close is a no-op for SSE (broadcaster manages its own lifecycle).
func (s *SSESubscriber) Close() error {
	return nil
}
