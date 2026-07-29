package remote

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestNewStartsNoRemoteRequest(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			requests.Add(1)
		},
	))
	defer server.Close()
	subscriber, err := New(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("constructor remote requests = %d, want 0", got)
	}
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close idle subscriber: %v", err)
	}
}

func TestConfigDefaultsAndLivenessMargin(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	subscriber, err := New(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New defaults: %v", err)
	}
	if subscriber.config.ExpectedHeartbeatInterval !=
		DefaultExpectedHeartbeatInterval ||
		subscriber.config.LivenessTimeout != DefaultLivenessTimeout ||
		subscriber.config.ShutdownTimeout != DefaultShutdownTimeout {
		t.Fatalf("normalized lifecycle config = %#v", subscriber.config)
	}
	policy := &PollingFallbackPolicy{
		AfterFailures: 2,
		Interval:      time.Second,
	}
	withFallback, err := New(Config{
		BaseURL:         server.URL,
		PollingFallback: policy,
	})
	if err != nil {
		t.Fatalf("New polling fallback: %v", err)
	}
	policy.AfterFailures = 99
	if withFallback.config.PollingFallback == policy ||
		withFallback.config.PollingFallback.AfterFailures != 2 {
		t.Fatalf(
			"subscriber retained mutable fallback config: %#v",
			withFallback.config.PollingFallback,
		)
	}

	for _, test := range []struct {
		name   string
		config Config
	}{
		{
			name: "negative heartbeat",
			config: Config{
				BaseURL: server.URL, ExpectedHeartbeatInterval: -time.Second,
			},
		},
		{
			name: "insufficient liveness margin",
			config: Config{
				BaseURL: server.URL, ExpectedHeartbeatInterval: time.Second,
				LivenessTimeout: time.Second,
			},
		},
		{
			name: "negative liveness",
			config: Config{
				BaseURL: server.URL, LivenessTimeout: -time.Second,
			},
		},
		{
			name: "negative shutdown",
			config: Config{
				BaseURL: server.URL, ShutdownTimeout: -time.Second,
			},
		},
		{
			name: "zero fallback threshold",
			config: Config{
				BaseURL: server.URL,
				PollingFallback: &PollingFallbackPolicy{
					Interval: time.Second,
				},
			},
		},
		{
			name: "zero fallback interval",
			config: Config{
				BaseURL: server.URL,
				PollingFallback: &PollingFallbackPolicy{
					AfterFailures: 1,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got, err := New(test.config); err == nil {
				_ = got.Close()
				t.Fatal("New accepted invalid lifecycle config")
			}
		})
	}
}

func TestExponentialJitterStaysWithinBoundedSchedule(t *testing.T) {
	t.Parallel()

	subscriber := &Subscriber{config: Config{
		ReconnectMinDelay: 100 * time.Millisecond,
		ReconnectMaxDelay: 800 * time.Millisecond,
	}}
	for attempt, ceiling := range []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		800 * time.Millisecond,
		800 * time.Millisecond,
	} {
		for range 100 {
			delay := subscriber.exponentialJitter(attempt)
			if delay < ceiling/2 || delay > ceiling {
				t.Fatalf(
					"attempt %d delay = %s, want [%s, %s]",
					attempt,
					delay,
					ceiling/2,
					ceiling,
				)
			}
		}
	}
	if delay := subscriber.exponentialJitter(1_000_000); delay < 400*time.Millisecond ||
		delay > 800*time.Millisecond {
		t.Fatalf("large-attempt delay = %s, want bounded max jitter", delay)
	}
}

func TestSubscriberMissingHeartbeatReconnectsAndCatchesUp(t *testing.T) {
	t.Parallel()

	first := subscriberTestGeneration(
		t,
		"generation-liveness-first",
		"provider-liveness-first",
		time.Date(2026, time.July, 29, 16, 0, 0, 0, time.UTC),
	)
	second := subscriberTestGeneration(
		t,
		"generation-liveness-second",
		"provider-liveness-second",
		time.Date(2026, time.July, 29, 16, 1, 0, 0, time.UTC),
	)
	var (
		mu          sync.RWMutex
		current     = first
		streamCount atomic.Int32
	)
	generations := map[string]catalogstore.Generation{
		first.Manifest.GenerationID:  first,
		second.Manifest.GenerationID: second,
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			resourcePath := request.URL.Path[len("/api/v1"):]
			mu.RLock()
			selected := current
			mu.RUnlock()
			switch resourcePath {
			case catalogremote.ManifestPath:
				writeSubscriberManifest(t, writer, selected)
				return
			case catalogremote.EventStreamPath:
				streamCount.Add(1)
				writer.Header().Set(
					"Content-Type",
					catalogremote.EventStreamMediaType,
				)
				_, _ = fmt.Fprint(writer, ": connected\n\n")
				writer.(http.Flusher).Flush()
				<-request.Context().Done()
				return
			}
			for id, generation := range generations {
				if resourcePath == catalogremote.SnapshotPath(id) {
					writer.Header().Set(
						"Content-Type",
						catalogs.CatalogPayloadMediaType,
					)
					_, _ = writer.Write(generation.Payload)
					return
				}
			}
			http.NotFound(writer, request)
		},
	))
	defer server.Close()

	subscriber, err := New(Config{
		BaseURL:                   server.URL + "/api/v1",
		HTTPClient:                server.Client(),
		ReconnectMinDelay:         time.Millisecond,
		ReconnectMaxDelay:         time.Millisecond,
		ExpectedHeartbeatInterval: 10 * time.Millisecond,
		LivenessTimeout:           25 * time.Millisecond,
		ShutdownTimeout:           time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	subscriber.retryDelay = func(int) time.Duration { return 0 }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	mu.Lock()
	current = second
	mu.Unlock()

	waitForSubscriberCondition(t, func() bool {
		if streamCount.Load() < 2 {
			return false
		}
		_, err := subscriber.Catalog().Provider("provider-liveness-second")
		return err == nil
	})
	started := time.Now()
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("Close took %s, want bounded join below 1s", elapsed)
	}
}

func TestSubscriberHeartbeatsPreserveStreamLiveness(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-heartbeat",
		"provider-heartbeat",
		time.Date(2026, time.July, 29, 17, 0, 0, 0, time.UTC),
	)
	var (
		streamCount             atomic.Int32
		conditionalManifestGets atomic.Int32
	)
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			resourcePath := request.URL.Path[len("/api/v1"):]
			switch resourcePath {
			case catalogremote.ManifestPath:
				if request.Header.Get("If-None-Match") != "" {
					conditionalManifestGets.Add(1)
				}
				writeSubscriberManifest(t, writer, generation)
			case catalogremote.SnapshotPath(generation.Manifest.GenerationID):
				writer.Header().Set(
					"Content-Type",
					catalogs.CatalogPayloadMediaType,
				)
				_, _ = writer.Write(generation.Payload)
			case catalogremote.EventStreamPath:
				streamCount.Add(1)
				writer.Header().Set(
					"Content-Type",
					catalogremote.EventStreamMediaType,
				)
				_, _ = fmt.Fprint(writer, ": connected\n\n")
				writer.(http.Flusher).Flush()
				ticker := time.NewTicker(5 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-request.Context().Done():
						return
					case <-ticker.C:
						_, _ = fmt.Fprint(writer, ": heartbeat\n\n")
						writer.(http.Flusher).Flush()
					}
				}
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	subscriber, err := New(Config{
		BaseURL:                   server.URL + "/api/v1",
		HTTPClient:                server.Client(),
		ExpectedHeartbeatInterval: 5 * time.Millisecond,
		LivenessTimeout:           100 * time.Millisecond,
		ShutdownTimeout:           time.Second,
		PollingFallback: &PollingFallbackPolicy{
			AfterFailures: 1,
			Interval:      time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	if got := streamCount.Load(); got != 1 {
		t.Fatalf("healthy heartbeat stream connections = %d, want 1", got)
	}
	if got := conditionalManifestGets.Load(); got != 0 {
		t.Fatalf("healthy stream conditional polls = %d, want 0", got)
	}
	if status := subscriber.PollingFallbackStatus(); status.Active ||
		status.Polls != 0 || status.Entries != 0 {
		t.Fatalf("healthy stream fallback status = %#v", status)
	}
	generatedAt := assertHeartbeatStreamHealth(t, subscriber, generation)
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	assertStoppedCatalogHealth(t, subscriber, generatedAt)
}

func TestSubscriberPollingFallbackIsExplicitBoundedAndConditional(t *testing.T) {
	t.Parallel()

	first := subscriberTestGeneration(
		t,
		"generation-poll-first",
		"provider-poll-first",
		time.Date(2026, time.July, 29, 17, 30, 0, 0, time.UTC),
	)
	second := subscriberTestGeneration(
		t,
		"generation-poll-second",
		"provider-poll-second",
		first.Manifest.GeneratedAt.Add(time.Minute),
	)
	var (
		mu              sync.RWMutex
		current         = first
		allowStream     atomic.Bool
		streamRequests  atomic.Int32
		conditionalGets atomic.Int32
		snapshotGets    atomic.Int32
		pollTimes       []time.Time
		healthyStream   = make(chan struct{}, 1)
	)
	generations := map[string]catalogstore.Generation{
		first.Manifest.GenerationID:  first,
		second.Manifest.GenerationID: second,
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			resourcePath := request.URL.Path[len("/api/v1"):]
			switch resourcePath {
			case catalogremote.ManifestPath:
				mu.RLock()
				selected := current
				mu.RUnlock()
				etag := catalogremote.ManifestETag(selected.Manifest.GenerationID)
				writer.Header().Set("ETag", etag)
				ifNoneMatch := request.Header.Get("If-None-Match")
				if ifNoneMatch == etag {
					mu.Lock()
					pollTimes = append(pollTimes, time.Now())
					mu.Unlock()
					if conditionalGets.Add(1) == 1 {
						mu.Lock()
						current = second
						mu.Unlock()
					}
					writer.WriteHeader(http.StatusNotModified)
					return
				}
				if ifNoneMatch != "" {
					mu.Lock()
					pollTimes = append(pollTimes, time.Now())
					mu.Unlock()
					conditionalGets.Add(1)
				}
				writeSubscriberManifest(t, writer, selected)
				return
			case catalogremote.EventStreamPath:
				streamRequests.Add(1)
				if !allowStream.Load() {
					writer.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				writer.Header().Set(
					"Content-Type",
					catalogremote.EventStreamMediaType,
				)
				_, _ = fmt.Fprint(writer, ": connected\n\n")
				writer.(http.Flusher).Flush()
				select {
				case healthyStream <- struct{}{}:
				default:
				}
				ticker := time.NewTicker(time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-request.Context().Done():
						return
					case <-ticker.C:
						_, _ = fmt.Fprint(writer, ": heartbeat\n\n")
						writer.(http.Flusher).Flush()
					}
				}
			}
			for id, generation := range generations {
				if resourcePath == catalogremote.SnapshotPath(id) {
					snapshotGets.Add(1)
					writer.Header().Set(
						"Content-Type",
						catalogs.CatalogPayloadMediaType,
					)
					_, _ = writer.Write(generation.Payload)
					return
				}
			}
			http.NotFound(writer, request)
		},
	))
	defer server.Close()

	subscriber, err := New(Config{
		BaseURL:                   server.URL + "/api/v1",
		HTTPClient:                server.Client(),
		ReconnectMinDelay:         time.Millisecond,
		ReconnectMaxDelay:         time.Millisecond,
		ExpectedHeartbeatInterval: time.Millisecond,
		LivenessTimeout:           3 * time.Millisecond,
		ShutdownTimeout:           time.Second,
		PollingFallback: &PollingFallbackPolicy{
			AfterFailures: 2,
			Interval:      25 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start with configured fallback: %v", err)
	}
	defer func() { _ = subscriber.Close() }()

	waitForSubscriberCondition(t, func() bool {
		status := subscriber.PollingFallbackStatus()
		if !status.Active || status.Entries != 1 || status.Polls != 2 ||
			status.Modified != 1 {
			return false
		}
		_, err := subscriber.Catalog().Provider("provider-poll-second")
		return err == nil
	})
	if got := conditionalGets.Load(); got != 2 {
		t.Fatalf("conditional manifest GETs = %d, want bounded 2", got)
	}
	if got := snapshotGets.Load(); got != 2 {
		t.Fatalf(
			"snapshot GETs = %d, want initial plus changed fallback only",
			got,
		)
	}
	mu.RLock()
	gotPollTimes := append([]time.Time(nil), pollTimes...)
	mu.RUnlock()
	if len(gotPollTimes) != 2 ||
		gotPollTimes[1].Sub(gotPollTimes[0]) < 25*time.Millisecond {
		t.Fatalf("poll cadence = %v, want two requests at least 25ms apart", gotPollTimes)
	}

	allowStream.Store(true)
	select {
	case <-healthyStream:
	case <-ctx.Done():
		t.Fatal("stream did not recover")
	}
	waitForSubscriberCondition(t, func() bool {
		status := subscriber.PollingFallbackStatus()
		return !status.Active
	})
	pollsAtRecovery := conditionalGets.Load()
	time.Sleep(10 * time.Millisecond)
	if got := conditionalGets.Load(); got != pollsAtRecovery {
		t.Fatalf(
			"healthy recovered stream triggered polling: %d -> %d",
			pollsAtRecovery,
			got,
		)
	}
	if got := streamRequests.Load(); got < 2 {
		t.Fatalf("stream requests = %d, want failed and recovered attempts", got)
	}
}

func TestCloseReportsBoundedJoinTimeout(t *testing.T) {
	t.Parallel()

	subscriber := &Subscriber{
		config: Config{ShutdownTimeout: 5 * time.Millisecond},
		state:  stateRunning,
		done:   make(chan struct{}),
	}
	err := subscriber.Close()
	if !stderrors.Is(err, pkgerrors.ErrTimeout) {
		t.Fatalf("Close error = %v, want typed timeout", err)
	}
}

func TestCloseCancelsAndJoinsInitialFetch(t *testing.T) {
	t.Parallel()

	fetchStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, request *http.Request) {
			close(fetchStarted)
			<-request.Context().Done()
		},
	))
	defer server.Close()
	subscriber, err := New(Config{
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
		ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	startResult := make(chan error, 1)
	go func() {
		startResult <- subscriber.Start(context.Background())
	}()

	select {
	case <-fetchStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("initial fetch did not start")
	}
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case err := <-startResult:
		if err == nil {
			t.Fatal("Start succeeded after Close canceled initialization")
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after Close")
	}
}

func TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay(t *testing.T) {
	t.Parallel()

	first := subscriberTestGeneration(
		t,
		"generation-first",
		"provider-first",
		time.Date(2026, time.July, 29, 14, 0, 0, 0, time.UTC),
	)
	second := subscriberTestGeneration(
		t,
		"generation-second",
		"provider-second",
		time.Date(2026, time.July, 29, 14, 1, 0, 0, time.UTC),
	)

	var (
		mu              sync.RWMutex
		current         = first
		requestCounts   = make(map[string]int)
		streamCount     atomic.Int32
		firstFrames     = make(chan string, 1)
		closeFirst      = make(chan struct{})
		secondConnected = make(chan string, 1)
		reconnectIDs    []string
		retryAttempts   []int
	)
	generations := map[string]catalogstore.Generation{
		first.Manifest.GenerationID:  first,
		second.Manifest.GenerationID: second,
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			resourcePath := request.URL.Path[len("/api/v1"):]
			mu.Lock()
			requestCounts[resourcePath]++
			selected := current
			mu.Unlock()

			switch resourcePath {
			case catalogremote.ManifestPath:
				writeSubscriberManifest(t, writer, selected)
				return
			case catalogremote.EventStreamPath:
				connection := streamCount.Add(1)
				if connection > 1 {
					mu.Lock()
					reconnectIDs = append(
						reconnectIDs,
						request.Header.Get("Last-Event-ID"),
					)
					mu.Unlock()
				}
				if connection == 2 || connection == 3 {
					writer.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				writer.Header().Set(
					"Content-Type",
					catalogremote.EventStreamMediaType,
				)
				_, _ = fmt.Fprint(writer, ": connected\n\n")
				writer.(http.Flusher).Flush()
				if connection == 1 {
					for {
						select {
						case frame := <-firstFrames:
							_, _ = fmt.Fprint(writer, frame)
							writer.(http.Flusher).Flush()
						case <-closeFirst:
							return
						case <-request.Context().Done():
							return
						}
					}
				}
				select {
				case secondConnected <- request.Header.Get("Last-Event-ID"):
				default:
				}
				<-request.Context().Done()
				return
			}

			for id, generation := range generations {
				switch resourcePath {
				case catalogremote.GenerationManifestPath(id):
					writeSubscriberManifest(t, writer, generation)
					return
				case catalogremote.SnapshotPath(id):
					writer.Header().Set(
						"Content-Type",
						catalogs.CatalogPayloadMediaType,
					)
					_, _ = writer.Write(generation.Payload)
					return
				}
			}
			http.NotFound(writer, request)
		},
	))
	defer server.Close()

	subscriber, err := New(Config{
		BaseURL:           server.URL + "/api/v1",
		HTTPClient:        server.Client(),
		ReconnectMinDelay: time.Millisecond,
		ReconnectMaxDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	subscriber.retryDelay = func(attempt int) time.Duration {
		mu.Lock()
		retryAttempts = append(retryAttempts, attempt)
		mu.Unlock()
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = subscriber.Close() }()
	if _, err := subscriber.Catalog().Provider("provider-first"); err != nil {
		t.Fatalf("initial remote catalog not activated: %v", err)
	}

	initialCatalog := subscriber.Catalog()
	firstFrames <- "id: 1\n" +
		"event: catalog.published\n" +
		"data: {\"generation_id\":\"generation-first\",\"sequence\":1}\n\n"
	waitForSubscriberCondition(t, func() bool {
		return subscriber.currentLastEventID() == "1"
	})
	if subscriber.Catalog() != initialCatalog {
		t.Fatal("duplicate generation event republished the immutable catalog")
	}
	mu.RLock()
	addressedDuplicateGets :=
		requestCounts[catalogremote.GenerationManifestPath(
			first.Manifest.GenerationID,
		)]
	mu.RUnlock()
	if addressedDuplicateGets != 0 {
		t.Fatalf(
			"duplicate generation caused %d addressed manifest requests",
			addressedDuplicateGets,
		)
	}

	mu.Lock()
	current = second
	mu.Unlock()
	close(closeFirst)

	select {
	case lastEventID := <-secondConnected:
		if lastEventID != "1" {
			t.Fatalf("reconnect Last-Event-ID = %q, want 1", lastEventID)
		}
	case <-ctx.Done():
		t.Fatal("subscriber did not reconnect")
	}
	waitForSubscriberCondition(t, func() bool {
		_, err := subscriber.Catalog().Provider("provider-second")
		return err == nil
	})
	if _, err := subscriber.Catalog().Provider("provider-first"); err == nil {
		t.Fatal("catch-up exposed a partial old/new catalog")
	}
	mu.RLock()
	gotRetryAttempts := append([]int(nil), retryAttempts...)
	gotReconnectIDs := append([]string(nil), reconnectIDs...)
	currentManifestGets := requestCounts[catalogremote.ManifestPath]
	mu.RUnlock()
	if !slices.Equal(gotRetryAttempts, []int{0, 1, 2}) {
		t.Fatalf(
			"reconnect retry attempts = %v, want exponential attempts [0 1 2]",
			gotRetryAttempts,
		)
	}
	if !slices.Equal(gotReconnectIDs, []string{"1", "1", "1"}) {
		t.Fatalf(
			"reconnect Last-Event-ID values = %v, want [1 1 1]",
			gotReconnectIDs,
		)
	}
	if currentManifestGets != 3 {
		t.Fatalf(
			"current manifest GETs = %d, want initial, post-subscribe, and reconnect catch-up",
			currentManifestGets,
		)
	}
	assertRecoveredStreamHealth(t, subscriber, second, 3)
}

func TestSubscriberDeduplicatesNewIdentityWithSamePayload(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-one",
		"provider-one",
		time.Date(2026, time.July, 29, 15, 0, 0, 0, time.UTC),
	)
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	subscriber, err := New(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if published, err := subscriber.activate(context.Background(), generation); err != nil ||
		!published {
		t.Fatalf("activate initial = %t/%v", published, err)
	}
	initialCatalog := subscriber.Catalog()

	duplicate := generation.Copy()
	duplicate.Manifest.GenerationID = "generation-two"
	duplicate.Manifest.GeneratedAt = generation.Manifest.GeneratedAt.Add(time.Minute)
	if err := duplicate.Validate(); err != nil {
		t.Fatalf("duplicate fixture: %v", err)
	}
	if published, err := subscriber.activate(context.Background(), duplicate); err != nil ||
		published {
		t.Fatalf("activate duplicate payload = %t/%v", published, err)
	}
	if subscriber.Catalog() != initialCatalog {
		t.Fatal("identical payload under a newer identity copied the catalog")
	}
	if !subscriber.isActiveGeneration(duplicate.Manifest.GenerationID) {
		t.Fatal("deduplication state did not advance to the newer identity")
	}
}

func TestSubscriberRejectsStaleAndInvalidGenerationsBeforeActivation(t *testing.T) {
	t.Parallel()

	active := subscriberTestGeneration(
		t,
		"generation-active",
		"provider-active",
		time.Date(2026, time.July, 29, 18, 0, 0, 0, time.UTC),
	)
	stale := subscriberTestGeneration(
		t,
		"generation-stale",
		"provider-stale",
		active.Manifest.GeneratedAt.Add(-time.Minute),
	)
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	subscriber, err := New(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if published, err := subscriber.activate(
		context.Background(),
		active,
	); err != nil || !published {
		t.Fatalf("activate current = %t/%v", published, err)
	}
	activeCatalog := subscriber.Catalog()
	activeIdentity := subscriber.active

	if published, err := subscriber.activate(
		context.Background(),
		stale,
	); err != nil || published {
		t.Fatalf("activate stale = %t/%v", published, err)
	}
	if subscriber.Catalog() != activeCatalog ||
		subscriber.active != activeIdentity {
		t.Fatal("stale generation changed active catalog or identity")
	}
	if _, err := subscriber.Catalog().Provider("provider-stale"); err == nil {
		t.Fatal("stale provider became visible")
	}

	corrupt := subscriberTestGeneration(
		t,
		"generation-corrupt",
		"provider-corrupt",
		active.Manifest.GeneratedAt.Add(time.Minute),
	)
	corrupt.Payload[0] ^= 1
	if published, err := subscriber.activate(
		context.Background(),
		corrupt,
	); err == nil || published {
		t.Fatalf("activate corrupt = %t/%v", published, err)
	}
	if subscriber.Catalog() != activeCatalog ||
		subscriber.active != activeIdentity {
		t.Fatal("invalid generation changed active catalog or identity")
	}
}

func subscriberTestGeneration(
	t testing.TB,
	generationID string,
	providerID catalogs.ProviderID,
	generatedAt time.Time,
) catalogstore.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID: providerID, Name: string(providerID),
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogstore.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generation := catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    generationID,
			GeneratedAt:     generatedAt,
			Payload:         descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "remote-subscriber-test/v1",
				ValidatedAt:      generatedAt,
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{
					Name:   "test",
					Status: catalogs.GenerationValidationCheckPassed,
				}},
			},
			SyncRunID: "sync-" + generationID,
			SourceObservations: []catalogs.SourceObservationLink{{
				Source:        catalogmeta.LocalCatalogID,
				ObservationID: "observation-" + generationID,
				ObservedAt:    generatedAt,
				Revision: catalogmeta.ObservationRevision{
					Kind:  catalogmeta.ObservationRevisionKindContentDigest,
					Value: descriptor.Checksum,
				},
				Completeness:     catalogmeta.ObservationCompletenessComplete,
				Status:           catalogmeta.ObservationStatusSucceeded,
				EvidenceChecksum: descriptor.Checksum,
			}},
			Completeness: catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
	if err := generation.Validate(); err != nil {
		t.Fatalf("generation fixture: %v", err)
	}
	return generation
}

func writeSubscriberManifest(
	t testing.TB,
	writer http.ResponseWriter,
	generation catalogstore.Generation,
) {
	t.Helper()
	data, err := catalogremote.MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatalf("MarshalManifest: %v", err)
	}
	writer.Header().Set("Content-Type", catalogremote.ManifestMediaType)
	_, _ = writer.Write(data)
}

func waitForSubscriberCondition(t testing.TB, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for subscriber condition")
}
