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
