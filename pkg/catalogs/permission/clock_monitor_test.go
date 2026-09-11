package permission

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func monitorTestConfig(observe func(context.Context) (ClockReading, error)) ClockCacheConfig {
	start := time.Now()
	return ClockCacheConfig{
		Observe: observe, Elapsed: func() (time.Duration, bool) { return time.Since(start), true },
		MaxAge: time.Minute, MaxDriftPPM: 500, CounterUncertainty: time.Microsecond,
	}
}

func TestClockMonitorStartsExplicitlyAndStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int64
		monitor, err := NewClockMonitor(monitorTestConfig(func(context.Context) (ClockReading, error) {
			calls.Add(1)
			return ClockReading{Time: time.Now(), Uncertainty: time.Millisecond, Known: true}, nil
		}), 10*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer monitor.Close()
		time.Sleep(time.Minute)
		if calls.Load() != 0 || monitor.Read().Known || monitor.Status().Running {
			t.Fatal("construction or passive reads started observations")
		}
		if err := monitor.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if calls.Load() != 1 || !monitor.Read().Known || !monitor.Status().Running {
			t.Fatal("explicit startup did not obtain the first sample")
		}
		if got := testing.AllocsPerRun(100, func() { _ = monitor.Read() }); got != 0 {
			t.Fatalf("cached read allocations=%v", got)
		}
		time.Sleep(10 * time.Second)
		synctest.Wait()
		if calls.Load() != 2 || monitor.Status().Attempts != 2 {
			t.Fatal("scheduled refresh did not run")
		}
		monitor.Close()
		if monitor.Read().Known || monitor.Status().Running {
			t.Fatal("shutdown retained usable evidence")
		}
		time.Sleep(time.Minute)
		if calls.Load() != 2 {
			t.Fatal("observation ran after shutdown")
		}
		if err := monitor.Start(t.Context()); err == nil {
			t.Fatal("closed monitor restarted")
		}
	})
}

func TestClockMonitorRecoversFromUnavailableSource(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		unavailable := errors.New("time service unavailable")
		var fail atomic.Bool
		fail.Store(true)
		monitor, err := NewClockMonitor(monitorTestConfig(func(context.Context) (ClockReading, error) {
			if fail.Load() {
				return ClockReading{}, unavailable
			}
			return ClockReading{Time: time.Now(), Known: true}, nil
		}), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer monitor.Close()
		if err := monitor.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if status := monitor.Status(); !status.Running || status.Known || !errors.Is(status.LastError, unavailable) {
			t.Fatalf("failed observation status=%+v", status)
		}
		fail.Store(false)
		time.Sleep(time.Second)
		synctest.Wait()
		if status := monitor.Status(); !status.Known || status.LastError != nil || status.Attempts != 2 {
			t.Fatalf("recovered status=%+v", status)
		}
		fail.Store(true)
		time.Sleep(time.Second)
		synctest.Wait()
		if monitor.Read().Known {
			t.Fatal("failed refresh retained permission time")
		}
	})
}

func TestClockMonitorCancellationDuringObservation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var calls atomic.Int64
		monitor, err := NewClockMonitor(monitorTestConfig(func(ctx context.Context) (ClockReading, error) {
			if calls.Add(1) > 1 {
				<-ctx.Done()
			}
			return ClockReading{Time: time.Now(), Known: true}, nil
		}), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer monitor.Close()
		if err := monitor.Start(ctx); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if !monitor.Read().Known {
			t.Fatal("initial sample missing")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		cancel()
		if monitor.Read().Known {
			t.Fatal("canceled host retained permission time")
		}
		monitor.Close()
		if monitor.Read().Known || monitor.Status().Running || calls.Load() != 2 {
			t.Fatal("late observation restored evidence or survived close")
		}
	})
}

func TestClockMonitorExpiresWhileRefreshIsBlocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int64
		monitor, err := NewClockMonitor(monitorTestConfig(func(ctx context.Context) (ClockReading, error) {
			if calls.Add(1) > 1 {
				<-ctx.Done()
				return ClockReading{}, ctx.Err()
			}
			return ClockReading{Time: time.Now(), Known: true}, nil
		}), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer monitor.Close()
		if err := monitor.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(time.Minute)
		synctest.Wait()
		if monitor.Read().Known || monitor.Status().Known || calls.Load() != 2 {
			t.Fatal("blocked refresh extended the old sample or overlapped another refresh")
		}
	})
}

func TestClockMonitorRejectsInvalidLifecycle(t *testing.T) {
	config := monitorTestConfig(func(context.Context) (ClockReading, error) { return ClockReading{}, nil })
	for _, interval := range []time.Duration{-time.Second, 0, config.MaxAge / 2, config.MaxAge} {
		if monitor, err := NewClockMonitor(config, interval); err == nil || monitor != nil {
			t.Fatalf("accepted interval=%v", interval)
		}
	}
	invalid := config
	invalid.Observe = nil
	if monitor, err := NewClockMonitor(invalid, time.Second); err == nil || monitor != nil {
		t.Fatal("accepted an invalid observation source")
	}
	for _, monitor := range []*ClockMonitor{nil, {}} {
		if err := monitor.Start(t.Context()); err == nil {
			t.Fatal("unconstructed monitor started")
		}
		monitor.Close()
		if monitor.Read().Known || monitor.Status().Running {
			t.Fatal("unconstructed monitor reported evidence")
		}
	}
	monitor, err := NewClockMonitor(config, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer monitor.Close()
	if err := monitor.Start(nil); err == nil {
		t.Fatal("accepted nil context")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := monitor.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled startup error=%v", err)
	}
	monitor.Close()
	if err := monitor.Start(t.Context()); err == nil {
		t.Fatal("start after close succeeded")
	}
}

func TestClockMonitorConcurrentStartAndClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int64
		monitor, err := NewClockMonitor(monitorTestConfig(func(ctx context.Context) (ClockReading, error) {
			calls.Add(1)
			<-ctx.Done()
			return ClockReading{}, ctx.Err()
		}), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		var started atomic.Int64
		var group sync.WaitGroup
		for range 20 {
			group.Go(func() {
				if err := monitor.Start(t.Context()); err == nil {
					started.Add(1)
				}
			})
		}
		group.Wait()
		synctest.Wait()
		if started.Load() != 1 || calls.Load() != 1 {
			t.Fatal("concurrent startup created multiple workers")
		}
		for range 20 {
			group.Go(monitor.Close)
		}
		group.Wait()
		if monitor.Read().Known || monitor.Status().Running {
			t.Fatal("concurrent close retained evidence")
		}
	})
}
