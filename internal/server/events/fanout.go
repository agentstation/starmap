package events

import (
	"errors"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// ErrBackpressure reports that a delivery target cannot accept an event now.
var ErrBackpressure = errors.New("event delivery backpressure")

// BackpressurePolicy controls what fan-out does when a target is full.
type BackpressurePolicy string

const (
	// BackpressureSkip drops the event for the slow target and keeps it connected.
	BackpressureSkip BackpressurePolicy = "skip"
	// BackpressureDisconnect drops the slow target from future fan-out.
	BackpressureDisconnect BackpressurePolicy = "disconnect"
)

// DeliveryTarget is one event fan-out destination.
type DeliveryTarget[T any] struct {
	ID    string
	Send  func(T) error
	Close func() error
}

// DeliveryResult describes one fan-out attempt.
type DeliveryResult struct {
	Sent         int
	Skipped      int
	Disconnected int
	Failed       int
}

// DeliveryStats contains cumulative fan-out counters.
type DeliveryStats struct {
	Sent         uint64
	Skipped      uint64
	Disconnected uint64
	Failed       uint64
}

// Fanout delivers events to targets using one explicit backpressure policy.
type Fanout[T any] struct {
	policy BackpressurePolicy
	logger *zerolog.Logger

	sent         uint64
	skipped      uint64
	disconnected uint64
	failed       uint64
}

// NewFanout creates a fan-out dispatcher.
func NewFanout[T any](policy BackpressurePolicy, logger *zerolog.Logger) *Fanout[T] {
	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
	}
	return &Fanout[T]{
		policy: policy,
		logger: logger,
	}
}

// Deliver sends an item to all targets without spawning per-target goroutines.
func (f *Fanout[T]) Deliver(targets []DeliveryTarget[T], item T) DeliveryResult {
	var result DeliveryResult

	for _, target := range targets {
		if target.Send == nil {
			result.Failed++
			f.logger.Warn().
				Str("target_id", target.ID).
				Msg("Event delivery target has no send function")
			continue
		}

		if err := target.Send(item); err != nil {
			if errors.Is(err, ErrBackpressure) {
				f.handleBackpressure(target, &result)
				continue
			}

			result.Failed++
			f.logger.Warn().
				Err(err).
				Str("target_id", target.ID).
				Msg("Event delivery target failed")
			continue
		}

		result.Sent++
	}

	addUint64(&f.sent, result.Sent)
	addUint64(&f.skipped, result.Skipped)
	addUint64(&f.disconnected, result.Disconnected)
	addUint64(&f.failed, result.Failed)

	return result
}

func addUint64(counter *uint64, value int) {
	if value <= 0 {
		return
	}
	atomic.AddUint64(counter, uint64(value))
}

// Stats returns cumulative fan-out counters.
func (f *Fanout[T]) Stats() DeliveryStats {
	return DeliveryStats{
		Sent:         atomic.LoadUint64(&f.sent),
		Skipped:      atomic.LoadUint64(&f.skipped),
		Disconnected: atomic.LoadUint64(&f.disconnected),
		Failed:       atomic.LoadUint64(&f.failed),
	}
}

func (f *Fanout[T]) handleBackpressure(target DeliveryTarget[T], result *DeliveryResult) {
	switch f.policy {
	case BackpressureDisconnect:
		result.Disconnected++
		if target.Close != nil {
			if err := target.Close(); err != nil {
				result.Failed++
				f.logger.Warn().
					Err(err).
					Str("target_id", target.ID).
					Msg("Failed to close backpressured event delivery target")
			}
		}
		f.logger.Warn().
			Str("target_id", target.ID).
			Msg("Event delivery target backpressured; disconnected")
	default:
		result.Skipped++
		f.logger.Warn().
			Str("target_id", target.ID).
			Msg("Event delivery target backpressured; skipped event")
	}
}

// TrySend attempts a non-blocking channel send and reports backpressure if full.
func TrySend[T any](ch chan<- T, item T) error {
	select {
	case ch <- item:
		return nil
	default:
		return ErrBackpressure
	}
}
