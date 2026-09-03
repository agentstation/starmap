package remote

import (
	"context"
	stderrors "errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/errors"
)

type lifecycleState uint8

const (
	stateIdle lifecycleState = iota
	stateStarting
	stateRunning
	stateStopped
)

// Subscriber owns one explicitly started remote catalog lifecycle.
type Subscriber struct {
	config   Config
	protocol *protocol.Client
	client   *starmap.Client

	mu              sync.Mutex
	state           lifecycleState
	cancel          context.CancelFunc
	done            chan struct{}
	lastEventID     string
	fallback        PollingFallbackStatus
	streamState     StreamState
	lastHeartbeatAt time.Time
	lastEventAt     time.Time
	lastCatchUpAt   time.Time
	retries         uint64
	lastError       *HealthError
	retryNotBefore  time.Time
	upstream        *UpstreamReport

	activationMu        sync.Mutex
	identityEstablished bool

	// reconnect paces stream reconnects. The run loop owns it, and it
	// publishes its observable values through the fields under mu.
	reconnect reconnectState

	retryDelay func(int) time.Duration
	now        func() time.Time
}

// PollingFallbackStatus is an immutable snapshot of the subscriber's bounded
// polling fallback. Counters are cumulative for the subscriber lifetime.
type PollingFallbackStatus struct {
	// Enabled reports whether construction configured a polling fallback.
	Enabled bool
	// Active reports that failures reached the threshold and the stream has not
	// recovered.
	Active bool
	// Entries counts transitions into fallback mode.
	Entries uint64
	// Polls counts conditional current-manifest requests.
	Polls uint64
	// Modified counts verified non-304 responses handled by fallback polling.
	Modified uint64
}

// New makes an idle subscriber and uses context.Background for store I/O. It
// does not create a goroutine or send a remote request. Call NewContext to
// cancel store I/O or set a deadline.
func New(config Config) (*Subscriber, error) {
	return NewContext(context.Background(), config)
}

// NewContext validates config and makes an idle subscriber. The context bounds
// caller-store reads and an optional pinned-bootstrap commit. NewContext does
// not create a goroutine or send a remote request.
func NewContext(ctx context.Context, config Config) (*Subscriber, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{
			Field: "remote.context", Message: "is required",
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalized, err := config.normalized()
	if err != nil {
		return nil, err
	}
	httpClient := normalized.HTTPClient
	if httpClient == nil {
		httpClient, err = normalized.transferClient()
		if err != nil {
			return nil, err
		}
	}
	protocol, err := protocol.NewClient(
		normalized.BaseURL,
		httpClient,
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		return nil, err
	}
	if normalized.PinnedBootstrap != nil {
		_, currentErr := normalized.CatalogStore.Current(ctx)
		switch {
		case currentErr == nil:
			// A durable current generation always wins over the offline pin.
		case stderrors.Is(currentErr, errors.ErrNotFound):
			if err := normalized.CatalogStore.Commit(
				ctx,
				*normalized.PinnedBootstrap,
				"",
			); err != nil {
				return nil, errors.WrapResource(
					"commit",
					"pinned bootstrap generation",
					normalized.PinnedBootstrap.Manifest.GenerationID,
					err,
				)
			}
		default:
			return nil, errors.WrapResource(
				"load",
				"stored current catalog generation",
				"current",
				currentErr,
			)
		}
	}
	client, err := starmap.NewContext(
		ctx,
		starmap.WithCatalogStore(normalized.CatalogStore),
	)
	if err != nil {
		return nil, errors.WrapResource(
			"construct",
			"remote catalog subscriber",
			normalized.BaseURL,
			err,
		)
	}
	subscriber := &Subscriber{
		config:      normalized,
		protocol:    protocol,
		client:      client,
		state:       stateIdle,
		streamState: StreamStateIdle,
		fallback: PollingFallbackStatus{
			Enabled: normalized.PollingFallback != nil,
		},
		identityEstablished: !client.Readiness().Embedded.Active,
		reconnect:           newReconnectState(normalized),
	}
	return subscriber, nil
}

// PollingFallbackStatus returns the current bounded polling fallback state.
func (s *Subscriber) PollingFallbackStatus() PollingFallbackStatus {
	if s == nil {
		return PollingFallbackStatus{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fallback
}

// Catalog returns the catalog from State. Construction selects the verified
// durable current generation, the optional pinned bootstrap for an empty
// store, or the embedded bootstrap in that order.
func (s *Subscriber) Catalog() *catalogs.Catalog {
	return s.State().Catalog
}

// State returns one atomic catalog, generation identity, payload checksum,
// timestamp, and sequence snapshot without performing I/O.
func (s *Subscriber) State() starmap.CatalogState {
	if s == nil || s.client == nil {
		return starmap.CatalogState{}
	}
	return s.client.CurrentCatalogState()
}

// Start runs the caller-context-owned remote lifecycle. It normally verifies
// current state, establishes the event stream, and closes the fetch-to-subscribe
// gap before it returns. A nonterminal initial transport failure keeps the
// verified local state and runs streaming recovery. Polling runs only when
// PollingFallbackPolicy enables it. HTTP 401 and 403 responses are terminal and
// never retry or enter polling fallback.
func (s *Subscriber) Start(ctx context.Context) error {
	if s == nil {
		return &errors.ValidationError{
			Field: "remote.subscriber", Message: "is required",
		}
	}
	if ctx == nil {
		return &errors.ValidationError{
			Field: "remote.context", Message: "is required",
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if s.state != stateIdle {
		actual := s.state.String()
		s.mu.Unlock()
		cancel()
		return &errors.ConflictError{
			Resource: "remote catalog subscriber",
			Expected: "idle",
			Actual:   actual,
			Message:  "Start may be called only once",
		}
	}
	s.state = stateStarting
	s.streamState = StreamStateStarting
	s.cancel = cancel
	s.done = make(chan struct{})
	done := s.done
	s.mu.Unlock()

	started := false
	defer func() {
		if started {
			return
		}
		cancel()
		s.mu.Lock()
		if s.state == stateStarting {
			s.state = stateIdle
			s.streamState = StreamStateIdle
			s.cancel = nil
			s.done = nil
		}
		s.mu.Unlock()
		close(done)
	}()

	initial, err := s.protocol.FetchCurrent(runCtx)
	if err != nil {
		s.recordHealthError("initial_fetch", err)
		if runErr := runCtx.Err(); runErr != nil {
			return runErr
		}
		if s.terminal(err) {
			return err
		}
		s.observeRefusal(err)
		if err := s.beginRun(runCtx, nil, done, 1, StreamStateRetrying); err != nil {
			return err
		}
		started = true
		return nil
	}
	if _, err := s.activate(runCtx, initial); err != nil {
		s.recordHealthError("initial_activate", err)
		return err
	}

	stream, err := s.protocol.OpenEventStream(runCtx, "")
	if err != nil {
		s.recordHealthError("stream_open", err)
		if runErr := runCtx.Err(); runErr != nil {
			return runErr
		}
		if s.terminal(err) {
			return err
		}
		s.observeRefusal(err)
		if err := s.beginRun(runCtx, nil, done, 1, StreamStateRetrying); err != nil {
			return err
		}
		started = true
		return nil
	}
	// The open stream starts its liveness window here. A later reconnect
	// resets the backoff only when this window completes.
	s.reconnect.opened(s.currentTime())
	if err := s.catchUp(runCtx); err != nil {
		_ = stream.Close()
		s.reconnect.closed(s.currentTime())
		if runErr := runCtx.Err(); runErr != nil {
			return runErr
		}
		if s.terminal(err) {
			return err
		}
		s.observeRefusal(err)
		if err := s.beginRun(runCtx, nil, done, 1, StreamStateRetrying); err != nil {
			return err
		}
		started = true
		return nil
	}
	if err := s.beginRun(runCtx, stream, done, 0, StreamStateStreaming); err != nil {
		_ = stream.Close()
		return err
	}
	started = true
	return nil
}

func (s *Subscriber) beginRun(
	ctx context.Context,
	stream *protocol.EventStream,
	done chan struct{},
	streamFailures int,
	streamState StreamState,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateStarting {
		return &errors.ConflictError{
			Resource: "remote catalog subscriber",
			Expected: "starting",
			Actual:   s.state.String(),
			Message:  "subscriber lifecycle changed while Start was initializing",
		}
	}
	s.state = stateRunning
	s.streamState = streamState
	go s.run(ctx, stream, done, streamFailures)
	return nil
}

// Close cancels and joins the subscriber lifecycle within ShutdownTimeout. It
// is idempotent.
func (s *Subscriber) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	if s.state == stateIdle {
		s.state = stateStopped
		s.streamState = StreamStateStopped
		s.mu.Unlock()
		return nil
	}
	if s.state == stateStopped && done == nil {
		s.mu.Unlock()
		return nil
	}
	s.state = stateStopped
	s.streamState = StreamStateStopped
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}

	timer := time.NewTimer(s.config.ShutdownTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.NewTimeoutError(
			"close remote catalog subscriber",
			s.config.ShutdownTimeout.String(),
			"owned lifecycle did not stop",
		)
	}
}

func (s *Subscriber) run(
	ctx context.Context,
	stream *protocol.EventStream,
	done chan struct{},
	streamFailures int,
) {
	defer func() {
		if stream != nil {
			_ = stream.Close()
		}
		s.mu.Lock()
		s.state = stateStopped
		s.streamState = StreamStateStopped
		s.fallback.Active = false
		s.mu.Unlock()
		close(done)
	}()

	attempt := 0
	var nextPoll time.Time
	for {
		if stream != nil {
			err := s.consume(ctx, stream)
			_ = stream.Close()
			stream = nil
			if ctx.Err() != nil {
				return
			}
			if err == nil {
				err = io.EOF
			}
			// The backoff resets only when the closed stream stayed open for a
			// healthy liveness window. A stream that opened and failed at once
			// proves no liveness, so its failure keeps the growing delay.
			if s.reconnect.closed(s.currentTime()) {
				attempt = 0
			}
			s.recordHealthError("stream_read", err)
			if s.terminal(err) {
				return
			}
			s.observeRefusal(err)
			streamFailures++
		}

		for {
			if !s.pollFallbackIfDue(
				ctx,
				streamFailures,
				&nextPoll,
			) {
				return
			}
			if !s.awaitCredentialChange(ctx) {
				return
			}
			s.setStreamState(StreamStateRetrying)
			s.recordRetry()
			delay, delayErr := s.nextReconnectDelay(attempt)
			if delayErr != nil {
				s.recordHealthError("stream_backoff", delayErr)
				return
			}
			if !waitRetry(ctx, delay) {
				return
			}

			lastEventID := s.currentLastEventID()
			next, openErr := s.protocol.OpenEventStream(ctx, lastEventID)
			if openErr != nil {
				s.recordHealthError("stream_open", openErr)
				if s.terminal(openErr) {
					return
				}
				s.observeRefusal(openErr)
				attempt++
				streamFailures++
				continue
			}
			// A TCP connection and a first response header prove no
			// liveness. An open stream therefore records the start of the
			// liveness window and never resets the backoff.
			s.reconnect.opened(s.currentTime())
			// Replay only improves efficiency. After each connection, the
			// subscriber fetches and verifies current state before it reads events.
			if catchUpErr := s.catchUp(ctx); catchUpErr != nil {
				_ = next.Close()
				s.reconnect.closed(s.currentTime())
				if s.terminal(catchUpErr) {
					return
				}
				s.observeRefusal(catchUpErr)
				attempt++
				streamFailures++
				continue
			}
			s.finishPollingFallback()
			s.setStreamState(StreamStateStreaming)
			stream = next
			streamFailures = 0
			nextPoll = time.Time{}
			break
		}
	}
}

// nextReconnectDelay returns the wait before the next reconnect attempt. A
// test may replace the delay through retryDelay.
func (s *Subscriber) nextReconnectDelay(attempt int) (time.Duration, error) {
	if s.retryDelay != nil {
		return s.retryDelay(attempt), nil
	}
	delay, err := s.reconnect.next(s.currentTime())
	if err != nil {
		return 0, err
	}
	s.publishRetryBoundary(s.reconnect.boundary())
	return delay, nil
}

// observeRefusal records a declared not-before boundary. The boundary is a
// hard floor that replaces the computed backoff on the next attempt.
func (s *Subscriber) observeRefusal(err error) {
	var refusal *protocol.RefusalError
	if !stderrors.As(err, &refusal) {
		return
	}
	now := s.currentTime()
	s.reconnect.refuse(now, refusal.NotBefore)
	s.publishRetryBoundary(s.reconnect.boundary())
}

// terminal reports whether an error ends the subscriber. An authentication
// failure is terminal only while the caller declared no credential-change
// signal. With a signal the subscriber waits for a new credential instead.
func (s *Subscriber) terminal(err error) bool {
	return isTerminalRemoteError(err) && s.config.CredentialChanges == nil
}

// awaitCredentialChange blocks while the last failure was an authentication
// failure and the caller declared a credential-change signal. A retry with the
// rejected credential cannot succeed, so the subscriber waits instead of
// spending its reconnect budget.
func (s *Subscriber) awaitCredentialChange(ctx context.Context) bool {
	changes := s.config.CredentialChanges
	if changes == nil || !s.lastErrorWasAuthentication() {
		return ctx.Err() == nil
	}
	s.setStreamState(StreamStateWaitingForCredentials)
	select {
	case <-ctx.Done():
		return false
	case _, open := <-changes:
		if !open {
			return false
		}
	}
	s.clearAuthenticationFailure()
	return ctx.Err() == nil
}

func isTerminalRemoteError(err error) bool {
	var apiErr *errors.APIError
	return stderrors.As(err, &apiErr) &&
		(apiErr.StatusCode == http.StatusUnauthorized ||
			apiErr.StatusCode == http.StatusForbidden)
}

func (s *Subscriber) pollFallbackIfDue(
	ctx context.Context,
	streamFailures int,
	nextPoll *time.Time,
) bool {
	policy := s.config.PollingFallback
	if policy == nil || streamFailures < policy.AfterFailures {
		return ctx.Err() == nil
	}
	s.startPollingFallback()
	s.setStreamState(StreamStatePolling)
	now := s.currentTime()
	if nextPoll.IsZero() {
		// The first poll of one fallback lands on the stable phase of this
		// instance. Many degraded subscribers therefore spread across the
		// interval instead of polling together.
		scheduled, err := fallbackPollAt(
			s.config.controllerIdentity(controllerPoll),
			policy.Interval,
			now,
		)
		if err != nil {
			s.recordHealthError("fallback_poll", err)
			return false
		}
		*nextPoll = scheduled
	}
	if now.Before(*nextPoll) {
		return ctx.Err() == nil
	}

	modified, err := s.pollCurrent(ctx)
	s.recordHealthError("fallback_poll", err)
	if ctx.Err() != nil {
		return false
	}
	*nextPoll = s.currentTime().Add(policy.Interval)
	s.recordFallbackPoll(modified)
	s.observeRefusal(err)
	if s.terminal(err) {
		return false
	}
	// A polling failure is observable later through P7.11 health, but it does
	// not replace or delay the primary streaming reconnect path.
	_ = err
	return true
}

func (s *Subscriber) pollCurrent(ctx context.Context) (bool, error) {
	generationID := s.activeGenerationID()
	generation, modified, err := s.protocol.FetchCurrentIfChanged(
		ctx,
		generationID,
	)
	if err != nil || !modified {
		return false, err
	}
	_, err = s.activate(ctx, generation)
	return err == nil, err
}

func (s *Subscriber) startPollingFallback() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.fallback.Active {
		s.fallback.Active = true
		s.fallback.Entries++
	}
}

func (s *Subscriber) recordFallbackPoll(modified bool) {
	s.mu.Lock()
	s.fallback.Polls++
	if modified {
		s.fallback.Modified++
	}
	s.mu.Unlock()
}

func (s *Subscriber) finishPollingFallback() {
	s.mu.Lock()
	s.fallback.Active = false
	s.mu.Unlock()
}

func (s *Subscriber) consume(
	ctx context.Context,
	stream *protocol.EventStream,
) error {
	readerCtx, cancelReader := context.WithCancel(ctx)
	results := make(chan streamReadResult, 1)
	readerDone := make(chan struct{})
	go readEventStream(readerCtx, stream, results, readerDone)
	defer func() {
		cancelReader()
		_ = stream.Close()
		<-readerDone
	}()

	liveness := time.NewTimer(s.config.LivenessTimeout)
	defer liveness.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-liveness.C:
			return errors.NewTimeoutError(
				"read remote catalog event stream",
				s.config.LivenessTimeout.String(),
				"no heartbeat or publication was received",
			)
		case result := <-results:
			if result.err != nil {
				return result.err
			}
			resetTimer(liveness, s.config.LivenessTimeout)
			event := result.event
			if event.Publication == nil {
				if event.Comment == "heartbeat" {
					s.recordHeartbeat()
				}
				continue
			}
			s.recordPublicationEvent()
			if s.isActiveGeneration(event.Publication.GenerationID) {
				s.recordEventID(event.EventID)
				continue
			}
			generation, err := s.protocol.FetchGeneration(
				ctx,
				event.Publication.GenerationID,
			)
			if err != nil {
				return err
			}
			if _, err := s.activate(ctx, generation); err != nil {
				return err
			}
			s.recordEventID(event.EventID)
		}
	}
}

type streamReadResult struct {
	event protocol.StreamEvent
	err   error
}

func readEventStream(
	ctx context.Context,
	stream *protocol.EventStream,
	results chan<- streamReadResult,
	done chan<- struct{},
) {
	defer close(done)
	for {
		event, err := stream.Next()
		select {
		case <-ctx.Done():
			return
		case results <- streamReadResult{event: event, err: err}:
		}
		if err != nil {
			return
		}
	}
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func (s *Subscriber) catchUp(ctx context.Context) error {
	generation, err := s.protocol.FetchCurrent(ctx)
	if err != nil {
		s.recordHealthError("catch_up", err)
		return err
	}
	if _, err = s.activate(ctx, generation); err != nil {
		s.recordHealthError("catch_up", err)
		return err
	}
	s.recordCatchUp()
	return nil
}

func (s *Subscriber) activate(
	ctx context.Context,
	generation catalogs.Generation,
) (bool, error) {
	if err := generation.Validate(); err != nil {
		return false, errors.WrapResource(
			"verify",
			"remote catalog generation",
			generation.Manifest.GenerationID,
			err,
		)
	}
	candidateID := generation.Manifest.GenerationID
	candidateDigest := generation.Manifest.Payload.Checksum
	candidateGeneratedAt := generation.Manifest.GeneratedAt

	s.activationMu.Lock()
	defer s.activationMu.Unlock()
	active := s.State()
	switch {
	case candidateID == active.GenerationID &&
		candidateDigest != active.PayloadChecksum:
		return false, &errors.ConflictError{
			Resource: "remote catalog generation",
			Expected: active.PayloadChecksum,
			Actual:   candidateDigest,
			Message:  "one generation ID cannot identify different payloads",
		}
	case !s.identityEstablished:
	case candidateID == active.GenerationID:
		s.identityEstablished = true
		return false, nil
	case candidateGeneratedAt.Before(active.GeneratedAt):
		return false, nil
	case candidateGeneratedAt.Equal(active.GeneratedAt) &&
		candidateDigest != active.PayloadChecksum:
		return false, &errors.ConflictError{
			Resource: "remote catalog generation order",
			Expected: "a strictly newer generated_at value",
			Actual:   candidateGeneratedAt.Format(time.RFC3339Nano),
			Message:  "distinct payloads cannot share the active generation timestamp",
		}
	}

	publication, err := s.client.Activate(ctx, generation)
	if err != nil {
		return false, err
	}
	s.identityEstablished = true
	return publication.Published, nil
}

func (s *Subscriber) isActiveGeneration(id string) bool {
	s.activationMu.Lock()
	defer s.activationMu.Unlock()
	return id != "" && id == s.State().GenerationID
}

func (s *Subscriber) activeGenerationID() string {
	s.activationMu.Lock()
	defer s.activationMu.Unlock()
	return s.State().GenerationID
}

func (s *Subscriber) recordEventID(id string) {
	s.mu.Lock()
	s.lastEventID = id
	s.mu.Unlock()
}

func (s *Subscriber) currentLastEventID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastEventID
}

func waitRetry(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s lifecycleState) String() string {
	switch s {
	case stateIdle:
		return "idle"
	case stateStarting:
		return "starting"
	case stateRunning:
		return "running"
	case stateStopped:
		return "stopped"
	default:
		return "unknown(" + strconv.Itoa(int(s)) + ")"
	}
}
