package fleet

import (
	"testing"
	"time"
)

// pollInterval is the public source check interval of the plan.
const pollInterval = time.Hour

func fixedRandom(value float64) Random {
	return func() float64 { return value }
}

func TestStablePhaseIsStableDistinctAndBounded(t *testing.T) {
	t.Parallel()

	base := Identity{Instance: "host-a", Controller: "source", Source: "public_github"}
	first, err := StablePhase(base, pollInterval)
	if err != nil {
		t.Fatalf("StablePhase: %v", err)
	}
	again, err := StablePhase(base, pollInterval)
	if err != nil {
		t.Fatalf("StablePhase repeat: %v", err)
	}
	if first != again {
		t.Fatalf("phase changed across calls: %s then %s", first, again)
	}
	if first < 0 || first >= pollInterval {
		t.Fatalf("phase = %s, want a value inside %s", first, pollInterval)
	}

	tests := []struct {
		name     string
		identity Identity
	}{
		{"other instance", Identity{Instance: "host-b", Controller: "source", Source: "public_github"}},
		{"other controller", Identity{Instance: "host-a", Controller: "acquisition", Source: "public_github"}},
		{"other source", Identity{Instance: "host-a", Controller: "source", Source: "starmap_upstream"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			other, err := StablePhase(test.identity, pollInterval)
			if err != nil {
				t.Fatalf("StablePhase: %v", err)
			}
			if other == first {
				t.Fatalf("phase of %s equals the base phase %s", test.name, first)
			}
		})
	}
}

func TestStablePhaseRejectsUnusableInput(t *testing.T) {
	t.Parallel()

	valid := Identity{Instance: "host-a", Controller: "source"}
	if _, err := StablePhase(valid, 0); err == nil {
		t.Fatal("StablePhase accepted a zero interval")
	}
	if _, err := StablePhase(Identity{Controller: "source"}, pollInterval); err == nil {
		t.Fatal("StablePhase accepted an empty instance identity")
	}
	if _, err := StablePhase(Identity{Instance: "host-a"}, pollInterval); err == nil {
		t.Fatal("StablePhase accepted an empty controller name")
	}
}

func TestStartupOffsetStaysInsideTheFifteenMinuteSpread(t *testing.T) {
	t.Parallel()

	if DefaultStartupSpread != 15*time.Minute {
		t.Fatalf("DefaultStartupSpread = %s, want 15m0s", DefaultStartupSpread)
	}
	identity := Identity{Instance: "host-a", Controller: "source", Source: "public_github"}
	offset, err := StartupOffset(identity, DefaultStartupSpread)
	if err != nil {
		t.Fatalf("StartupOffset: %v", err)
	}
	if offset < 0 || offset >= DefaultStartupSpread {
		t.Fatalf("offset = %s, want a value inside %s", offset, DefaultStartupSpread)
	}
	immediate, err := StartupOffset(identity, 0)
	if err != nil {
		t.Fatalf("StartupOffset with no spread: %v", err)
	}
	if immediate != 0 {
		t.Fatalf("offset without a spread = %s, want 0s", immediate)
	}
}

func TestRetryPolicyUsesDecorrelatedJitterInsideItsBudget(t *testing.T) {
	t.Parallel()

	policy := DefaultRetryPolicy()
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if policy.MinDelay != time.Second || policy.MaxDelay != 15*time.Minute {
		t.Fatalf("policy range = %s..%s, want 1s..15m0s", policy.MinDelay, policy.MaxDelay)
	}
	if policy.MaxAttempts != 3 {
		t.Fatalf("MaxAttempts = %d, want 3", policy.MaxAttempts)
	}

	first, err := policy.Next(0, fixedRandom(0.5))
	if err != nil {
		t.Fatalf("Next first: %v", err)
	}
	if first != policy.MinDelay {
		t.Fatalf("first delay = %s, want %s", first, policy.MinDelay)
	}

	// The decorrelated range of a one-second previous delay is 1s to 3s.
	middle, err := policy.Next(time.Second, fixedRandom(0.5))
	if err != nil {
		t.Fatalf("Next middle: %v", err)
	}
	if middle != 2*time.Second {
		t.Fatalf("middle delay = %s, want 2s", middle)
	}

	capped, err := policy.Next(time.Hour, fixedRandom(0.999))
	if err != nil {
		t.Fatalf("Next capped: %v", err)
	}
	if capped > policy.MaxDelay {
		t.Fatalf("capped delay = %s, want at most %s", capped, policy.MaxDelay)
	}

	for retries := 0; retries < policy.MaxAttempts; retries++ {
		if !policy.Allows(retries) {
			t.Fatalf("Allows(%d) = false, want true inside the budget", retries)
		}
	}
	if policy.Allows(policy.MaxAttempts) {
		t.Fatalf("Allows(%d) = true, want false beyond the budget", policy.MaxAttempts)
	}
}

func TestRetryPolicyRejectsUnusableSettings(t *testing.T) {
	t.Parallel()

	for _, policy := range []RetryPolicy{
		{MinDelay: 0, MaxDelay: time.Minute, MaxAttempts: 3},
		{MinDelay: time.Minute, MaxDelay: time.Second, MaxAttempts: 3},
		{MinDelay: time.Second, MaxDelay: time.Minute, MaxAttempts: 0},
	} {
		if err := policy.Validate(); err == nil {
			t.Fatalf("Validate accepted %#v", policy)
		}
		if _, err := policy.Next(time.Second, fixedRandom(0.5)); err == nil {
			t.Fatalf("Next accepted %#v", policy)
		}
	}
}

func TestNotBeforeTreatsTheBoundaryAsAHardFloor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	boundary := now.Add(10 * time.Minute)

	earliest := NotBefore(now, boundary, fixedRandom(0))
	if !earliest.Equal(boundary) {
		t.Fatalf("earliest retry = %s, want the boundary %s", earliest, boundary)
	}
	latest := NotBefore(now, boundary, fixedRandom(0.999))
	if latest.Before(boundary) {
		t.Fatalf("latest retry = %s, want no earlier than %s", latest, boundary)
	}
	if latest.After(boundary.Add(MaxNotBeforeJitter)) {
		t.Fatalf("latest retry = %s, want at most %s after the boundary", latest, MaxNotBeforeJitter)
	}

	// An expired boundary still spreads the fleet across the jitter window.
	expired := NotBefore(now, now.Add(-time.Hour), fixedRandom(0.5))
	if expired.Before(now) {
		t.Fatalf("expired boundary retry = %s, want no earlier than %s", expired, now)
	}
	if expired.After(now.Add(MaxNotBeforeJitter)) {
		t.Fatalf("expired boundary retry = %s, want inside the jitter window", expired)
	}
}
