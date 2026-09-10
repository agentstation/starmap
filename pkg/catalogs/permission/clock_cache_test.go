package permission

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestClockCacheIsPassiveAndExpires(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	var ticks atomic.Int64
	var observations atomic.Int64
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(context.Context) (ClockReading, error) {
			observations.Add(1)
			ticks.Add(int64(time.Millisecond))
			return ClockReading{Time: now, Uncertainty: time.Millisecond, Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return time.Duration(ticks.Load()), true },
		MaxAge:  time.Second, MaxDriftPPM: 500, CounterUncertainty: time.Microsecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if observations.Load() != 0 || clock.Read().Known {
		t.Fatal("construction observed or trusted an absent sample")
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	ticks.Add(int64(100 * time.Millisecond))
	reading := clock.Read()
	if !reading.Known || !reading.Time.Equal(now.Add(100*time.Millisecond)) || reading.Uncertainty <= 2*time.Millisecond {
		t.Fatalf("reading=%+v", reading)
	}
	if observations.Load() != 1 {
		t.Fatal("read observed the time service")
	}
	if got := testing.AllocsPerRun(100, func() { _ = clock.Read() }); got != 0 {
		t.Fatalf("allocations=%v", got)
	}
	ticks.Store(int64(time.Second))
	if clock.Read().Known {
		t.Fatal("sample remained valid at its expiry")
	}
	ticks.Store(int64(time.Millisecond))
	if clock.Read().Known {
		t.Fatal("expired sample revived after a counter reset")
	}
}

func TestClockCacheFailureClearsEvidence(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	nativeError := errors.New("time service unavailable")
	var failed bool
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(context.Context) (ClockReading, error) {
			if failed {
				return ClockReading{}, nativeError
			}
			return ClockReading{Time: now, Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return time.Second, true }, MaxAge: time.Second, MaxDriftPPM: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !clock.Read().Known {
		t.Fatal("qualified sample missing")
	}
	failed = true
	if err := clock.Refresh(t.Context()); !errors.Is(err, nativeError) {
		t.Fatalf("error=%v", err)
	}
	if clock.Read().Known {
		t.Fatal("failed observation retained clock authority")
	}
}

func TestClockCacheRejectsInvalidConfiguration(t *testing.T) {
	baseline := ClockCacheConfig{
		Observe: func(context.Context) (ClockReading, error) {
			t.Fatal("constructor observed")
			return ClockReading{}, nil
		},
		Elapsed: func() (time.Duration, bool) { t.Fatal("constructor sampled"); return 0, false },
		MaxAge:  time.Second, MaxDriftPPM: 500,
	}
	cases := map[string]func(*ClockCacheConfig){
		"missing observation":     func(c *ClockCacheConfig) { c.Observe = nil },
		"missing counter":         func(c *ClockCacheConfig) { c.Elapsed = nil },
		"zero age":                func(c *ClockCacheConfig) { c.MaxAge = 0 },
		"negative age":            func(c *ClockCacheConfig) { c.MaxAge = -time.Second },
		"excessive age":           func(c *ClockCacheConfig) { c.MaxAge = 5*time.Minute + time.Nanosecond },
		"zero drift":              func(c *ClockCacheConfig) { c.MaxDriftPPM = 0 },
		"unbounded drift":         func(c *ClockCacheConfig) { c.MaxDriftPPM = 1_000_000 },
		"negative counter error":  func(c *ClockCacheConfig) { c.CounterUncertainty = -time.Nanosecond },
		"excessive counter error": func(c *ClockCacheConfig) { c.CounterUncertainty = 30*time.Second + time.Nanosecond },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			config := baseline
			change(&config)
			if cache, err := NewClockCache(config); err == nil || cache != nil {
				t.Fatalf("cache=%v error=%v", cache, err)
			}
		})
	}
}

func TestClockCacheRejectsUnsafeEvidence(t *testing.T) {
	instant := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name         string
		sample       ClockReading
		ticks        []time.Duration
		known        []bool
		maxAge       time.Duration
		drift        uint32
		counterError time.Duration
	}{
		{name: "unknown source", sample: ClockReading{Time: instant}, ticks: []time.Duration{0, 0}, known: []bool{true, true}},
		{name: "zero time", sample: ClockReading{Known: true}, ticks: []time.Duration{0, 0}, known: []bool{true, true}},
		{name: "negative source error", sample: ClockReading{Time: instant, Known: true, Uncertainty: -time.Nanosecond}, ticks: []time.Duration{0, 0}, known: []bool{true, true}},
		{name: "excessive source error", sample: ClockReading{Time: instant, Known: true, Uncertainty: 31 * time.Second}, ticks: []time.Duration{0, 0}, known: []bool{true, true}},
		{name: "unknown initial counter", sample: ClockReading{Time: instant, Known: true}, ticks: []time.Duration{0}, known: []bool{false}},
		{name: "unknown final counter", sample: ClockReading{Time: instant, Known: true}, ticks: []time.Duration{0, 0}, known: []bool{true, false}},
		{name: "negative initial counter", sample: ClockReading{Time: instant, Known: true}, ticks: []time.Duration{-1}, known: []bool{true}},
		{name: "counter regressed", sample: ClockReading{Time: instant, Known: true}, ticks: []time.Duration{time.Second, 0}, known: []bool{true, true}},
		{name: "query exhausted age", sample: ClockReading{Time: instant, Known: true}, ticks: []time.Duration{0, time.Second}, known: []bool{true, true}},
		{name: "query exhausted uncertainty", sample: ClockReading{Time: instant, Known: true, Uncertainty: 30 * time.Second}, ticks: []time.Duration{0, time.Nanosecond}, known: []bool{true, true}},
		{name: "counter error exhausted age", sample: ClockReading{Time: instant, Known: true}, ticks: []time.Duration{0, 0}, known: []bool{true, true}, counterError: time.Second},
		{name: "maximum drift does not overflow", sample: ClockReading{Time: instant, Known: true}, ticks: []time.Duration{0, 4 * time.Minute}, known: []bool{true, true}, maxAge: 5 * time.Minute, drift: 999999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index := 0
			maxAge := tc.maxAge
			if maxAge == 0 {
				maxAge = time.Second
			}
			drift := tc.drift
			if drift == 0 {
				drift = 500
			}
			cache, err := NewClockCache(ClockCacheConfig{
				Observe: func(context.Context) (ClockReading, error) { return tc.sample, nil },
				Elapsed: func() (time.Duration, bool) {
					value, known := tc.ticks[index], tc.known[index]
					index++
					return value, known
				},
				MaxAge: maxAge, MaxDriftPPM: drift, CounterUncertainty: tc.counterError,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := cache.Refresh(t.Context()); err == nil {
				t.Fatal("unsafe evidence accepted")
			}
			if cache.Read().Known {
				t.Fatal("unsafe evidence remained available")
			}
		})
	}
}

func TestClockCacheRefusesSuspendAndCounterFailure(t *testing.T) {
	for _, kind := range []string{"suspend", "regression", "unknown", "drift expiry"} {
		t.Run(kind, func(t *testing.T) {
			var ticks atomic.Int64
			ticks.Store(int64(time.Second))
			var known atomic.Bool
			known.Store(true)
			clock, err := NewClockCache(ClockCacheConfig{
				Observe: func(context.Context) (ClockReading, error) {
					return ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}, nil
				},
				Elapsed: func() (time.Duration, bool) { return time.Duration(ticks.Load()), known.Load() },
				MaxAge:  time.Second, MaxDriftPPM: 500,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := clock.Refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "suspend":
				ticks.Add(int64(time.Hour))
			case "regression":
				ticks.Store(0)
			case "unknown":
				known.Store(false)
			case "drift expiry":
				ticks.Add(int64(time.Second - time.Nanosecond))
			}
			if clock.Read().Known {
				t.Fatal("unsafe counter permitted a read")
			}
			ticks.Store(int64(time.Second))
			known.Store(true)
			if clock.Read().Known {
				t.Fatal("invalid evidence revived")
			}
			if err := clock.Refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			if !clock.Read().Known {
				t.Fatal("fresh evidence did not recover")
			}
		})
	}
}

func TestClockCacheInvalidationFencesInFlightObservation(t *testing.T) {
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var calls atomic.Int64
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(ctx context.Context) (ClockReading, error) {
			if calls.Add(1) == 2 {
				close(started)
				select {
				case <-release:
				case <-ctx.Done():
					return ClockReading{}, ctx.Err()
				}
			}
			return ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return 0, true }, MaxAge: time.Second, MaxDriftPPM: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	go func() { done <- clock.Refresh(t.Context()) }()
	<-started
	if !clock.Read().Known {
		t.Fatal("valid prior sample disappeared during observation")
	}
	clock.Invalidate()
	if clock.Read().Known {
		t.Fatal("invalidation preserved evidence")
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("in-flight observation undid invalidation")
	}
	if clock.Read().Known {
		t.Fatal("in-flight evidence became available")
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !clock.Read().Known {
		t.Fatal("new observation did not recover")
	}
}

func TestClockCacheCanceledCallerPreservesEvidence(t *testing.T) {
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(context.Context) (ClockReading, error) {
			return ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return 0, true }, MaxAge: time.Second, MaxDriftPPM: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := clock.Refresh(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if !clock.Read().Known {
		t.Fatal("an already canceled caller cleared clock evidence")
	}
}

func TestClockCacheBoundsObservationAndIssuerValidity(t *testing.T) {
	var ticks atomic.Int64
	instant := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(ctx context.Context) (ClockReading, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > time.Second {
				t.Fatal("observation lacks its finite deadline")
			}
			return ClockReading{Time: instant, Uncertainty: time.Millisecond, Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return time.Duration(ticks.Load()), true }, MaxAge: time.Second, MaxDriftPPM: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	store := storage.NewMemory()
	generation := issuerGeneration(t, "clock-cache-issuer", 1)
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuer(store, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: clock.Read})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.ReadPermission(t.Context()); err == nil {
		t.Fatal("cold cache authorized a receipt")
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	receipt, err := issuer.ReadPermission(t.Context())
	if err != nil || receipt.Head != generation.Manifest.AuthorityHead {
		t.Fatalf("receipt=%+v error=%v", receipt, err)
	}
	ticks.Store(int64(time.Hour))
	if _, err := issuer.ReadPermission(t.Context()); err == nil {
		t.Fatal("suspend extended receipt issuance")
	}
	head, err := store.CurrentAuthorityHead(t.Context())
	if err != nil || head != generation.Manifest.AuthorityHead {
		t.Fatalf("diagnostic head=%+v error=%v", head, err)
	}
}

func TestClockCacheConcurrentReadsKeepCompleteEvidence(t *testing.T) {
	first := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Second)
	var observations atomic.Int64
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(context.Context) (ClockReading, error) {
			if observations.Add(1)%2 == 0 {
				return ClockReading{Time: second, Uncertainty: 2 * time.Millisecond, Known: true}, nil
			}
			return ClockReading{Time: first, Uncertainty: time.Millisecond, Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return 0, true }, MaxAge: time.Second, MaxDriftPPM: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 1000 {
				value := clock.Read()
				if value.Known && !((value.Time.Equal(first) && value.Uncertainty == time.Millisecond) || (value.Time.Equal(second) && value.Uncertainty == 2*time.Millisecond)) {
					t.Errorf("mixed clock sample=%+v", value)
					return
				}
			}
		})
	}
	for range 100 {
		if err := clock.Refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	workers.Wait()
}

func TestClockCacheRefreshCancellation(t *testing.T) {
	started, done := make(chan struct{}), make(chan error, 1)
	var observations atomic.Int64
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(ctx context.Context) (ClockReading, error) {
			if observations.Add(1) > 1 {
				close(started)
				<-ctx.Done()
				return ClockReading{}, ctx.Err()
			}
			return ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return 0, true }, MaxAge: time.Second, MaxDriftPPM: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { done <- clock.Refresh(ctx) }()
	<-started
	waiter, stop := context.WithCancel(t.Context())
	stop()
	if err := clock.Refresh(waiter); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter error=%v", err)
	}
	if !clock.Read().Known {
		t.Fatal("canceled waiter cleared current evidence")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("observation error=%v", err)
	}
	if clock.Read().Known {
		t.Fatal("failed observation preserved evidence")
	}
}

func BenchmarkClockCacheRead(b *testing.B) {
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(context.Context) (ClockReading, error) {
			return ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return time.Second, true }, MaxAge: time.Second, MaxDriftPPM: 500,
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := clock.Refresh(b.Context()); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if !clock.Read().Known {
			b.Fatal("cached clock lost validity")
		}
	}
}

func TestClockCacheRoundsUncertaintyUp(t *testing.T) {
	var ticks atomic.Int64
	clock, err := NewClockCache(ClockCacheConfig{
		Observe: func(context.Context) (ClockReading, error) {
			ticks.Add(5)
			return ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Uncertainty: 2 * time.Nanosecond, Known: true}, nil
		},
		Elapsed: func() (time.Duration, bool) { return time.Duration(ticks.Load()), true },
		MaxAge:  time.Second, MaxDriftPPM: 1, CounterUncertainty: time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := clock.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := clock.Read(); !got.Known || got.Uncertainty != 10*time.Nanosecond {
		t.Fatalf("rounded sample=%+v", got)
	}
}

func TestClockCacheUnconstructedValuesRefuse(t *testing.T) {
	for _, clock := range []*ClockCache{nil, {}} {
		if clock.Read().Known {
			t.Fatal("unconstructed clock is qualified")
		}
		clock.Invalidate()
		if err := clock.Refresh(t.Context()); err == nil {
			t.Fatal("unconstructed clock refreshed")
		}
		if err := clock.Refresh(nil); err == nil {
			t.Fatal("nil context accepted")
		}
	}
}
