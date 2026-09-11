package runtime

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestAuthorityClockUsesOneQualifiedSample(t *testing.T) {
	p := retainedAuthorityPermissions(t)
	var samples atomic.Int64
	clock := func() permission.ClockReading {
		samples.Add(1)
		return permission.ClockReading{Time: p.activeReceipt.IssuedAt.Add(time.Second), Known: true}
	}
	r := authorityClockRuntime(t, p, clock)
	if samples.Load() != 0 {
		t.Fatal("clock configuration sampled time")
	}
	if !r.AllowsNewAttempt() || samples.Load() != 1 {
		t.Fatal("admission did not use one qualified sample")
	}
	receipt, err := r.ReadPermission(t.Context())
	if err != nil || receipt != p.activeReceipt || samples.Load() != 2 {
		t.Fatalf("relay did not use one qualified sample: receipt=%+v error=%v samples=%d", receipt, err, samples.Load())
	}
	report := r.Status()
	if !report.PermissionValid || !report.Usable || samples.Load() != 3 {
		t.Fatalf("status did not use one qualified sample: report=%+v samples=%d", report, samples.Load())
	}
}

func TestAuthorityClockRefusesUnsafeSamples(t *testing.T) {
	p := retainedAuthorityPermissions(t)
	valid := p.activeReceipt.IssuedAt.Add(time.Second)
	for _, test := range []struct {
		name   string
		sample permission.ClockReading
	}{
		{name: "unknown", sample: permission.ClockReading{Time: valid}},
		{name: "zero time", sample: permission.ClockReading{Known: true}},
		{name: "negative uncertainty", sample: permission.ClockReading{Time: valid, Uncertainty: -time.Second, Known: true}},
		{name: "excessive uncertainty", sample: permission.ClockReading{Time: valid, Uncertainty: time.Minute, Known: true}},
		{name: "expired", sample: permission.ClockReading{Time: p.activeReceipt.ValidUntil, Known: true}},
		{name: "uncertainty consumes lifetime", sample: permission.ClockReading{Time: p.activeReceipt.ValidUntil.Add(-time.Second), Uncertainty: time.Second, Known: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := authorityClockRuntime(t, p, func() permission.ClockReading { return test.sample })
			if r.AllowsNewAttempt() || r.Status().PermissionValid || r.Status().Usable {
				t.Fatal("unsafe clock sample authorized inference")
			}
			if receipt, err := r.ReadPermission(t.Context()); err == nil || receipt.Version != 0 {
				t.Fatalf("unsafe clock sample returned a receipt: %+v %v", receipt, err)
			}
			if r.Catalog() == nil || !r.Status().CatalogAvailable {
				t.Fatal("unsafe clock sample removed diagnostic metadata")
			}
		})
	}
}

func TestAuthorityClockConfigurationIsExplicitAndPassive(t *testing.T) {
	if err := WithPermissionClock(nil)(defaults()); !errors.IsValidationError(err) {
		t.Fatalf("nil clock error = %v", err)
	}
	var samples atomic.Int64
	clock := WithPermissionClock(func() permission.ClockReading { samples.Add(1); return permission.ClockReading{} })
	legacy := WithPermissionClockUncertainty(func() (time.Duration, bool) { samples.Add(1); return 0, true })
	for _, opts := range [][]Option{{clock, legacy}, {legacy, clock}} {
		config, err := defaults().apply(opts...)
		if err == nil {
			err = config.validate()
		}
		if !errors.IsValidationError(err) || samples.Load() != 0 {
			t.Fatalf("mixed clock contracts: error=%v samples=%d", err, samples.Load())
		}
	}
}

func TestAuthorityClockAdmissionAllocatesNoMemory(t *testing.T) {
	p := retainedAuthorityPermissions(t)
	r := authorityClockRuntime(t, p, func() permission.ClockReading {
		return permission.ClockReading{Time: p.activeReceipt.IssuedAt.Add(time.Second), Known: true}
	})
	var allowed bool
	allocations := testing.AllocsPerRun(100, func() { allowed = r.AllowsNewAttempt() })
	if !allowed || allocations != 0 {
		t.Fatalf("admission allowed=%v allocations=%v", allowed, allocations)
	}
}

func TestAuthorityClockConcurrentSamplesDoNotMixAssurance(t *testing.T) {
	p := retainedAuthorityPermissions(t)
	var samples atomic.Int64
	r := authorityClockRuntime(t, p, func() permission.ClockReading {
		if samples.Add(1)%2 == 0 {
			return permission.ClockReading{Time: p.activeReceipt.IssuedAt.Add(time.Second)}
		}
		return permission.ClockReading{Time: p.activeReceipt.ValidUntil, Known: true}
	})
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 100 {
				if r.AllowsNewAttempt() || r.Status().PermissionValid {
					t.Error("separate unsafe samples authorized inference")
				}
				if receipt, err := r.ReadPermission(t.Context()); err == nil || receipt.Version != 0 {
					t.Error("separate unsafe samples produced a receipt")
				}
			}
		})
	}
	workers.Wait()
}

func authorityClockRuntime(t *testing.T, p authorityPermissions, clock func() permission.ClockReading) *Runtime {
	t.Helper()
	config := defaults()
	config.source.StartupPolicy = StartupRequireAuthority
	config.now = func() time.Time { return p.activeReceipt.ValidUntil.Add(time.Hour) }
	if err := WithPermissionClock(clock)(config); err != nil {
		t.Fatal(err)
	}
	return &Runtime{
		ctx: t.Context(), config: *config, permissions: p,
		effective: starmap.CatalogState{Catalog: &catalogs.Catalog{}},
		lease:     newLeaseKeeper(nil, "clock-test", config.now),
	}
}
