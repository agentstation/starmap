package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// mockSubscriber is a mock subscriber for testing.
type mockSubscriber struct {
	events []Event
	mu     sync.Mutex
	closed bool
}

func newMockSubscriber() *mockSubscriber {
	return &mockSubscriber{
		events: make([]Event, 0),
	}
}

func (m *mockSubscriber) Send(event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockSubscriber) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockSubscriber) EventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

// TestBroker_NewBroker tests broker creation.
func TestBroker_NewBroker(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroker(&logger)

	if b == nil {
		t.Fatal("NewBroker returned nil")
	}

	if b.subscribers == nil {
		t.Error("subscribers slice not initialized")
	}

	if b.events == nil {
		t.Error("events channel not initialized")
	}

	if b.register == nil {
		t.Error("register channel not initialized")
	}

	if b.unregister == nil {
		t.Error("unregister channel not initialized")
	}
}

// TestBroker_BasicOperation tests basic broker operations.
func TestBroker_BasicOperation(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroker(&logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Subscribe
	sub := newMockSubscriber()
	b.Subscribe(sub)
	time.Sleep(10 * time.Millisecond)

	if count := b.SubscriberCount(); count != 1 {
		t.Fatalf("expected 1 subscriber, got %d", count)
	}

	// Publish event
	b.Publish(ModelAdded, map[string]any{"model": "test"})
	time.Sleep(50 * time.Millisecond)

	// Verify event received
	if count := sub.EventCount(); count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

// TestBroker_Shutdown tests graceful shutdown.
func TestBroker_Shutdown(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroker(&logger)

	ctx, cancel := context.WithCancel(context.Background())

	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Subscribe
	sub1 := newMockSubscriber()
	sub2 := newMockSubscriber()
	b.Subscribe(sub1)
	b.Subscribe(sub2)
	time.Sleep(10 * time.Millisecond)

	if count := b.SubscriberCount(); count != 2 {
		t.Fatalf("expected 2 subscribers, got %d", count)
	}

	// Trigger shutdown
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Verify all subscribers disconnected
	if count := b.SubscriberCount(); count != 0 {
		t.Errorf("expected 0 subscribers after shutdown, got %d", count)
	}
}
