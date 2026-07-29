package remote

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestSubscriberRejectsUnauthorizedStreamWithoutRetryOrPolling(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-unauthorized",
		"provider-unauthorized",
		time.Date(2026, time.July, 29, 19, 0, 0, 0, time.UTC),
	)
	var streamRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path[len("/api/v1"):] {
			case catalogremote.ManifestPath:
				writeSubscriberManifest(t, writer, generation)
			case catalogremote.SnapshotPath(generation.Manifest.GenerationID):
				writer.Header().Set(
					"Content-Type",
					catalogs.CatalogPayloadMediaType,
				)
				_, _ = writer.Write(generation.Payload)
			case catalogremote.EventStreamPath:
				streamRequests.Add(1)
				writer.WriteHeader(http.StatusForbidden)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	subscriber, err := New(Config{
		BaseURL:    server.URL + "/api/v1",
		HTTPClient: server.Client(),
		PollingFallback: &PollingFallbackPolicy{
			AfterFailures: 1,
			Interval:      time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = subscriber.Start(context.Background())
	var apiErr *pkgerrors.APIError
	if !stderrors.As(err, &apiErr) ||
		apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("Start error = %v, want typed forbidden API error", err)
	}
	if got := streamRequests.Load(); got != 1 {
		t.Fatalf("unauthorized stream requests = %d, want no retry", got)
	}
	if status := subscriber.PollingFallbackStatus(); status.Active ||
		status.Polls != 0 {
		t.Fatalf("unauthorized request entered fallback: %#v", status)
	}
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSubscriberStopsAfterUnauthorizedReconnectAndRejectsRestart(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-reconnect-unauthorized",
		"provider-reconnect-unauthorized",
		time.Date(2026, time.July, 29, 19, 5, 0, 0, time.UTC),
	)
	var (
		streamRequests atomic.Int32
		closeStream    = make(chan struct{})
	)
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path[len("/api/v1"):] {
			case catalogremote.ManifestPath:
				writeSubscriberManifest(t, writer, generation)
			case catalogremote.SnapshotPath(generation.Manifest.GenerationID):
				writer.Header().Set(
					"Content-Type",
					catalogs.CatalogPayloadMediaType,
				)
				_, _ = writer.Write(generation.Payload)
			case catalogremote.EventStreamPath:
				if streamRequests.Add(1) > 1 {
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				writer.Header().Set(
					"Content-Type",
					catalogremote.EventStreamMediaType,
				)
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
		ReconnectMinDelay: time.Millisecond,
		ReconnectMaxDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := subscriber.Start(ctx); !stderrors.Is(err, pkgerrors.ErrConflict) {
		t.Fatalf("second Start error = %v, want conflict", err)
	}
	close(closeStream)
	waitForSubscriberCondition(t, func() bool {
		subscriber.mu.Lock()
		defer subscriber.mu.Unlock()
		return subscriber.state == stateStopped
	})
	if got := streamRequests.Load(); got != 2 {
		t.Fatalf("stream requests after unauthorized reconnect = %d, want 2", got)
	}
	if err := subscriber.Start(ctx); !stderrors.Is(err, pkgerrors.ErrConflict) {
		t.Fatalf("Start after stop error = %v, want conflict", err)
	}
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSubscriberStopsWhenFallbackPollBecomesUnauthorized(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-poll-unauthorized",
		"provider-poll-unauthorized",
		time.Date(2026, time.July, 29, 19, 7, 0, 0, time.UTC),
	)
	var (
		manifestRequests atomic.Int32
		streamRequests   atomic.Int32
	)
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path[len("/api/v1"):] {
			case catalogremote.ManifestPath:
				if manifestRequests.Add(1) > 1 {
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				writeSubscriberManifest(t, writer, generation)
			case catalogremote.SnapshotPath(generation.Manifest.GenerationID):
				writer.Header().Set(
					"Content-Type",
					catalogs.CatalogPayloadMediaType,
				)
				_, _ = writer.Write(generation.Payload)
			case catalogremote.EventStreamPath:
				streamRequests.Add(1)
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
		ReconnectMinDelay: time.Millisecond,
		ReconnectMaxDelay: time.Millisecond,
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
	waitForSubscriberCondition(t, func() bool {
		subscriber.mu.Lock()
		defer subscriber.mu.Unlock()
		return subscriber.state == stateStopped
	})
	if got := manifestRequests.Load(); got != 2 {
		t.Fatalf("manifest requests = %d, want initial plus terminal poll", got)
	}
	if got := streamRequests.Load(); got != 1 {
		t.Fatalf("stream requests = %d, want initial attempt only", got)
	}
	status := subscriber.PollingFallbackStatus()
	if status.Active || status.Entries != 1 || status.Polls != 1 ||
		status.Modified != 0 {
		t.Fatalf("fallback status after terminal poll = %#v", status)
	}
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSubscriberOutOfOrderEventsCannotRegressCatalog(t *testing.T) {
	t.Parallel()

	first := subscriberTestGeneration(
		t,
		"generation-order-first",
		"provider-order-first",
		time.Date(2026, time.July, 29, 19, 10, 0, 0, time.UTC),
	)
	second := subscriberTestGeneration(
		t,
		"generation-order-second",
		"provider-order-second",
		first.Manifest.GeneratedAt.Add(time.Minute),
	)
	third := subscriberTestGeneration(
		t,
		"generation-order-third",
		"provider-order-third",
		second.Manifest.GeneratedAt.Add(time.Minute),
	)
	var (
		currentMu sync.RWMutex
		current   = first
		events    = make(chan string, 2)
	)
	generations := map[string]catalogstore.Generation{
		first.Manifest.GenerationID:  first,
		second.Manifest.GenerationID: second,
		third.Manifest.GenerationID:  third,
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			resourcePath := request.URL.Path[len("/api/v1"):]
			currentMu.RLock()
			selected := current
			currentMu.RUnlock()
			switch resourcePath {
			case catalogremote.ManifestPath:
				writeSubscriberManifest(t, writer, selected)
				return
			case catalogremote.EventStreamPath:
				writer.Header().Set(
					"Content-Type",
					catalogremote.EventStreamMediaType,
				)
				_, _ = fmt.Fprint(writer, ": connected\n\n")
				writer.(http.Flusher).Flush()
				for {
					select {
					case <-request.Context().Done():
						return
					case frame := <-events:
						_, _ = fmt.Fprint(writer, frame)
						writer.(http.Flusher).Flush()
					}
				}
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
		BaseURL:         server.URL + "/api/v1",
		HTTPClient:      server.Client(),
		ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = subscriber.Close() }()

	currentMu.Lock()
	current = third
	currentMu.Unlock()
	events <- publicationFrame(1, third.Manifest.GenerationID)
	waitForSubscriberCondition(t, func() bool {
		_, err := subscriber.Catalog().Provider("provider-order-third")
		return err == nil
	})
	thirdCatalog := subscriber.Catalog()

	events <- publicationFrame(2, second.Manifest.GenerationID)
	waitForSubscriberCondition(t, func() bool {
		return subscriber.currentLastEventID() == "2"
	})
	if subscriber.Catalog() != thirdCatalog {
		t.Fatal("out-of-order retained event regressed the immutable catalog")
	}
	if _, err := subscriber.Catalog().Provider("provider-order-second"); err == nil {
		t.Fatal("out-of-order provider became active")
	}
}

func publicationFrame(sequence uint64, generationID string) string {
	return fmt.Sprintf(
		"id: %d\nevent: catalog.published\ndata: {\"generation_id\":%q,\"sequence\":%d}\n\n",
		sequence,
		generationID,
		sequence,
	)
}
