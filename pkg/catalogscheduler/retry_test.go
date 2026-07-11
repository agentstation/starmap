package catalogscheduler

import (
	"context"
	stderrors "errors"
	"fmt"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	starmaperrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sync"
)

type sequenceSyncer struct {
	calls  atomic.Int32
	errors []error
}

type orderingLease struct {
	t        *testing.T
	jittered *atomic.Bool
	delegate Lease
}

func (l orderingLease) Acquire(ctx context.Context, request LeaseRequest) (LeaseGuard, error) {
	l.t.Helper()
	if !l.jittered.Load() {
		l.t.Fatal("lease acquired before scheduled jitter completed")
	}
	return l.delegate.Acquire(ctx, request)
}

func (s *sequenceSyncer) Sync(context.Context, ...sync.Option) (*sync.Result, error) {
	index := int(s.calls.Add(1)) - 1
	if index < len(s.errors) && s.errors[index] != nil {
		return nil, s.errors[index]
	}
	return &sync.Result{}, nil
}

func TestSchedulerRetryOnlyTransientFailuresWithBoundedBackoff(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 4, BaseDelay: 10 * time.Millisecond, MaxDelay: 25 * time.Millisecond, JitterFraction: 0.5}
	syncer := &sequenceSyncer{errors: []error{
		&starmaperrors.APIError{Provider: "test", StatusCode: 503, Message: "unavailable"},
		&starmaperrors.TimeoutError{Operation: "sync", Message: "timeout"},
		nil,
	}}
	runner, err := NewRunner(syncer, NewMemoryLease(), LeaseRequest{
		Key: DefaultLeaseKey, Owner: "retry-replica", TTL: DefaultLeaseTTL,
	}, WithRetryPolicy(policy))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	var delays []time.Duration
	runner.random = func() float64 { return 0.5 }
	runner.sleep = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}
	result, err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.Status != RunStatusSucceeded || result.Attempts != 3 ||
		fmt.Sprint(result.RetryDelays) != fmt.Sprint([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond}) ||
		fmt.Sprint(delays) != fmt.Sprint(result.RetryDelays) {
		t.Fatalf("result/delays = %#v/%v", result, delays)
	}
}

func TestSchedulerRetryPermanentFailureStopsImmediately(t *testing.T) {
	syncer := &sequenceSyncer{errors: []error{
		&starmaperrors.ValidationError{Field: "catalog", Message: "invalid"},
		nil,
	}}
	runner, err := NewRunner(syncer, NewMemoryLease(), LeaseRequest{
		Key: DefaultLeaseKey, Owner: "permanent-replica", TTL: DefaultLeaseTTL,
	}, WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: time.Second}))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.sleep = func(context.Context, time.Duration) error {
		t.Fatal("permanent failure slept for retry")
		return nil
	}
	result, err := runner.RunOnce(context.Background())
	if err == nil || result.Attempts != 1 || syncer.calls.Load() != 1 || len(result.RetryDelays) != 0 {
		t.Fatalf("result/error/calls = %#v/%v/%d", result, err, syncer.calls.Load())
	}
}

func TestSchedulerRetryExhaustionIsBounded(t *testing.T) {
	transient := &starmaperrors.APIError{Provider: "test", StatusCode: 429, Message: "rate limited"}
	syncer := &sequenceSyncer{errors: []error{transient, transient, transient, nil}}
	runner, err := NewRunner(syncer, NewMemoryLease(), LeaseRequest{
		Key: DefaultLeaseKey, Owner: "bounded-replica", TTL: DefaultLeaseTTL,
	}, WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 15 * time.Millisecond}))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.sleep = func(context.Context, time.Duration) error { return nil }
	result, err := runner.RunOnce(context.Background())
	if err == nil || result.Attempts != 3 || syncer.calls.Load() != 3 ||
		fmt.Sprint(result.RetryDelays) != fmt.Sprint([]time.Duration{10 * time.Millisecond, 15 * time.Millisecond}) {
		t.Fatalf("result/error/calls = %#v/%v/%d", result, err, syncer.calls.Load())
	}
}

func TestSchedulerJitterPrecedesLeaseAndStaysWithinWindow(t *testing.T) {
	syncer := &sequenceSyncer{}
	var jittered atomic.Bool
	runner := newTestRunner(t, syncer, orderingLease{t: t, jittered: &jittered, delegate: NewMemoryLease()}, "jitter-replica")
	runner.random = func() float64 { return 0.25 }
	var delays []time.Duration
	runner.sleep = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		jittered.Store(true)
		return nil
	}
	result, err := runner.RunScheduledOnce(context.Background(), 40*time.Second)
	if err != nil {
		t.Fatalf("RunScheduledOnce: %v", err)
	}
	if result.Status != RunStatusSucceeded || fmt.Sprint(delays) != fmt.Sprint([]time.Duration{10 * time.Second}) {
		t.Fatalf("result/delays = %#v/%v", result, delays)
	}
}

func TestSchedulerRetryCancellationStopsAndReleasesLease(t *testing.T) {
	syncer := &sequenceSyncer{errors: []error{starmaperrors.ErrProviderUnavailable}}
	lease := NewMemoryLease()
	runner, err := NewRunner(syncer, lease, LeaseRequest{
		Key: DefaultLeaseKey, Owner: "cancel-replica", TTL: DefaultLeaseTTL,
	}, WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: time.Second}))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.sleep = func(context.Context, time.Duration) error { return context.Canceled }
	result, err := runner.RunOnce(context.Background())
	if !stderrors.Is(err, context.Canceled) || result.Attempts != 1 || syncer.calls.Load() != 1 {
		t.Fatalf("result/error/calls = %#v/%v/%d", result, err, syncer.calls.Load())
	}
	next := newTestRunner(t, &sequenceSyncer{}, lease, "next-replica")
	if result, err := next.RunOnce(context.Background()); err != nil || result.Status != RunStatusSucceeded {
		t.Fatalf("lease was not released after canceled backoff: %#v/%v", result, err)
	}
}

func TestSchedulerRetryClassificationMatrix(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want RetryClass
	}{
		{name: "rate limit", err: starmaperrors.ErrRateLimited, want: RetryClassTransient},
		{name: "provider unavailable", err: starmaperrors.ErrProviderUnavailable, want: RetryClassTransient},
		{name: "timeout", err: context.DeadlineExceeded, want: RetryClassTransient},
		{name: "HTTP 408", err: &starmaperrors.APIError{StatusCode: 408}, want: RetryClassTransient},
		{name: "HTTP 425", err: &starmaperrors.APIError{StatusCode: 425}, want: RetryClassTransient},
		{name: "HTTP 500", err: &starmaperrors.APIError{StatusCode: 500}, want: RetryClassTransient},
		{name: "connection reset", err: fmt.Errorf("read provider response: %w", syscall.ECONNRESET), want: RetryClassTransient},
		{name: "HTTP 400", err: &starmaperrors.APIError{StatusCode: 400}, want: RetryClassPermanent},
		{name: "validation", err: &starmaperrors.ValidationError{Field: "x"}, want: RetryClassPermanent},
		{name: "config wraps transient", err: &starmaperrors.ConfigError{Component: "x", Err: starmaperrors.ErrProviderUnavailable}, want: RetryClassPermanent},
		{name: "conflict", err: starmaperrors.ErrConflict, want: RetryClassPermanent},
		{name: "canceled", err: context.Canceled, want: RetryClassPermanent},
		{name: "unknown", err: stderrors.New("unknown"), want: RetryClassPermanent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifyRetry(test.err); got != test.want {
				t.Fatalf("ClassifyRetry(%v) = %q, want %q", test.err, got, test.want)
			}
		})
	}
}
