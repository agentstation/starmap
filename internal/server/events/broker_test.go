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

// TestBroker_SubscribeBeforeRun tests that Subscribe() doesn't block when called before Run().
// This test catches the deadlock bug where unbuffered channels would block on Subscribe()
// before the event loop starts running.
func TestBroker_SubscribeBeforeRun(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroker(&logger)

	// Try to subscribe BEFORE starting Run() - this should NOT block with buffered channels
	done := make(chan struct{})
	go func() {
		sub := newMockSubscriber()
		b.Subscribe(sub) // This would deadlock if channels weren't buffered
		close(done)
	}()

	select {
	case <-done:
		// Success - Subscribe() did not block
	case <-time.After(2 * time.Second):
		t.Fatal("broker.Subscribe() deadlocked when called before broker.Run() - channels are not buffered!")
	}

	// Now start the broker to clean up
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go b.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Verify subscriber was registered
	if count := b.SubscriberCount(); count != 1 {
		t.Errorf("expected 1 subscriber, got %d", count)
	}
}

// TestBroker_MultipleSubscribersBeforeRun tests multiple Subscribe() calls before Run().
// This tests the buffer size is adequate for typical initialization patterns.
func TestBroker_MultipleSubscribersBeforeRun(t *testing.T) {
	logger := zerolog.Nop()
	b := NewBroker(&logger)

	// Subscribe multiple times before Run() starts
	const numSubscribers = 5
	done := make(chan struct{})

	go func() {
		for i := 0; i < numSubscribers; i++ {
			sub := newMockSubscriber()
			b.Subscribe(sub)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - all Subscribe() calls completed
	case <-time.After(2 * time.Second):
		t.Fatal("broker.Subscribe() deadlocked with multiple subscribers before Run()")
	}

	// Now start the broker
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go b.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Verify all subscribers were registered
	if count := b.SubscriberCount(); count != numSubscribers {
		t.Errorf("expected %d subscribers, got %d", numSubscribers, count)
	}
}
