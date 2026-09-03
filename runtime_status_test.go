package starmap

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"
)

// testClock is an injected clock. Tests move time forward instead of sleeping,
// so a freshness assertion needs no real delay.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now returns the current test time.
func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// advance moves the test time forward.
func (c *testClock) advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(delta)
}

// TestRuntimeStatusKeepsUsabilityFreshnessFallbackAndHealthIndependent proves
// that status reports five separate judgments. A stale catalog never hides a
// working transfer, a working transfer never hides a degraded upstream, and a
// failed source never reports an unusable runtime.
func TestRuntimeStatusKeepsUsabilityFreshnessFallbackAndHealthIndependent(t *testing.T) {
	t.Parallel()

	t.Run("a failed source keeps the runtime usable", func(t *testing.T) {
		t.Parallel()
		clock := &testClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
		source := newStubSource("unreachable-source")
		source.errs = []error{stderrors.New("the source refused the connection")}
		runtime := openTestRuntime(t, WithSource(source), WithClock(clock.Now))

		// Before any check the runtime already serves the embedded catalog.
		initial := runtime.Status()
		if !initial.Usable {
			t.Fatal("the runtime must serve the embedded catalog before the first check")
		}
		if !initial.Fallback || initial.FallbackReason != FallbackAwaitingSource {
			t.Errorf("fallback = %v/%q, want true/%q",
				initial.Fallback, initial.FallbackReason, FallbackAwaitingSource)
		}
		if initial.SourceCheckFreshness != FreshnessUnknown {
			t.Errorf("source-check freshness = %q, want %q",
				initial.SourceCheckFreshness, FreshnessUnknown)
		}

		if _, err := runtime.RefreshSource(context.Background()); err == nil {
			t.Fatal("RefreshSource succeeded against a refusing source")
		}
		status := runtime.Status()
		if !status.Usable {
			t.Error("a failed source must not make the runtime unusable")
		}
		if !status.Fallback || status.FallbackReason != FallbackSourceUnavailable {
			t.Errorf("fallback = %v/%q, want true/%q",
				status.Fallback, status.FallbackReason, FallbackSourceUnavailable)
		}
		if status.SourceHealth != HealthUnavailable {
			t.Errorf("source health = %q, want %q", status.SourceHealth, HealthUnavailable)
		}
		if status.UpstreamHealth != HealthUnknown {
			t.Errorf("upstream health = %q, want %q", status.UpstreamHealth, HealthUnknown)
		}
		if status.SourceReason == "" {
			t.Error("a failed source must report a safe reason code")
		}
		if status.SourceCheckFreshness != FreshnessCurrent {
			t.Errorf("source-check freshness = %q, want %q",
				status.SourceCheckFreshness, FreshnessCurrent)
		}
	})

	t.Run("a healthy transfer reports a degraded upstream", func(t *testing.T) {
		t.Parallel()
		start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		clock := &testClock{now: start}
		source := newStubSource("chain-source")
		payload := testCatalogPayload(t, "status-provider", "status-model", "Status Model")

		// The upstream publishes an old generation and reports itself degraded.
		published := start.Add(-12 * time.Hour)
		read := testSourceRead(t, "generation-status", payload, published)
		read.Health = HealthDegraded
		read.Chain = []SourceHop{{
			Identity:    "upstream-runtime",
			Health:      HealthDegraded,
			PublishedAt: published,
			ObservedAt:  start,
		}}
		source.replies = []SourceRead{read}

		runtime := openTestRuntime(t, WithSource(source), WithClock(clock.Now))
		if _, err := runtime.RefreshSource(context.Background()); err != nil {
			t.Fatalf("RefreshSource: %v", err)
		}
		status := runtime.Status()

		if !status.Usable {
			t.Error("a published generation must leave the runtime usable")
		}
		if status.Fallback {
			t.Errorf("fallback = true/%q, want false", status.FallbackReason)
		}
		if status.SourceHealth != HealthOK {
			t.Errorf("source health = %q, want %q", status.SourceHealth, HealthOK)
		}
		if status.UpstreamHealth != HealthDegraded {
			t.Errorf("upstream health = %q, want %q", status.UpstreamHealth, HealthDegraded)
		}
		if status.Freshness != FreshnessCritical {
			t.Errorf("catalog freshness = %q, want %q", status.Freshness, FreshnessCritical)
		}
		if status.SourceCheckFreshness != FreshnessCurrent {
			t.Errorf("source-check freshness = %q, want %q",
				status.SourceCheckFreshness, FreshnessCurrent)
		}
		if len(status.Chain) != 1 || status.Chain[0].Identity != "upstream-runtime" {
			t.Errorf("chain = %+v, want one sanitized hop", status.Chain)
		}
		if status.SourceIdentity != "chain-source" {
			t.Errorf("source identity = %q, want %q", status.SourceIdentity, "chain-source")
		}
		if status.Lease != string(leaseNotRequired) {
			t.Errorf("lease = %q, want %q", status.Lease, leaseNotRequired)
		}

		// Time alone moves freshness. Health and fallback stay where they were.
		clock.advance(100 * time.Minute)
		later := runtime.Status()
		if later.SourceCheckFreshness != FreshnessWarn {
			t.Errorf("source-check freshness = %q, want %q",
				later.SourceCheckFreshness, FreshnessWarn)
		}
		if later.SourceHealth != HealthOK || later.UpstreamHealth != HealthDegraded {
			t.Errorf("health moved with the clock: %q and %q",
				later.SourceHealth, later.UpstreamHealth)
		}
		if later.Fallback {
			t.Error("fallback moved with the clock")
		}
		if !later.Usable {
			t.Error("usability moved with the clock")
		}
	})
}

// TestSourceMaxAgeSetsTheChannelThresholds proves that the source maximum age
// derives the channel freshness thresholds. An operator who widens the maximum
// age widens the grade with it, and an explicit freshness policy still wins.
func TestSourceMaxAgeSetsTheChannelThresholds(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	const week = 7 * day

	// gradeChannel opens a runtime whose upstream published one generation the
	// given age ago, then returns the status that the runtime reports.
	gradeChannel := func(t *testing.T, age time.Duration, opts ...Option) RuntimeStatus {
		t.Helper()
		start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		clock := &testClock{now: start}
		source := newStubSource("max-age-source")
		payload := testCatalogPayload(t, "status-provider", "status-model", "Status Model")
		source.replies = []SourceRead{
			testSourceRead(t, "generation-max-age", payload, start.Add(-age)),
		}
		base := []Option{WithSource(source), WithClock(clock.Now)}
		runtime := openTestRuntime(t, append(base, opts...)...)
		if _, err := runtime.RefreshSource(context.Background()); err != nil {
			t.Fatalf("RefreshSource: %v", err)
		}
		return runtime.Status()
	}

	t.Run("a seven-day maximum age grades a two-day channel current", func(t *testing.T) {
		t.Parallel()
		status := gradeChannel(t, 2*day, WithSourceMaxAge(week))
		if status.ChannelFreshness != FreshnessCurrent {
			t.Errorf("channel freshness = %q, want %q",
				status.ChannelFreshness, FreshnessCurrent)
		}
		if status.Freshness != FreshnessCurrent {
			t.Errorf("catalog freshness = %q, want %q", status.Freshness, FreshnessCurrent)
		}
	})

	t.Run("a channel older than the maximum age warns", func(t *testing.T) {
		t.Parallel()
		status := gradeChannel(t, 8*day, WithSourceMaxAge(week))
		if status.ChannelFreshness != FreshnessWarn {
			t.Errorf("channel freshness = %q, want %q", status.ChannelFreshness, FreshnessWarn)
		}
	})

	t.Run("a channel older than the scaled critical age is critical", func(t *testing.T) {
		t.Parallel()
		// The defaults hold six hours against ten, so a seven-day maximum age
		// turns critical at 280 hours.
		status := gradeChannel(t, 281*time.Hour, WithSourceMaxAge(week))
		if status.ChannelFreshness != FreshnessCritical {
			t.Errorf("channel freshness = %q, want %q",
				status.ChannelFreshness, FreshnessCritical)
		}
	})

	t.Run("a zero maximum age keeps the defaults", func(t *testing.T) {
		t.Parallel()
		current := gradeChannel(t, 5*time.Hour, WithSourceMaxAge(0))
		if current.ChannelFreshness != FreshnessCurrent {
			t.Errorf("five-hour channel freshness = %q, want %q",
				current.ChannelFreshness, FreshnessCurrent)
		}
		warn := gradeChannel(t, 7*time.Hour, WithSourceMaxAge(0))
		if warn.ChannelFreshness != FreshnessWarn {
			t.Errorf("seven-hour channel freshness = %q, want %q",
				warn.ChannelFreshness, FreshnessWarn)
		}
		critical := gradeChannel(t, 11*time.Hour, WithSourceMaxAge(0))
		if critical.ChannelFreshness != FreshnessCritical {
			t.Errorf("eleven-hour channel freshness = %q, want %q",
				critical.ChannelFreshness, FreshnessCritical)
		}
	})

	t.Run("the default maximum age reproduces the defaults", func(t *testing.T) {
		t.Parallel()
		// The ratio constants must keep naming the default pair, so the
		// default runtime grades exactly as it graded before.
		defaults := DefaultFreshnessPolicy()
		if derived := defaults.withChannelMaxAge(DefaultSourceMaxAge); derived != defaults {
			t.Errorf("derived policy = %+v, want %+v", derived, defaults)
		}
	})

	t.Run("an explicit freshness policy wins", func(t *testing.T) {
		t.Parallel()
		policy := DefaultFreshnessPolicy()
		policy.ChannelWarnAge = time.Hour
		policy.ChannelCriticalAge = 2 * time.Hour

		// Neither option order changes the outcome. The runtime resolves the
		// derivation once, after it applies every option.
		orders := map[string][]Option{
			"the maximum age first": {WithSourceMaxAge(week), WithFreshnessPolicy(policy)},
			"the policy first":      {WithFreshnessPolicy(policy), WithSourceMaxAge(week)},
		}
		for name, opts := range orders {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				status := gradeChannel(t, 2*day, opts...)
				if status.ChannelFreshness != FreshnessCritical {
					t.Errorf("channel freshness = %q, want %q",
						status.ChannelFreshness, FreshnessCritical)
				}
			})
		}
	})
}
