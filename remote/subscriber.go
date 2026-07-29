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

	activationMu sync.Mutex
	active       generationIdentity

	retryDelay func(int) time.Duration
}

type generationIdentity struct {
	id          string
	digest      string
	generatedAt time.Time
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
	}
	subscriber.retryDelay = subscriber.exponentialJitter
	return subscriber, nil
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

// Start performs a verified initial fetch, establishes the event stream, closes
// the fetch-to-subscribe gap with a mandatory current-state catch-up, and then
// starts the caller-context-owned reconnect lifecycle.
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

	s.mu.Lock()
	if s.state != stateIdle {
		s.mu.Unlock()
		return &errors.ConflictError{
			Resource: "remote catalog subscriber",
			Expected: "idle",
			Actual:   s.state.String(),
			Message:  "Start may be called only once",
		}
	}
	s.mu.Unlock()

	initial, err := s.protocol.FetchCurrent(ctx)
	if err != nil {
		return err
	}
	if _, err := s.activate(ctx, initial); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	stream, err := s.protocol.OpenEventStream(runCtx, "")
	if err != nil {
		cancel()
		return err
	}
	if err := s.catchUp(runCtx); err != nil {
		_ = stream.Close()
		cancel()
		return err
	}

	s.mu.Lock()
	if s.state != stateIdle {
		s.mu.Unlock()
		_ = stream.Close()
		cancel()
		return &errors.ConflictError{
			Resource: "remote catalog subscriber",
			Expected: "idle",
			Actual:   s.state.String(),
			Message:  "subscriber lifecycle changed while Start was initializing",
		}
	}
	s.state = stateRunning
	s.cancel = cancel
	s.done = make(chan struct{})
	done := s.done
	go s.run(runCtx, stream, done)
	s.mu.Unlock()
	return nil
}

// Close cancels and joins the subscriber lifecycle. It is idempotent.
func (s *Subscriber) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	if s.state == stateIdle {
		s.state = stateStopped
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *Subscriber) run(
	ctx context.Context,
	stream *catalogremote.EventStream,
	done chan struct{},
) {
	defer func() {
		_ = stream.Close()
		s.mu.Lock()
		s.state = stateStopped
		s.mu.Unlock()
		close(done)
	}()

	attempt := 0
	for {
		err := s.consume(ctx, stream)
		_ = stream.Close()
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			err = io.EOF
		}
		_ = err // P7.11 exposes terminal/retry health without changing recovery.

		if !waitRetry(ctx, s.retryDelay(attempt)) {
			return
		}
		attempt++

		lastEventID := s.currentLastEventID()
		next, openErr := s.protocol.OpenEventStream(ctx, lastEventID)
		if openErr != nil {
			continue
		}
		// Replay is only an optimization. Every established connection performs
		// a verified current-state fetch before event consumption resumes.
		if catchUpErr := s.catchUp(ctx); catchUpErr != nil {
			_ = next.Close()
			continue
		}
		stream = next
		attempt = 0
	}
}

func (s *Subscriber) consume(
	ctx context.Context,
	stream *catalogremote.EventStream,
) error {
	for {
		event, err := stream.Next()
		if err != nil {
			return err
		}
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
	case stateRunning:
		return "running"
	case stateStopped:
		return "stopped"
	default:
		return "unknown(" + strconv.Itoa(int(s)) + ")"
	}
}
