package events

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Broker manages event distribution to multiple subscribers.
// It provides a central hub for catalog events, fanning them out to
// all registered subscribers (WebSocket, SSE, etc.) concurrently.
type Broker struct {
	subscribers []Subscriber
	events      chan Event
	register    chan Subscriber
	unregister  chan Subscriber
	mu          sync.RWMutex
	logger      *zerolog.Logger
}

// NewBroker creates a new event broker.
func NewBroker(logger *zerolog.Logger) *Broker {
	return &Broker{
		subscribers: make([]Subscriber, 0),
		events:      make(chan Event, 256),
		register:    make(chan Subscriber),
		unregister:  make(chan Subscriber),
		logger:      logger,
	}
}

// Run starts the broker's event loop. Should be called in a goroutine.
// The broker will run until the context is cancelled.
func (b *Broker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown: close all subscribers
			b.mu.Lock()
			for _, sub := range b.subscribers {
				_ = sub.Close()
			}
			b.subscribers = nil
			b.mu.Unlock()
			b.logger.Info().Msg("Event broker shut down")
			return

		case sub := <-b.register:
			b.mu.Lock()
			b.subscribers = append(b.subscribers, sub)
			b.mu.Unlock()
			b.logger.Info().
				Int("total_subscribers", len(b.subscribers)).
				Msg("Subscriber registered")

		case sub := <-b.unregister:
			b.mu.Lock()
			for i, s := range b.subscribers {
				if s == sub {
					b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
					_ = s.Close()
					break
				}
			}
			b.mu.Unlock()
			b.logger.Info().
				Int("total_subscribers", len(b.subscribers)).
				Msg("Subscriber unregistered")

		case event := <-b.events:
			b.mu.RLock()
			subs := make([]Subscriber, len(b.subscribers))
			copy(subs, b.subscribers)
			b.mu.RUnlock()

			// Fan-out to all subscribers concurrently
			for _, sub := range subs {
				go func(s Subscriber, e Event) {
					if err := s.Send(e); err != nil {
						b.logger.Warn().
							Err(err).
							Str("event_type", string(e.Type)).
							Msg("Failed to send event to subscriber")
					}
				}(sub, event)
			}

			b.logger.Debug().
				Str("event_type", string(event.Type)).
				Int("subscribers", len(subs)).
				Msg("Event broadcasted")
		}
	}
}

// Publish sends an event to all subscribers.
func (b *Broker) Publish(eventType EventType, data any) {
	event := Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}

	select {
	case b.events <- event:
	default:
		b.logger.Warn().
			Str("event_type", string(eventType)).
			Msg("Event channel full, event dropped")
	}
}

// Subscribe registers a new subscriber to receive events.
func (b *Broker) Subscribe(sub Subscriber) {
	b.register <- sub
}

// Unsubscribe removes a subscriber from receiving events.
func (b *Broker) Unsubscribe(sub Subscriber) {
	b.unregister <- sub
}

// SubscriberCount returns the current number of subscribers.
func (b *Broker) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
