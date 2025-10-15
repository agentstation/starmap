package events

// Subscriber is an interface for event consumers.
// Implementations adapt the unified event stream to specific transport
// mechanisms (WebSocket, SSE, MQTT, webhooks, etc.).
type Subscriber interface {
	// Send delivers an event to the subscriber.
	// Implementations should be non-blocking and handle errors gracefully.
	Send(Event) error

	// Close cleanly shuts down the subscriber.
	Close() error
}
