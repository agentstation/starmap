package remote

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

// testIdleTimeout is short enough to keep the stall tests fast.
const testIdleTimeout = 150 * time.Millisecond

// testMaxDuration bounds the slow-drip transfer test.
const testMaxDuration = 300 * time.Millisecond

// testDripInterval paces the slow-drip server.
const testDripInterval = 10 * time.Millisecond

// testHeaderDelay exceeds the response-header bound of the transport test.
const testHeaderDelay = 300 * time.Millisecond

// testHeaderTimeout is the response-header bound of the transport test.
const testHeaderTimeout = 60 * time.Millisecond

// fastPolicy returns a valid policy with short bounds for tests.
func fastPolicy() TransferPolicy {
	policy := DefaultTransferPolicy()
	policy.ConnectTimeout = time.Second
	policy.TLSHandshakeTimeout = time.Second
	policy.ResponseHeaderTimeout = time.Second
	policy.IdleTimeout = testIdleTimeout
	policy.MaxDuration = 10 * time.Second
	return policy
}

func getRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	return request
}

func TestTransferMaxDurationDefaultsToSixtyMinutes(t *testing.T) {
	t.Parallel()

	policy := DefaultTransferPolicy()
	if policy.MaxDuration != 60*time.Minute {
		t.Fatalf("MaxDuration = %s, want 60m0s", policy.MaxDuration)
	}
	if DefaultTransferMaxDuration != 60*time.Minute {
		t.Fatalf("DefaultTransferMaxDuration = %s, want 60m0s", DefaultTransferMaxDuration)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate default policy: %v", err)
	}
	if policy.IdleTimeout != 2*time.Minute {
		t.Fatalf("IdleTimeout = %s, want 2m0s", policy.IdleTimeout)
	}
}

func TestTransferMaxDurationRejectsZero(t *testing.T) {
	t.Parallel()

	policy := DefaultTransferPolicy()
	policy.MaxDuration = 0
	err := policy.Validate()
	if err == nil {
		t.Fatal("Validate accepted a zero maximum duration")
	}
	var validation *starmaperrors.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Validate error = %T, want *errors.ValidationError", err)
	}
	if validation.Field != "catalog_transfer.max_duration" {
		t.Fatalf("Validate field = %q, want catalog_transfer.max_duration", validation.Field)
	}
	if _, err := NewTransferClient(policy); err == nil {
		t.Fatal("NewTransferClient accepted a zero maximum duration")
	}

	policy.MaxDuration = -time.Second
	if err := policy.Validate(); err == nil {
		t.Fatal("Validate accepted a negative maximum duration")
	}
}

func TestTransferIdleTimeoutStopsStalledBody(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/octet-stream")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("first"))
		writer.(http.Flusher).Flush()
		select {
		case <-release:
		case <-request.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	var reported atomic.Int64
	transfer := Transfer{
		Client:   server.Client(),
		Policy:   fastPolicy(),
		Progress: func(progress TransferProgress) { reported.Store(progress.BytesReceived) },
	}
	started := time.Now()
	_, err := transfer.Body(context.Background(), getRequest(t, server.URL), "stalled-asset")
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("Body read a stalled response without an error")
	}
	var timeout *starmaperrors.TimeoutError
	if !errors.As(err, &timeout) {
		t.Fatalf("Body error = %T (%v), want *errors.TimeoutError", err, err)
	}
	if !strings.Contains(timeout.Message, "inactivity") {
		t.Fatalf("timeout message = %q, want the inactivity bound", timeout.Message)
	}
	if timeout.Duration != testIdleTimeout.String() {
		t.Fatalf("timeout duration = %q, want %q", timeout.Duration, testIdleTimeout.String())
	}
	if elapsed > 5*time.Second {
		t.Fatalf("stalled transfer took %s, want the inactivity bound", elapsed)
	}
	if got := reported.Load(); got == 0 {
		t.Fatal("progress reported no received bytes before the stall")
	}
}

func TestSlowDripTransferFailsAtMaxDuration(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/octet-stream")
		writer.WriteHeader(http.StatusOK)
		ticker := time.NewTicker(testDripInterval)
		defer ticker.Stop()
		for {
			select {
			case <-request.Context().Done():
				return
			case <-ticker.C:
				if _, err := writer.Write([]byte("x")); err != nil {
					return
				}
				writer.(http.Flusher).Flush()
			}
		}
	}))
	defer server.Close()

	policy := fastPolicy()
	// The drip keeps resetting the inactivity timer, so only the per-transfer
	// maximum can stop this transfer.
	policy.IdleTimeout = 5 * time.Second
	policy.MaxDuration = testMaxDuration
	transfer := Transfer{Client: server.Client(), Policy: policy}

	started := time.Now()
	_, err := transfer.Body(context.Background(), getRequest(t, server.URL), "drip-asset")
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("Body read an endless drip without an error")
	}
	var timeout *starmaperrors.TimeoutError
	if !errors.As(err, &timeout) {
		t.Fatalf("Body error = %T (%v), want *errors.TimeoutError", err, err)
	}
	if !strings.Contains(timeout.Message, "maximum duration") {
		t.Fatalf("timeout message = %q, want the per-transfer maximum", timeout.Message)
	}
	if timeout.Duration != testMaxDuration.String() {
		t.Fatalf("timeout duration = %q, want %q", timeout.Duration, testMaxDuration.String())
	}
	if elapsed < testMaxDuration {
		t.Fatalf("drip transfer stopped after %s, want at least %s", elapsed, testMaxDuration)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("drip transfer took %s, want the per-transfer maximum", elapsed)
	}
}

func TestCatalogTransportAppliesResponseHeaderTimeout(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		select {
		case <-time.After(testHeaderDelay):
		case <-request.Context().Done():
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	policy := fastPolicy()
	policy.ResponseHeaderTimeout = testHeaderTimeout
	client, err := NewTransferClient(policy)
	if err != nil {
		t.Fatalf("NewTransferClient: %v", err)
	}
	if client.Timeout != 0 {
		t.Fatalf("transfer client timeout = %s, want no client-wide timeout", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transfer client transport = %T, want *http.Transport", client.Transport)
	}
	if transport.ResponseHeaderTimeout != testHeaderTimeout {
		t.Fatalf("response header timeout = %s, want %s",
			transport.ResponseHeaderTimeout, testHeaderTimeout)
	}

	// The bound applies to every request the client sends, not only the first.
	for attempt := 1; attempt <= 2; attempt++ {
		started := time.Now()
		_, err := client.Do(getRequest(t, server.URL))
		if err == nil {
			t.Fatalf("attempt %d received a response after the header bound", attempt)
		}
		if elapsed := time.Since(started); elapsed >= testHeaderDelay {
			t.Fatalf("attempt %d waited %s, want the %s header bound", attempt, elapsed, testHeaderTimeout)
		}
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("server requests = %d, want 2", got)
	}
}

func TestTransferReportsProgressAndRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("catalog", 1024)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte(payload))
	}))
	defer server.Close()

	var stages []TransferStage
	transfer := Transfer{
		Client:   server.Client(),
		Policy:   fastPolicy(),
		Progress: func(progress TransferProgress) { stages = append(stages, progress.Stage) },
	}
	answer, err := transfer.Body(context.Background(), getRequest(t, server.URL), "catalog-asset")
	if err != nil {
		t.Fatalf("Body: %v", err)
	}
	if string(answer.Body) != payload {
		t.Fatalf("body length = %d, want %d", len(answer.Body), len(payload))
	}
	if len(stages) < 2 || stages[0] != TransferStageHeaders ||
		stages[len(stages)-1] != TransferStageComplete {
		t.Fatalf("progress stages = %v, want headers first and complete last", stages)
	}

	bounded := fastPolicy()
	bounded.MaxCompressedBytes = 16
	bounded.MaxExpandedBytes = 16
	small := Transfer{Client: server.Client(), Policy: bounded}
	if _, err := small.Body(context.Background(), getRequest(t, server.URL), "catalog-asset"); err == nil {
		t.Fatal("Body accepted a body beyond the size bound")
	}
}
