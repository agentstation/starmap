package events

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/constants"
)

const brokerSubscriberQueueSize = constants.ChannelBufferSize

// Broker manages event distribution to multiple subscribers.
// It provides a central hub for catalog events, fanning them out to
// all registered subscribers (WebSocket, SSE, etc.) concurrently.
type Broker struct {
	subscribers     []*brokerSubscriber
	events          chan Event
	register        chan Subscriber
	unregister      chan Subscriber
	mu              sync.RWMutex
	logger          *zerolog.Logger
	fanout          *Fanout[Event]
	eventsPublished uint64 // atomic counter
	eventsDropped   uint64 // atomic counter
}

type brokerSubscriber struct {
	subscriber Subscriber
	queue      chan Event
	done       chan struct{}
	closeOnce  sync.Once
	logger     *zerolog.Logger
}

func newBrokerSubscriber(subscriber Subscriber, logger *zerolog.Logger) *brokerSubscriber {
	return &brokerSubscriber{
		subscriber: subscriber,
		queue:      make(chan Event, brokerSubscriberQueueSize),
		done:       make(chan struct{}),
		logger:     logger,
	}
}

func (s *brokerSubscriber) Run() {
	for {
		select {
		case <-s.done:
			return
		case event := <-s.queue:
			if err := s.subscriber.Send(event); err != nil {
				s.logger.Warn().
					Err(err).
					Str("subscriber_type", fmt.Sprintf("%T", s.subscriber)).
					Msg("Event subscriber send failed")
			}
		}
	}
}

func (s *brokerSubscriber) Send(event Event) error {
	return TrySend(s.queue, event)
}

func (s *brokerSubscriber) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		err = s.subscriber.Close()
	})
	return err
}

// NewBroker creates a new event broker.
func NewBroker(logger *zerolog.Logger) *Broker {
	return &Broker{
		subscribers: make([]*brokerSubscriber, 0),
		events:      make(chan Event, 256),
		register:    make(chan Subscriber, 10), // Buffer to prevent blocking during setup
		unregister:  make(chan Subscriber, 10), // Buffer to prevent blocking during shutdown
		logger:      logger,
		fanout:      NewFanout[Event](BackpressureSkip, logger),
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
			wrapped := newBrokerSubscriber(sub, b.logger)
			b.mu.Lock()
			b.subscribers = append(b.subscribers, wrapped)
			b.mu.Unlock()
			go wrapped.Run()
			b.logger.Debug().
				Int("total_subscribers", len(b.subscribers)).
				Msg("Internal subscriber registered")

		case sub := <-b.unregister:
			b.mu.Lock()
			for i, s := range b.subscribers {
				if s.subscriber == sub {
					b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
					_ = s.Close()
					break
				}
			}
			b.mu.Unlock()
			b.logger.Debug().
				Int("total_subscribers", len(b.subscribers)).
				Msg("Internal subscriber unregistered")

		case event := <-b.events:
			atomic.AddUint64(&b.eventsPublished, 1)

			b.mu.RLock()
			subs := make([]*brokerSubscriber, len(b.subscribers))
			copy(subs, b.subscribers)
			b.mu.RUnlock()

			targets := make([]DeliveryTarget[Event], 0, len(subs))
			for _, sub := range subs {
				subscriber := sub
				targets = append(targets, DeliveryTarget[Event]{
					ID:    fmt.Sprintf("%T", subscriber),
					Send:  subscriber.Send,
					Close: subscriber.Close,
				})
			}
			result := b.fanout.Deliver(targets, event)

			b.logger.Debug().
				Str("event_type", string(event.Type)).
				Int("subscribers", len(subs)).
				Int("sent", result.Sent).
				Int("failed", result.Failed).
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
		atomic.AddUint64(&b.eventsDropped, 1)
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

// EventsPublished returns the total number of events published.
func (b *Broker) EventsPublished() uint64 {
	return atomic.LoadUint64(&b.eventsPublished)
}

// EventsDropped returns the total number of events dropped.
func (b *Broker) EventsDropped() uint64 {
	return atomic.LoadUint64(&b.eventsDropped)
}

// QueueDepth returns the current number of events in the queue.
func (b *Broker) QueueDepth() int {
	return len(b.events)
}

// DeliveryStats returns cumulative subscriber delivery counters.
func (b *Broker) DeliveryStats() DeliveryStats {
	return b.fanout.Stats()
}
