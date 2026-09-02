package remote

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// Catalog transfer bounds. Each value bounds one stage of one finite HTTP
// body transfer. No value bounds a subscription lifetime.
const (
	// DefaultConnectTimeout bounds one TCP connection attempt.
	DefaultConnectTimeout = 30 * time.Second

	// DefaultTLSHandshakeTimeout bounds one TLS handshake.
	DefaultTLSHandshakeTimeout = 30 * time.Second

	// DefaultResponseHeaderTimeout bounds the wait for response headers after
	// the client writes the request.
	DefaultResponseHeaderTimeout = 60 * time.Second

	// DefaultTransferIdleTimeout bounds the time a transfer may make no
	// progress. Every successful body read resets the timer.
	DefaultTransferIdleTimeout = 2 * time.Minute

	// DefaultTransferMaxDuration bounds one complete body transfer. A 64 MiB
	// body at 256 kilobits per second takes about 35 minutes, so this value
	// leaves headroom over a slow link.
	DefaultTransferMaxDuration = 60 * time.Minute

	// DefaultMaxCompressedBytes bounds the bytes read from one response body.
	DefaultMaxCompressedBytes int64 = 64 << 20

	// DefaultMaxExpandedBytes bounds the bytes one compressed body may expand
	// into after decoding.
	DefaultMaxExpandedBytes int64 = 256 << 20

	// keepAliveInterval is the TCP keep-alive probe interval.
	keepAliveInterval = 30 * time.Second

	// expectContinueTimeout bounds the wait for a 100 Continue response.
	expectContinueTimeout = time.Second

	// idleConnectionTimeout bounds an unused pooled connection.
	idleConnectionTimeout = 90 * time.Second

	// maxIdleConnections bounds the shared connection pool.
	maxIdleConnections = 100

	// transferChunkBytes is the read size of one progress step.
	transferChunkBytes = 32 << 10
)

// TransferStage names one phase of one catalog transfer.
type TransferStage string

const (
	// TransferStageHeaders reports that the response headers arrived.
	TransferStageHeaders TransferStage = "headers"
	// TransferStageBody reports body bytes in flight.
	TransferStageBody TransferStage = "body"
	// TransferStageComplete reports a finished body.
	TransferStageComplete TransferStage = "complete"
)

// transfer failure causes. A guard records one cause before it cancels the
// request, so the caller reports the bound that stopped the transfer.
const (
	causeIdle        = "idle"
	causeMaxDuration = "max_duration"
)

// TransferProgress reports how much of one transfer arrived. The resource is
// a safe caller-supplied label. It never carries a URL, a token, or a host
// name.
type TransferProgress struct {
	// Resource is the safe label of the transferred resource.
	Resource string

	// Stage is the phase that produced this report.
	Stage TransferStage

	// BytesReceived is the running count of body bytes read.
	BytesReceived int64

	// TotalBytes is the declared body length, or zero when the response
	// declares none.
	TotalBytes int64
}

// ProgressFunc receives one transfer progress report. Implementations must
// return promptly, because the transfer calls them inline.
type ProgressFunc func(TransferProgress)

// TransferPolicy bounds one finite HTTP body transfer at every stage. It
// replaces http.Client.Timeout, which also covers body reads and therefore
// rejects a healthy slow link.
type TransferPolicy struct {
	// ConnectTimeout bounds one TCP connection attempt.
	ConnectTimeout time.Duration

	// TLSHandshakeTimeout bounds one TLS handshake.
	TLSHandshakeTimeout time.Duration

	// ResponseHeaderTimeout bounds the wait for response headers.
	ResponseHeaderTimeout time.Duration

	// IdleTimeout bounds the time one transfer may make no progress.
	IdleTimeout time.Duration

	// MaxDuration bounds one complete body transfer. Zero is invalid.
	MaxDuration time.Duration

	// MaxCompressedBytes bounds the bytes read from one response body.
	MaxCompressedBytes int64

	// MaxExpandedBytes bounds the bytes one body may expand into.
	MaxExpandedBytes int64
}

// DefaultTransferPolicy returns the catalog transfer bounds.
func DefaultTransferPolicy() TransferPolicy {
	return TransferPolicy{
		ConnectTimeout:        DefaultConnectTimeout,
		TLSHandshakeTimeout:   DefaultTLSHandshakeTimeout,
		ResponseHeaderTimeout: DefaultResponseHeaderTimeout,
		IdleTimeout:           DefaultTransferIdleTimeout,
		MaxDuration:           DefaultTransferMaxDuration,
		MaxCompressedBytes:    DefaultMaxCompressedBytes,
		MaxExpandedBytes:      DefaultMaxExpandedBytes,
	}
}

// Validate reports whether every bound is positive. A zero maximum duration
// is invalid, because an unbounded transfer can hold a connection forever.
func (p TransferPolicy) Validate() error {
	for _, bound := range []struct {
		name  string
		value time.Duration
	}{
		{"connect_timeout", p.ConnectTimeout},
		{"tls_handshake_timeout", p.TLSHandshakeTimeout},
		{"response_header_timeout", p.ResponseHeaderTimeout},
		{"idle_timeout", p.IdleTimeout},
		{"max_duration", p.MaxDuration},
	} {
		if bound.value <= 0 {
			return transferValidation(bound.name, bound.value, "must be positive")
		}
	}
	for _, bound := range []struct {
		name  string
		value int64
	}{
		{"max_compressed_bytes", p.MaxCompressedBytes},
		{"max_expanded_bytes", p.MaxExpandedBytes},
	} {
		if bound.value <= 0 {
			return transferValidation(bound.name, bound.value, "must be positive")
		}
	}
	if p.MaxExpandedBytes < p.MaxCompressedBytes {
		return transferValidation("max_expanded_bytes", p.MaxExpandedBytes,
			"must be at least the compressed bound")
	}
	return nil
}

// NewTransport returns an HTTP transport that applies the connection, TLS, and
// response-header bounds of the policy. The body bounds belong to Transfer.
func NewTransport(policy TransferPolicy) (*http.Transport, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return newTransport(policy), nil
}

// newTransport builds the transport of an already valid policy.
func newTransport(policy TransferPolicy) *http.Transport {
	dialer := &net.Dialer{Timeout: policy.ConnectTimeout, KeepAlive: keepAliveInterval}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   policy.TLSHandshakeTimeout,
		ResponseHeaderTimeout: policy.ResponseHeaderTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxIdleConnections,
		IdleConnTimeout:       idleConnectionTimeout,
	}
}

// NewTransferClient returns an HTTP client that applies the policy through its
// transport. The client sets no total timeout, because http.Client.Timeout
// also covers body reads and cannot coexist with progress-aware transfers.
func NewTransferClient(policy TransferPolicy) (*http.Client, error) {
	transport, err := NewTransport(policy)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport}, nil
}

// DefaultTransferClient returns a transfer client with the default policy.
// The default policy is a set of constants, so this call cannot fail.
func DefaultTransferClient() *http.Client {
	return &http.Client{Transport: newTransport(DefaultTransferPolicy())}
}

// Transfer reads one HTTP body under a bound and reports its progress.
type Transfer struct {
	// Client sends the request. A nil client uses the default transfer client.
	Client *http.Client

	// Policy bounds the transfer. A zero policy uses the defaults.
	Policy TransferPolicy

	// Progress receives progress reports. A nil value reports nothing.
	Progress ProgressFunc
}

// Reply is one complete bounded HTTP reply. The transfer already read and
// closed the body, so a caller owns no stream and closes nothing.
type Reply struct {
	// StatusCode is the reply status.
	StatusCode int

	// Header is a caller-owned copy of the reply header.
	Header http.Header

	// Body is the complete reply body.
	Body []byte
}

// Body sends request and reads the complete response body under the policy.
// The resource is a safe label for progress and error reporting.
//
// Body returns a *errors.TimeoutError when the inactivity bound or the
// per-transfer maximum stops the transfer, and a *errors.ValidationError when
// the body exceeds the size bound.
func (t Transfer) Body(ctx context.Context, request *http.Request, resource string) (Reply, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	policy := t.Policy
	if policy == (TransferPolicy{}) {
		policy = DefaultTransferPolicy()
	}
	if err := policy.Validate(); err != nil {
		return Reply{}, err
	}
	client := t.Client
	if client == nil {
		client = DefaultTransferClient()
	}

	transferCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	guard := newTransferGuard(cancel, policy)
	defer guard.stop()

	// The caller owns the destination. This transfer is the shared catalog
	// download path, so the URL is a configured catalog source or a caller
	// integration point.
	response, err := client.Do(request.Clone(transferCtx)) //nolint:gosec // The caller owns the catalog source URL.
	if err != nil {
		return Reply{}, guard.wrap(ctx, resource, err)
	}
	defer func() { _ = response.Body.Close() }()
	t.report(TransferProgress{
		Resource: resource, Stage: TransferStageHeaders, TotalBytes: response.ContentLength,
	})
	guard.observeProgress()

	body, err := t.read(ctx, guard, policy, response, resource)
	if err != nil {
		return Reply{}, err
	}
	t.report(TransferProgress{
		Resource:      resource,
		Stage:         TransferStageComplete,
		BytesReceived: int64(len(body)),
		TotalBytes:    response.ContentLength,
	})
	return Reply{
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		Body:       body,
	}, nil
}

// read consumes the body in progress steps and enforces the size bound.
func (t Transfer) read(
	ctx context.Context,
	guard *transferGuard,
	policy TransferPolicy,
	response *http.Response,
	resource string,
) ([]byte, error) {
	if response.ContentLength > policy.MaxCompressedBytes {
		return nil, transferValidation("body", response.ContentLength, "exceeds the transfer size bound")
	}
	limited := io.LimitReader(response.Body, policy.MaxCompressedBytes+1)
	body := make([]byte, 0, transferChunkBytes)
	chunk := make([]byte, transferChunkBytes)
	for {
		count, err := limited.Read(chunk)
		if count > 0 {
			guard.observeProgress()
			body = append(body, chunk[:count]...)
			if int64(len(body)) > policy.MaxCompressedBytes {
				return nil, transferValidation("body", len(body), "exceeds the transfer size bound")
			}
			t.report(TransferProgress{
				Resource:      resource,
				Stage:         TransferStageBody,
				BytesReceived: int64(len(body)),
				TotalBytes:    response.ContentLength,
			})
		}
		if err == io.EOF {
			return body, nil
		}
		if err != nil {
			return nil, guard.wrap(ctx, resource, err)
		}
	}
}

func (t Transfer) report(progress TransferProgress) {
	if t.Progress != nil {
		t.Progress(progress)
	}
}

// transferGuard cancels one transfer when it stalls or exceeds its maximum
// duration. It records the cause before it cancels, so the caller reports the
// bound that stopped the transfer rather than a plain cancellation.
type transferGuard struct {
	mu       sync.Mutex
	cause    string
	stopped  bool
	cancel   context.CancelFunc
	idle     *time.Timer
	maximum  *time.Timer
	policy   TransferPolicy
	idleWait time.Duration
}

func newTransferGuard(cancel context.CancelFunc, policy TransferPolicy) *transferGuard {
	guard := &transferGuard{cancel: cancel, policy: policy, idleWait: policy.IdleTimeout}
	guard.idle = time.AfterFunc(policy.IdleTimeout, func() { guard.trip(causeIdle) })
	guard.maximum = time.AfterFunc(policy.MaxDuration, func() { guard.trip(causeMaxDuration) })
	return guard
}

// observeProgress restarts the inactivity timer after real progress.
func (g *transferGuard) observeProgress() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped || g.cause != "" {
		return
	}
	g.idle.Reset(g.idleWait)
}

// trip records the first cause and cancels the transfer.
func (g *transferGuard) trip(cause string) {
	g.mu.Lock()
	if g.stopped || g.cause != "" {
		g.mu.Unlock()
		return
	}
	g.cause = cause
	g.mu.Unlock()
	g.cancel()
}

// stop releases the timers.
func (g *transferGuard) stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopped = true
	g.idle.Stop()
	g.maximum.Stop()
}

// wrap converts one transport error into the bound that caused it.
func (g *transferGuard) wrap(ctx context.Context, resource string, err error) error {
	g.mu.Lock()
	cause := g.cause
	g.mu.Unlock()
	switch cause {
	case causeIdle:
		return &errors.TimeoutError{
			Operation: "catalog transfer " + resource,
			Duration:  g.policy.IdleTimeout.String(),
			Message:   "the transfer made no progress within the inactivity bound",
		}
	case causeMaxDuration:
		return &errors.TimeoutError{
			Operation: "catalog transfer " + resource,
			Duration:  g.policy.MaxDuration.String(),
			Message:   "the transfer exceeded the per-transfer maximum duration",
		}
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return &errors.APIError{
		Provider: "catalog-transfer",
		Endpoint: resource,
		Message:  "transfer failed",
		Err:      err,
	}
}

func transferValidation(field string, value any, message string) error {
	return &errors.ValidationError{
		Field:   "catalog_transfer." + field,
		Value:   value,
		Message: message,
	}
}
