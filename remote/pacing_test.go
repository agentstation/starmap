package remote

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// TestSubscriberOpenHonorsResponseHeaderTimeout proves the bound that applies
// to opening an event stream. OpenEventStream clears the client deadline,
// because a healthy stream stays open for hours. The response-header timeout of
// the transfer policy is then the only bound on the open. A publisher that
// accepts the connection and writes no header cannot hang a subscriber.
func TestSubscriberOpenHonorsResponseHeaderTimeout(t *testing.T) {
	t.Parallel()

	const headerBound = 200 * time.Millisecond
	hang := make(chan struct{})
	defer close(hang)
	server := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, request *http.Request) {
			select {
			case <-request.Context().Done():
			case <-hang:
			}
		},
	))
	defer server.Close()

	policy := protocol.DefaultTransferPolicy()
	policy.ResponseHeaderTimeout = headerBound
	config := Config{
		BaseURL:        server.URL + "/api/v1",
		CatalogStore:   storage.NewMemory(),
		TransferPolicy: &policy,
	}
	client, err := config.transferClient()
	if err != nil {
		t.Fatalf("transferClient: %v", err)
	}
	if client.Timeout != 0 {
		t.Fatalf("transfer client timeout = %s, want no whole-request deadline", client.Timeout)
	}
	remoteClient, err := protocol.NewClient(
		config.BaseURL,
		client,
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	started := time.Now()
	stream, err := remoteClient.OpenEventStream(ctx, "")
	if err == nil {
		_ = stream.Close()
		t.Fatal("OpenEventStream succeeded against a publisher that wrote no header")
	}
	if elapsed := time.Since(started); elapsed > 20*headerBound {
		t.Fatalf("open returned after %s, want the response-header bound", elapsed)
	}
}

// TestSubscriberReconnectDelayCapsAtFifteenMinutes proves the fleet reconnect
// bounds. The delay grows with decorrelated jitter from one second and never
// exceeds fifteen minutes. A long outage therefore causes no unbounded wait
// and no tight reconnect loop.
func TestSubscriberReconnectDelayCapsAtFifteenMinutes(t *testing.T) {
	t.Parallel()

	// A maximum draw grows the delay as fast as the policy allows, so the cap
	// is the only value that can bound it.
	state := newReconnectState(Config{
		ReconnectMinDelay: DefaultReconnectMinDelay,
		ReconnectMaxDelay: DefaultReconnectMaxDelay,
		Random:            func() float64 { return 1 },
	})
	now := time.Date(2026, time.July, 29, 20, 0, 0, 0, time.UTC)
	first, err := state.next(now)
	if err != nil {
		t.Fatalf("first delay: %v", err)
	}
	if first < fleet.MinRetryDelay {
		t.Fatalf("first delay = %s, want at least %s", first, fleet.MinRetryDelay)
	}
	capped := false
	for attempt := range 64 {
		delay, err := state.next(now)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		if delay > fleet.MaxRetryDelay {
			t.Fatalf("attempt %d delay = %s, want at most %s",
				attempt, delay, fleet.MaxRetryDelay)
		}
		if delay == fleet.MaxRetryDelay {
			capped = true
		}
	}
	if !capped {
		t.Fatal("the growing delay never reached the fifteen-minute cap")
	}
}

// TestSubscriberResetsBackoffAfterHealthyWindow proves the reset rule. A TCP
// connection and a first response header prove no liveness. Only a stream that
// stayed open for a healthy liveness window returns the delay to the minimum.
// A publisher that accepts and drops each connection therefore keeps the
// growing delay.
func TestSubscriberResetsBackoffAfterHealthyWindow(t *testing.T) {
	t.Parallel()

	const window = time.Minute
	state := newReconnectState(Config{
		ReconnectMinDelay: DefaultReconnectMinDelay,
		ReconnectMaxDelay: DefaultReconnectMaxDelay,
		HealthyWindow:     window,
		Random:            func() float64 { return 1 },
	})
	now := time.Date(2026, time.July, 29, 20, 30, 0, 0, time.UTC)
	for range 6 {
		if _, err := state.next(now); err != nil {
			t.Fatalf("grow delay: %v", err)
		}
	}
	grown := state.delay
	if grown <= fleet.MinRetryDelay {
		t.Fatalf("grown delay = %s, want growth above %s", grown, fleet.MinRetryDelay)
	}

	// An open stream alone proves no liveness.
	state.opened(now)
	if state.delay != grown {
		t.Fatalf("delay after open = %s, want the grown delay %s", state.delay, grown)
	}
	// A stream that ends inside the window proves no liveness either.
	if state.closed(now.Add(window - time.Nanosecond)) {
		t.Fatal("a short stream reset the backoff")
	}
	if state.delay != grown {
		t.Fatalf("delay after a short stream = %s, want the grown delay %s",
			state.delay, grown)
	}

	state.opened(now)
	if !state.closed(now.Add(window)) {
		t.Fatal("a healthy stream did not reset the backoff")
	}
	if state.delay != 0 {
		t.Fatalf("delay after a healthy window = %s, want a reset", state.delay)
	}
	next, err := state.next(now)
	if err != nil {
		t.Fatalf("delay after reset: %v", err)
	}
	if next > fleet.MinRetryDelay {
		t.Fatalf("delay after reset = %s, want the minimum %s", next, fleet.MinRetryDelay)
	}
}

// TestSubscriberHonorsRetryAfterNotBefore proves that a declared boundary is a
// hard floor. A refusal that names Retry-After replaces the computed backoff,
// and health publishes the boundary so an operator sees why the subscriber
// waits.
func TestSubscriberHonorsRetryAfterNotBefore(t *testing.T) {
	t.Parallel()

	const boundaryDelay = time.Hour
	state := newReconnectState(Config{
		ReconnectMinDelay: time.Millisecond,
		ReconnectMaxDelay: 2 * time.Millisecond,
		Random:            func() float64 { return 0 },
	})
	now := time.Date(2026, time.July, 29, 21, 0, 0, 0, time.UTC)
	state.refuse(now, now.Add(boundaryDelay))
	delay, err := state.next(now)
	if err != nil {
		t.Fatalf("delay after refusal: %v", err)
	}
	if delay < boundaryDelay {
		t.Fatalf("delay after refusal = %s, want at least %s", delay, boundaryDelay)
	}
	if delay > boundaryDelay+fleet.MaxNotBeforeJitter {
		t.Fatalf("delay after refusal = %s, want at most %s",
			delay, boundaryDelay+fleet.MaxNotBeforeJitter)
	}

	generation := subscriberTestGeneration(
		t,
		"generation-retry-after",
		"provider-retry-after",
		time.Date(2026, time.July, 29, 21, 0, 0, 0, time.UTC),
	)
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path[len("/api/v1"):] {
			case protocol.ManifestPath:
				writeSubscriberManifest(t, writer, generation)
			case protocol.PayloadPath(generation.Manifest.GenerationID):
				writer.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
				_, _ = writer.Write(generation.Payload)
			case protocol.EventStreamPath:
				writer.Header().Set("Retry-After", "3600")
				writer.WriteHeader(http.StatusServiceUnavailable)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	subscriber, err := New(Config{
		BaseURL:           server.URL + "/api/v1",
		HTTPClient:        server.Client(),
		CatalogStore:      storage.NewMemory(),
		ReconnectMinDelay: time.Millisecond,
		ReconnectMaxDelay: 2 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := subscriber.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForSubscriberCondition(t, func() bool {
		return !subscriber.Health().RetryNotBefore.IsZero()
	})
	health := subscriber.Health()
	if wait := time.Until(health.RetryNotBefore); wait < 30*time.Minute {
		t.Fatalf("published boundary waits %s, want the declared hour", wait)
	}
}

// TestSubscriberWaitsForCredentialChange proves that a rejected credential
// suspends the subscriber instead of stopping it. A retry with the same
// credential cannot succeed, so the subscriber spends no reconnect budget and
// resumes when the caller reports a replacement.
func TestSubscriberWaitsForCredentialChange(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-credential-change",
		"provider-credential-change",
		time.Date(2026, time.July, 29, 22, 0, 0, 0, time.UTC),
	)
	var (
		authorized   atomic.Bool
		streamOpens  atomic.Int32
		closeStream  = make(chan struct{})
		credentials  = make(chan struct{}, 1)
		streamPath   = protocol.EventStreamPath
		manifestPath = protocol.ManifestPath
	)
	defer close(closeStream)
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path[len("/api/v1"):] {
			case manifestPath:
				writeSubscriberManifest(t, writer, generation)
			case protocol.PayloadPath(generation.Manifest.GenerationID):
				writer.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
				_, _ = writer.Write(generation.Payload)
			case streamPath:
				streamOpens.Add(1)
				if !authorized.Load() {
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				writer.Header().Set("Content-Type", protocol.EventStreamMediaType)
				_, _ = fmt.Fprint(writer, ": connected\n\n")
				writer.(http.Flusher).Flush()
				select {
				case <-request.Context().Done():
				case <-closeStream:
				}
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	subscriber, err := New(Config{
		BaseURL:           server.URL + "/api/v1",
		HTTPClient:        server.Client(),
		CatalogStore:      storage.NewMemory(),
		ReconnectMinDelay: time.Millisecond,
		ReconnectMaxDelay: 2 * time.Millisecond,
		CredentialChanges: credentials,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := subscriber.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitForSubscriberCondition(t, func() bool {
		return subscriber.Health().StreamState == StreamStateWaitingForCredentials
	})
	waiting := subscriber.Health()
	if waiting.LastError == nil || waiting.LastError.Terminal {
		t.Fatalf("last error = %+v, want a nonterminal authentication failure", waiting.LastError)
	}
	if opens := streamOpens.Load(); opens != 1 {
		t.Fatalf("stream opens while waiting = %d, want no retry budget spent", opens)
	}

	authorized.Store(true)
	credentials <- struct{}{}
	waitForSubscriberCondition(t, func() bool {
		return subscriber.Health().StreamState == StreamStateStreaming
	})
	if opens := streamOpens.Load(); opens < 2 {
		t.Fatalf("stream opens after the credential change = %d, want a retry", opens)
	}
}

// TestFallbackPollingUsesStablePhase proves that fallback polling spreads
// across the interval. Every subscriber of one fleet polls at the stable phase
// of its own identity. A fifteen-minute interval therefore spreads the requests
// across the whole interval instead of a shared instant.
func TestFallbackPollingUsesStablePhase(t *testing.T) {
	t.Parallel()

	const interval = DefaultFallbackPollInterval
	now := time.Date(2026, time.July, 29, 23, 4, 17, 0, time.UTC)
	identity := fleet.Identity{
		Instance:   "instance-alpha",
		Controller: controllerPoll,
		Source:     "starmap_cascade",
	}
	phase, err := fleet.StablePhase(identity, interval)
	if err != nil {
		t.Fatalf("StablePhase: %v", err)
	}
	at, err := fallbackPollAt(identity, interval, now)
	if err != nil {
		t.Fatalf("fallbackPollAt: %v", err)
	}
	expected := now.Truncate(interval).Add(phase)
	if !expected.After(now) {
		expected = expected.Add(interval)
	}
	if !at.Equal(expected) {
		t.Fatalf("poll at %s, want the stable phase %s", at, expected)
	}
	if !at.After(now) || at.Sub(now) > interval {
		t.Fatalf("poll waits %s, want a wait inside the interval", at.Sub(now))
	}

	other := identity
	other.Instance = "instance-beta"
	otherAt, err := fallbackPollAt(other, interval, now)
	if err != nil {
		t.Fatalf("fallbackPollAt for the second instance: %v", err)
	}
	if otherAt.Equal(at) {
		t.Fatal("two instances share one poll instant, so polling never spreads")
	}

	// A subscriber outside a fleet keeps the landed immediate first poll.
	alone, err := fallbackPollAt(fleet.Identity{Controller: controllerPoll}, interval, now)
	if err != nil {
		t.Fatalf("fallbackPollAt without an instance: %v", err)
	}
	if !alone.Equal(now) {
		t.Fatalf("poll without an instance identity = %s, want %s", alone, now)
	}
}
