package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func runtimeTestClockMonitor(t *testing.T, observe func(context.Context) (permission.ClockReading, error)) *permission.ClockMonitor {
	t.Helper()
	start := time.Now()
	monitor, err := permission.NewClockMonitor(permission.ClockCacheConfig{
		Observe: observe, Elapsed: func() (time.Duration, bool) { return time.Since(start), true },
		MaxAge: time.Minute, MaxDriftPPM: 500, CounterUncertainty: time.Microsecond,
	}, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(monitor.Close)
	return monitor
}

func TestManagedPermissionClockConfigurationIsPassive(t *testing.T) {
	var calls atomic.Int64
	monitor := runtimeTestClockMonitor(t, func(context.Context) (permission.ClockReading, error) {
		calls.Add(1)
		return permission.ClockReading{}, nil
	})
	managed := WithPermissionClockMonitor(monitor)
	if _, err := defaults().apply(WithPermissionClockMonitor(nil)); err == nil {
		t.Fatal("nil monitor accepted")
	}
	for _, external := range []Option{
		WithPermissionClock(func() permission.ClockReading { return permission.ClockReading{} }),
		WithPermissionClockUncertainty(func() (time.Duration, bool) { return 0, false }),
	} {
		for _, options := range [][]Option{{managed, external}, {external, managed}} {
			config, err := defaults().apply(options...)
			if err == nil {
				err = config.validate()
			}
			if err == nil {
				t.Fatal("multiple clock owners accepted")
			}
		}
	}
	if calls.Load() != 0 || monitor.Status().Running {
		t.Fatal("option resolution started clock work")
	}
	if status := (*Runtime)(nil).PermissionClockStatus(); status.Running || status.Known {
		t.Fatal("nil runtime reported clock evidence")
	}
}

func TestRuntimeOwnsPermissionClockLifecycle(t *testing.T) {
	monitor := runtimeTestClockMonitor(t, func(context.Context) (permission.ClockReading, error) {
		return permission.ClockReading{Time: time.Now(), Uncertainty: time.Millisecond, Known: true}, nil
	})
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithPermissionClockMonitor(monitor))
	eventually(t, 5*time.Second, "managed clock must obtain evidence", func() bool { return r.PermissionClockStatus().Known })
	if !r.readPermissionClock().Known || r.Catalog() == nil {
		t.Fatal("runtime did not use its managed clock")
	}
	if allocations := testing.AllocsPerRun(100, func() { _ = r.readPermissionClock() }); allocations != 0 {
		t.Fatalf("cached runtime clock allocations=%v", allocations)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if status := monitor.Status(); status.Running || status.Known || r.readPermissionClock().Known {
		t.Fatal("runtime shutdown retained managed permission time")
	}
	if err := monitor.Start(t.Context()); err == nil {
		t.Fatal("runtime did not close its clock monitor")
	}
}

func TestManagedClockFailurePreservesRuntimeDiagnostics(t *testing.T) {
	unavailable := errors.New("time service unavailable")
	var failure atomic.Bool
	failure.Store(true)
	monitor := runtimeTestClockMonitor(t, func(context.Context) (permission.ClockReading, error) {
		if failure.Load() {
			return permission.ClockReading{}, unavailable
		}
		return permission.ClockReading{Time: time.Now(), Known: true}, nil
	})
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithPermissionClockMonitor(monitor))
	eventually(t, 5*time.Second, "failed clock observation must reach diagnostics", func() bool {
		return errors.Is(r.PermissionClockStatus().LastError, unavailable)
	})
	if r.readPermissionClock().Known || !r.Status().CatalogAvailable {
		t.Fatal("clock failure supplied time or removed catalog diagnostics")
	}
	failure.Store(false)
	eventually(t, 5*time.Second, "clock must recover without a runtime restart", func() bool {
		return r.readPermissionClock().Known && r.PermissionClockStatus().LastError == nil
	})
}

func TestFailedRuntimeOpenCancelsOwnedClock(t *testing.T) {
	entered, exited := make(chan struct{}), make(chan struct{})
	var enterOnce, exitOnce sync.Once
	monitor := runtimeTestClockMonitor(t, func(ctx context.Context) (permission.ClockReading, error) {
		enterOnce.Do(func() { close(entered) })
		<-ctx.Done()
		exitOnce.Do(func() { close(exited) })
		return permission.ClockReading{}, ctx.Err()
	})
	source := newStubSource("unavailable")
	source.errs = []error{errors.New("source unavailable")}
	source.observe = func(ctx context.Context) {
		select {
		case <-entered:
		case <-ctx.Done():
		}
	}
	r, err := Open(t.Context(), WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("starmap"),
		WithSourceURL("https://authority.example"), WithSource(source), WithSourceStartupPolicy("require_source"),
		WithAcquisitionEnabled(false), WithSourcePollInterval(0), WithPermissionClockMonitor(monitor))
	if err == nil || r != nil {
		t.Fatal("required unavailable source allowed startup")
	}
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("failed Open left an observation active")
	}
	if status := monitor.Status(); status.Running || status.Known {
		t.Fatal("failed Open retained clock authority")
	}
}

func TestClockMonitorCannotServeTwoRuntimeOwners(t *testing.T) {
	monitor := runtimeTestClockMonitor(t, func(context.Context) (permission.ClockReading, error) {
		return permission.ClockReading{Time: time.Now(), Known: true}, nil
	})
	first := openTestRuntime(t, WithCatalogSource("embedded"), WithPermissionClockMonitor(monitor))
	second, err := Open(t.Context(), WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"),
		WithAcquisitionEnabled(false), WithSourcePollInterval(0), WithPermissionClockMonitor(monitor))
	if err == nil || second != nil {
		t.Fatal("two runtimes claimed one monitor")
	}
	eventually(t, 5*time.Second, "failed second owner must preserve the first monitor", func() bool {
		return first.PermissionClockStatus().Running && first.readPermissionClock().Known
	})
}

func TestOriginUsesRuntimeOwnedClockForReceipts(t *testing.T) {
	monitor := runtimeTestClockMonitor(t, func(context.Context) (permission.ClockReading, error) {
		return permission.ClockReading{Time: time.Now(), Uncertainty: time.Millisecond, Known: true}, nil
	})
	config := originTestConfig()
	config.Clock = monitor.Read
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithAuthorityOrigin(storage.NewMemory(), config),
		WithPermissionClockMonitor(monitor))
	eventually(t, 5*time.Second, "origin clock must obtain evidence", func() bool { return r.PermissionClockStatus().Known })
	receipt, err := r.ReadPermission(t.Context())
	if err != nil || receipt.Head != r.State().AuthorityHead || receipt.ValidUntil.IsZero() {
		t.Fatalf("origin receipt=%+v error=%v", receipt, err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadPermission(t.Context()); err == nil || monitor.Read().Known {
		t.Fatal("stopped origin issued a permission receipt")
	}
}
