package remote

import (
	"context"
	stderrors "errors"
	"io"
	"math/rand/v2"
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

	activationMu        sync.Mutex
	identityEstablished bool

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
	protocol, err := protocol.NewClient(
		normalized.BaseURL,
		normalized.HTTPClient,
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
	}
	subscriber.retryDelay = subscriber.exponentialJitter
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
		if isTerminalRemoteError(err) {
			return err
		}
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
		if isTerminalRemoteError(err) {
			return err
		}
		if err := s.beginRun(runCtx, nil, done, 1, StreamStateRetrying); err != nil {
			return err
		}
		started = true
		return nil
	}
	if err := s.catchUp(runCtx); err != nil {
		_ = stream.Close()
		if runErr := runCtx.Err(); runErr != nil {
			return runErr
		}
		if isTerminalRemoteError(err) {
			return err
		}
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
			s.recordHealthError("stream_read", err)
			if isTerminalRemoteError(err) {
				return
			}
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
			s.setStreamState(StreamStateRetrying)
			s.recordRetry()
			if !waitRetry(ctx, s.retryDelay(attempt)) {
				return
			}

			lastEventID := s.currentLastEventID()
			next, openErr := s.protocol.OpenEventStream(ctx, lastEventID)
			if openErr != nil {
				s.recordHealthError("stream_open", openErr)
				if isTerminalRemoteError(openErr) {
					return
				}
				attempt++
				streamFailures++
				continue
			}
			// Replay only improves efficiency. After each connection, the
			// subscriber fetches and verifies current state before it reads events.
			if catchUpErr := s.catchUp(ctx); catchUpErr != nil {
				_ = next.Close()
				if isTerminalRemoteError(catchUpErr) {
					return
				}
				attempt++
				streamFailures++
				continue
			}
			s.finishPollingFallback()
			s.setStreamState(StreamStateStreaming)
			stream = next
			attempt = 0
			streamFailures = 0
			nextPoll = time.Time{}
			break
		}
	}
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
	now := time.Now()
	if !nextPoll.IsZero() && now.Before(*nextPoll) {
		return ctx.Err() == nil
	}

	modified, err := s.pollCurrent(ctx)
	s.recordHealthError("fallback_poll", err)
	if ctx.Err() != nil {
		return false
	}
	*nextPoll = time.Now().Add(policy.Interval)
	s.recordFallbackPoll(modified)
	if isTerminalRemoteError(err) {
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

func (s *Subscriber) exponentialJitter(attempt int) time.Duration {
	delay := s.config.ReconnectMinDelay
	for range min(attempt, 62) {
		if delay >= s.config.ReconnectMaxDelay/2 {
			delay = s.config.ReconnectMaxDelay
			break
		}
		delay *= 2
	}
	if delay > s.config.ReconnectMaxDelay {
		delay = s.config.ReconnectMaxDelay
	}
	half := delay / 2
	if half <= 0 {
		return delay
	}
	// Reconnect jitter is load spreading, not a security decision.
	return half + time.Duration(rand.Int64N(int64(delay-half)+1)) //nolint:gosec
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
