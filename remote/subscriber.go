package remote

import (
	"context"
	"io"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
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
	protocol *catalogremote.Client
	client   *starmap.Client

	mu          sync.Mutex
	state       lifecycleState
	cancel      context.CancelFunc
	done        chan struct{}
	lastEventID string
	fallback    PollingFallbackStatus

	activationMu sync.Mutex
	active       generationIdentity

	retryDelay func(int) time.Duration
}

type generationIdentity struct {
	id          string
	digest      string
	generatedAt time.Time
}

// PollingFallbackStatus is an immutable snapshot of the subscriber's bounded
// polling fallback. Counters are cumulative for the subscriber lifetime.
type PollingFallbackStatus struct {
	// Enabled reports whether construction configured a polling fallback.
	Enabled bool
	// Active reports that the stream failure threshold has been reached and
	// streaming has not yet recovered.
	Active bool
	// Entries counts transitions into fallback mode.
	Entries uint64
	// Polls counts conditional current-manifest requests.
	Polls uint64
	// Modified counts verified non-304 responses handled by fallback polling.
	Modified uint64
}

// New validates config and constructs an idle subscriber. It starts no
// goroutine and performs no remote request.
func New(config Config) (*Subscriber, error) {
	normalized, err := config.normalized()
	if err != nil {
		return nil, err
	}
	protocol, err := catalogremote.NewClient(
		normalized.BaseURL,
		normalized.HTTPClient,
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		return nil, err
	}
	client, err := starmap.New(
		starmap.WithCatalogStore(catalogstore.NewMemory()),
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
		config:   normalized,
		protocol: protocol,
		client:   client,
		state:    stateIdle,
		fallback: PollingFallbackStatus{
			Enabled: normalized.PollingFallback != nil,
		},
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

// Catalog returns the current immutable catalog. Before Start succeeds it is
// the verified embedded bootstrap; afterward it is the latest activated remote
// generation.
func (s *Subscriber) Catalog() *catalogs.Catalog {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Catalog()
}

// Start performs a verified initial fetch and starts the caller-context-owned
// lifecycle. Normally it establishes the event stream and closes the
// fetch-to-subscribe gap with a mandatory current-state catch-up before
// returning. When an explicit PollingFallbackPolicy is configured, an initial
// stream failure starts bounded fallback/reconnect recovery instead.
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
			s.cancel = nil
			s.done = nil
		}
		s.mu.Unlock()
		close(done)
	}()

	initial, err := s.protocol.FetchCurrent(runCtx)
	if err != nil {
		return err
	}
	if _, err := s.activate(runCtx, initial); err != nil {
		return err
	}

	stream, err := s.protocol.OpenEventStream(runCtx, "")
	if err != nil {
		if runErr := runCtx.Err(); runErr != nil {
			return runErr
		}
		if s.config.PollingFallback == nil {
			return err
		}
		s.mu.Lock()
		if s.state != stateStarting {
			actual := s.state.String()
			s.mu.Unlock()
			return &errors.ConflictError{
				Resource: "remote catalog subscriber",
				Expected: "starting",
				Actual:   actual,
				Message:  "subscriber lifecycle changed while Start was initializing",
			}
		}
		s.state = stateRunning
		started = true
		go s.run(runCtx, nil, done, 1)
		s.mu.Unlock()
		return nil
	}
	if err := s.catchUp(runCtx); err != nil {
		_ = stream.Close()
		return err
	}

	s.mu.Lock()
	if s.state != stateStarting {
		actual := s.state.String()
		s.mu.Unlock()
		_ = stream.Close()
		return &errors.ConflictError{
			Resource: "remote catalog subscriber",
			Expected: "starting",
			Actual:   actual,
			Message:  "subscriber lifecycle changed while Start was initializing",
		}
	}
	s.state = stateRunning
	started = true
	go s.run(runCtx, stream, done, 0)
	s.mu.Unlock()
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
		s.mu.Unlock()
		return nil
	}
	if s.state == stateStopped && done == nil {
		s.mu.Unlock()
		return nil
	}
	s.state = stateStopped
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
	stream *catalogremote.EventStream,
	done chan struct{},
	streamFailures int,
) {
	defer func() {
		if stream != nil {
			_ = stream.Close()
		}
		s.mu.Lock()
		s.state = stateStopped
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
			_ = err // P7.11 exposes terminal/retry health without changing recovery.
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
			if !waitRetry(ctx, s.retryDelay(attempt)) {
				return
			}

			lastEventID := s.currentLastEventID()
			next, openErr := s.protocol.OpenEventStream(ctx, lastEventID)
			if openErr != nil {
				attempt++
				streamFailures++
				continue
			}
			// Replay is only an optimization. Every established connection
			// performs a verified current-state fetch before event consumption
			// resumes.
			if catchUpErr := s.catchUp(ctx); catchUpErr != nil {
				_ = next.Close()
				attempt++
				streamFailures++
				continue
			}
			s.finishPollingFallback()
			stream = next
			attempt = 0
			streamFailures = 0
			nextPoll = time.Time{}
			break
		}
	}
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
	now := time.Now()
	if !nextPoll.IsZero() && now.Before(*nextPoll) {
		return ctx.Err() == nil
	}

	modified, err := s.pollCurrent(ctx)
	if ctx.Err() != nil {
		return false
	}
	*nextPoll = time.Now().Add(policy.Interval)
	s.recordFallbackPoll(modified)
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
	stream *catalogremote.EventStream,
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
				continue
			}
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
	event catalogremote.StreamEvent
	err   error
}

func readEventStream(
	ctx context.Context,
	stream *catalogremote.EventStream,
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
		return err
	}
	_, err = s.activate(ctx, generation)
	return err
}

func (s *Subscriber) activate(
	ctx context.Context,
	generation catalogstore.Generation,
) (bool, error) {
	if err := generation.Validate(); err != nil {
		return false, errors.WrapResource(
			"verify",
			"remote catalog generation",
			generation.Manifest.GenerationID,
			err,
		)
	}
	candidate := generationIdentity{
		id:          generation.Manifest.GenerationID,
		digest:      generation.Manifest.Payload.Checksum,
		generatedAt: generation.Manifest.GeneratedAt,
	}

	s.activationMu.Lock()
	defer s.activationMu.Unlock()
	switch {
	case s.active.id == "":
	case candidate.id == s.active.id:
		if candidate.digest != s.active.digest {
			return false, &errors.ConflictError{
				Resource: "remote catalog generation",
				Expected: s.active.digest,
				Actual:   candidate.digest,
				Message:  "one generation ID cannot identify different payloads",
			}
		}
		return false, nil
	case candidate.generatedAt.Before(s.active.generatedAt):
		return false, nil
	case candidate.generatedAt.Equal(s.active.generatedAt) &&
		candidate.digest != s.active.digest:
		return false, &errors.ConflictError{
			Resource: "remote catalog generation order",
			Expected: "a strictly newer generated_at value",
			Actual:   candidate.generatedAt.Format(time.RFC3339Nano),
			Message:  "distinct payloads cannot share the active generation timestamp",
		}
	case candidate.digest == s.active.digest:
		// The remote source published a newer identity for identical canonical
		// bytes. Advance deduplication state without copying or republishing the
		// unchanged immutable catalog.
		s.active = candidate
		return false, nil
	}

	publication, err := s.client.Activate(ctx, generation)
	if err != nil {
		return false, err
	}
	s.active = candidate
	return publication.Published, nil
}

func (s *Subscriber) isActiveGeneration(id string) bool {
	s.activationMu.Lock()
	defer s.activationMu.Unlock()
	return id != "" && id == s.active.id
}

func (s *Subscriber) activeGenerationID() string {
	s.activationMu.Lock()
	defer s.activationMu.Unlock()
	return s.active.id
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
