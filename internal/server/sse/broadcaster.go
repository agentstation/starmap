// Package sse provides the sole reactive catalog-publication transport.
package sse

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// CatalogPublishedEvent is the stable SSE event name.
	CatalogPublishedEvent = remote.CatalogPublishedEvent
	// DefaultHeartbeatInterval keeps idle streams alive through common proxies.
	DefaultHeartbeatInterval = 20 * time.Second
	// DefaultWriteTimeout bounds each event or heartbeat write and flush. The
	// broadcaster resets the deadline before every frame, so the value bounds
	// one frame and never bounds the stream. Two minutes lets a slow reader on
	// a congested link keep its subscription.
	DefaultWriteTimeout = 2 * time.Minute

	publicationQueueSize = 1
)

// Config controls per-connection SSE liveness and write behavior.
type Config struct {
	HeartbeatInterval time.Duration
	WriteTimeout      time.Duration
}

// Publication identifies one committed immutable catalog generation.
type Publication = remote.Publication

// DeliveryStats is a lock-free snapshot of SSE delivery behavior.
type DeliveryStats struct {
	Published              uint64 `json:"published"`
	Sent                   uint64 `json:"sent"`
	Heartbeats             uint64 `json:"heartbeats"`
	Disconnected           uint64 `json:"disconnected"`
	BackpressureTerminated uint64 `json:"backpressure_terminated"`
	Failed                 uint64 `json:"failed"`
}

// StreamState is the server-side publication stream state.
type StreamState string

const (
	// StreamStateIdle means the broadcaster accepts streams but has no clients.
	StreamStateIdle StreamState = "idle"
	// StreamStateStreaming means the broadcaster has active clients.
	StreamStateStreaming StreamState = "streaming"
	// StreamStateStopped means the broadcaster rejects new streams.
	StreamStateStopped StreamState = "stopped"
)

// DeliveryError classifies the latest stream failure without exposing secrets.
type DeliveryError struct {
	Kind       string    `json:"kind"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Health reports server-side SSE publication delivery without conflating
// heartbeat liveness with catalog freshness.
type Health struct {
	State            StreamState    `json:"state"`
	Clients          int            `json:"clients"`
	LastHeartbeatAt  time.Time      `json:"last_heartbeat_at"`
	LastEventAt      time.Time      `json:"last_event_at"`
	LastGenerationID string         `json:"last_generation_id,omitempty"`
	LastSequence     uint64         `json:"last_sequence"`
	LastError        *DeliveryError `json:"last_error,omitempty"`
	Delivery         DeliveryStats  `json:"delivery"`
}

// Broadcaster delivers publications to HTTP SSE connections. Each connection
// owns exactly one writer goroutine: its request handler. Publication overload
// terminates that connection so reconnect catch-up can recover without a
// silently healthy stream.
type Broadcaster struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	closed  bool
	config  Config
	logger  *zerolog.Logger

	published              atomic.Uint64
	sent                   atomic.Uint64
	heartbeats             atomic.Uint64
	disconnected           atomic.Uint64
	backpressureTerminated atomic.Uint64
	failed                 atomic.Uint64
	lastHeartbeatAt        atomic.Int64
	lastEventAt            atomic.Int64

	healthMu         sync.Mutex
	lastGenerationID string
	lastSequence     uint64
	lastError        *DeliveryError
	now              func() time.Time
}

type client struct {
	publications chan Publication
	done         chan struct{}
	stopOnce     sync.Once
}

func newClient() *client {
	return &client{
		publications: make(chan Publication, publicationQueueSize),
		done:         make(chan struct{}),
	}
}

func (c *client) stop() {
	c.stopOnce.Do(func() { close(c.done) })
}

func (c *client) offer(publication Publication) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case <-c.done:
		return false
	case c.publications <- publication:
		return true
	default:
		return false
	}
}

// NewBroadcaster constructs an idle broadcaster. It starts no goroutine.
func NewBroadcaster(config Config, logger *zerolog.Logger) (*Broadcaster, error) {
	if logger == nil {
		return nil, &errors.ValidationError{
			Field: "sse.logger", Message: "is required",
		}
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = DefaultWriteTimeout
	}
	if config.HeartbeatInterval < 0 {
		return nil, &errors.ValidationError{
			Field: "sse.heartbeat_interval", Value: config.HeartbeatInterval,
			Message: "must be positive",
		}
	}
	if config.WriteTimeout < 0 {
		return nil, &errors.ValidationError{
			Field: "sse.write_timeout", Value: config.WriteTimeout,
			Message: "must be positive",
		}
	}
	return &Broadcaster{
		clients: make(map[*client]struct{}),
		config:  config,
		logger:  logger,
	}, nil
}

// Publish offers one committed generation to every connected stream. It never
// blocks the catalog commit path. Publish terminates a connection that cannot
// accept the generation, which prevents silent data loss.
func (b *Broadcaster) Publish(publication Publication) error {
	if publication.GenerationID == "" {
		return &errors.ValidationError{
			Field: "sse.publication.generation_id", Message: "is required",
		}
	}
	if publication.Sequence == 0 {
		return &errors.ValidationError{
			Field: "sse.publication.sequence", Message: "must be positive",
		}
	}

	b.published.Add(1)
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	clients := make([]*client, 0, len(b.clients))
	for connection := range b.clients {
		clients = append(clients, connection)
	}
	b.mu.Unlock()

	for _, connection := range clients {
		if connection.offer(publication) {
			continue
		}
		if b.disconnect(connection) {
			b.backpressureTerminated.Add(1)
			b.recordDeliveryError("backpressure")
			b.logger.Warn().
				Uint64("sequence", publication.Sequence).
				Str("generation_id", publication.GenerationID).
				Msg("SSE connection terminated after publication backpressure")
		}
	}
	return nil
}

// ClientCount returns the number of currently registered SSE connections.
func (b *Broadcaster) ClientCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients)
}

// Stats returns cumulative delivery counters.
func (b *Broadcaster) Stats() DeliveryStats {
	return DeliveryStats{
		Published:              b.published.Load(),
		Sent:                   b.sent.Load(),
		Heartbeats:             b.heartbeats.Load(),
		Disconnected:           b.disconnected.Load(),
		BackpressureTerminated: b.backpressureTerminated.Load(),
		Failed:                 b.failed.Load(),
	}
}

// Health returns the current server-side stream delivery health.
func (b *Broadcaster) Health() Health {
	if b == nil {
		return Health{State: StreamStateStopped}
	}
	b.mu.Lock()
	state := StreamStateIdle
	if b.closed {
		state = StreamStateStopped
	} else if len(b.clients) != 0 {
		state = StreamStateStreaming
	}
	clients := len(b.clients)
	b.mu.Unlock()

	b.healthMu.Lock()
	health := Health{
		State:            state,
		Clients:          clients,
		LastGenerationID: b.lastGenerationID,
		LastSequence:     b.lastSequence,
		Delivery:         b.Stats(),
	}
	if b.lastError != nil {
		lastError := *b.lastError
		health.LastError = &lastError
	}
	b.healthMu.Unlock()
	health.LastHeartbeatAt = timestamp(b.lastHeartbeatAt.Load())
	health.LastEventAt = timestamp(b.lastEventAt.Load())
	return health
}

// Close terminates every active connection and rejects new ones.
func (b *Broadcaster) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	clients := make([]*client, 0, len(b.clients))
	for connection := range b.clients {
		clients = append(clients, connection)
		delete(b.clients, connection)
	}
	b.mu.Unlock()
	for _, connection := range clients {
		connection.stop()
		b.disconnected.Add(1)
	}
}

func (b *Broadcaster) register(connection *client) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	b.clients[connection] = struct{}{}
	return true
}

func (b *Broadcaster) disconnect(connection *client) bool {
	b.mu.Lock()
	_, found := b.clients[connection]
	if found {
		delete(b.clients, connection)
	}
	b.mu.Unlock()
	if found {
		connection.stop()
		b.disconnected.Add(1)
	}
	return found
}

// ServeHTTP serves one heartbeat-enabled SSE connection.
func (b *Broadcaster) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := writer.(http.Flusher); !ok {
		http.Error(writer, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	controller := http.NewResponseController(writer)
	if err := controller.SetWriteDeadline(time.Now().Add(b.config.WriteTimeout)); err != nil {
		http.Error(writer, "Streaming write deadlines not supported", http.StatusInternalServerError)
		return
	}

	connection := newClient()
	if !b.register(connection) {
		http.Error(writer, "Streaming unavailable", http.StatusServiceUnavailable)
		return
	}
	defer b.disconnect(connection)

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache, no-transform")
	writer.Header().Set("X-Accel-Buffering", "no")

	if err := b.writeFrame(writer, controller, []byte(": connected\n\n")); err != nil {
		b.recordWriteFailure(err)
		return
	}

	heartbeat := time.NewTicker(b.config.HeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case <-connection.done:
			return
		case publication := <-connection.publications:
			select {
			case <-connection.done:
				return
			default:
			}
			frame, err := encodePublication(publication)
			if err != nil {
				b.recordWriteFailure(err)
				return
			}
			if err := b.writeFrame(writer, controller, frame); err != nil {
				b.recordWriteFailure(err)
				return
			}
			b.sent.Add(1)
			b.recordPublication(publication)
		case <-heartbeat.C:
			if err := b.writeFrame(writer, controller, []byte(": heartbeat\n\n")); err != nil {
				b.recordWriteFailure(err)
				return
			}
			b.heartbeats.Add(1)
			b.lastHeartbeatAt.Store(b.currentTime().UnixNano())
		}
	}
}

func (b *Broadcaster) writeFrame(
	writer http.ResponseWriter,
	controller *http.ResponseController,
	frame []byte,
) error {
	if err := controller.SetWriteDeadline(time.Now().Add(b.config.WriteTimeout)); err != nil {
		return err
	}
	if _, err := writer.Write(frame); err != nil {
		return err
	}
	return controller.Flush()
}

func (b *Broadcaster) recordWriteFailure(err error) {
	b.failed.Add(1)
	b.recordDeliveryError("write_failed")
	if !stderrors.Is(err, http.ErrServerClosed) {
		b.logger.Warn().Err(err).Msg("SSE connection write failed")
	}
}

func (b *Broadcaster) recordDeliveryError(kind string) {
	b.healthMu.Lock()
	b.lastError = &DeliveryError{
		Kind:       kind,
		OccurredAt: b.currentTime(),
	}
	b.healthMu.Unlock()
}

func (b *Broadcaster) recordPublication(publication Publication) {
	now := b.currentTime()
	b.lastEventAt.Store(now.UnixNano())
	b.healthMu.Lock()
	if publication.Sequence >= b.lastSequence {
		b.lastSequence = publication.Sequence
		b.lastGenerationID = publication.GenerationID
	}
	b.healthMu.Unlock()
}

func (b *Broadcaster) currentTime() time.Time {
	if b.now != nil {
		return b.now().UTC()
	}
	return time.Now().UTC()
}

func timestamp(unixNano int64) time.Time {
	if unixNano == 0 {
		return time.Time{}
	}
	return time.Unix(0, unixNano).UTC()
}

func encodePublication(publication Publication) ([]byte, error) {
	data, err := json.Marshal(publication)
	if err != nil {
		return nil, errors.WrapParse("json", "SSE catalog publication", err)
	}
	return []byte(fmt.Sprintf(
		"id: %s\nevent: %s\ndata: %s\n\n",
		strconv.FormatUint(publication.Sequence, 10),
		CatalogPublishedEvent,
		data,
	)), nil
}
