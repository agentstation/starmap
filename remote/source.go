package remote

import (
	"context"
	stderrors "errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/starmap"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// DefaultSourceIdentity is the safe identity of a cascaded Starmap source. It
// matches the identity the runtime source policy reports, so status, layers,
// and the fleet phase all name one source.
const DefaultSourceIdentity = "starmap_cascade"

// SourceConfig builds one cascaded Starmap source. It carries the subscriber
// configuration plus the identity rules that keep a cascade acyclic.
type SourceConfig struct {
	// Subscriber configures the reactive upstream consumer.
	Subscriber Config

	// Identity is the safe identity this source reports. Empty selects
	// DefaultSourceIdentity. It names no URL and no credential.
	Identity string

	// Self is the stable identity of the runtime that reads this source. A
	// chain that names it is a self reference, so the source rejects it. An
	// empty value disables the self check.
	Self string

	// Aliases are the other identities that name this same runtime, such as a
	// load-balancer name or a second deployment name. URL comparison alone
	// cannot detect an alias, so a deployment declares one here.
	Aliases []string

	// MaxHops bounds the accepted chain length, counting the serving upstream
	// as the first hop. Zero selects the protocol maximum.
	MaxHops int

	// MaxAge is the propagated channel age at which this source reports a
	// degraded upstream. Zero disables the age grade.
	MaxAge time.Duration
}

// Source adapts the reactive subscriber onto the runtime source role. The
// subscriber streams upstream publications, and each Read reports the current
// verified generation, the sanitized upstream chain, and the propagated
// channel time of the origin.
//
// The source owns one background lifecycle. Read starts it once, and Close
// stops it. A read context never bounds the stream, because one refresh run is
// far shorter than one subscription.
type Source struct {
	subscriber *Subscriber
	store      storage.Store
	identity   string
	self       string
	aliases    []string
	maxHops    int
	maxAge     time.Duration

	startOnce sync.Once
	startErr  error
	cancel    context.CancelFunc

	mu                sync.Mutex
	lastGenerationID  string
	lastChannelUpdate time.Time
}

// NewSource builds the cascaded Starmap source. It starts no goroutine and
// sends no request. The first Read starts the subscriber.
func NewSource(ctx context.Context, config SourceConfig) (*Source, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if config.Subscriber.CatalogStore == nil {
		config.Subscriber.CatalogStore = storage.NewMemory()
	}
	subscriber, err := NewContext(ctx, config.Subscriber)
	if err != nil {
		return nil, err
	}
	identity := strings.TrimSpace(config.Identity)
	if identity == "" {
		identity = DefaultSourceIdentity
	}
	maxHops := config.MaxHops
	if maxHops <= 0 || maxHops > protocol.MaxSourceChainHops {
		maxHops = protocol.MaxSourceChainHops
	}
	aliases := make([]string, 0, len(config.Aliases))
	for _, alias := range config.Aliases {
		if trimmed := strings.TrimSpace(alias); trimmed != "" {
			aliases = append(aliases, trimmed)
		}
	}
	return &Source{
		subscriber: subscriber,
		store:      config.Subscriber.CatalogStore,
		identity:   identity,
		self:       strings.TrimSpace(config.Self),
		aliases:    aliases,
		maxHops:    maxHops,
		maxAge:     config.MaxAge,
	}, nil
}

// Identity returns the safe identity of the cascaded source. It stays stable
// for the life of the source, because the retained layer identity depends on
// it.
func (s *Source) Identity() string { return s.identity }

// Health returns the subscriber's own transport health. It stays independent
// of the upstream-reported health that Read carries.
func (s *Source) Health() Health { return s.subscriber.Health() }

// Close stops the subscriber lifecycle. It is idempotent.
func (s *Source) Close() error {
	if s == nil {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	return s.subscriber.Close()
}

// Read reports the current upstream generation, the sanitized chain, and the
// propagated channel time. It rejects a self reference, an alias of itself,
// a repeated identity, and an excessive hop count before it reports a
// generation, so the runtime never activates a cyclic cascade.
func (s *Source) Read(ctx context.Context) (starmap.SourceRead, error) {
	if err := s.start(ctx); err != nil {
		return starmap.SourceRead{}, err
	}
	chain, chainErr := s.subscriber.protocol.FetchSourceChain(ctx)
	if chainErr != nil && !stderrors.Is(chainErr, errors.ErrNotFound) {
		var apiErr *errors.APIError
		if !stderrors.As(chainErr, &apiErr) || apiErr.StatusCode == 0 {
			return starmap.SourceRead{}, chainErr
		}
	}
	disclosed := chainErr == nil
	if disclosed {
		if err := s.acceptChain(chain); err != nil {
			return starmap.SourceRead{}, err
		}
	}

	now := s.subscriber.currentTime()
	read := starmap.SourceRead{
		Health:           s.gradeUpstream(chain, disclosed, now),
		Chain:            chainHops(chain, disclosed),
		ChannelUpdatedAt: chain.ChannelUpdatedAt,
	}
	if disclosed {
		s.subscriber.recordUpstream(UpstreamReport{
			Identity:         chain.Identity,
			Health:           chain.Health,
			UpstreamHealth:   chain.UpstreamHealth,
			GenerationID:     chain.GenerationID,
			ChannelUpdatedAt: chain.ChannelUpdatedAt,
			Hops:             len(chain.Hops),
			ObservedAt:       now,
		})
	}

	state := s.subscriber.State()
	if state.GenerationID == "" {
		return read, nil
	}
	s.mu.Lock()
	unchanged := state.GenerationID == s.lastGenerationID &&
		chain.ChannelUpdatedAt.Equal(s.lastChannelUpdate)
	s.mu.Unlock()
	if unchanged {
		return read, nil
	}

	generation, err := s.store.Current(ctx)
	if err != nil {
		return starmap.SourceRead{}, errors.WrapResource(
			"load", "cascaded catalog generation", state.GenerationID, err)
	}
	read.Changed = true
	read.Generation = generation
	read.PublishedAt = generation.Manifest.GeneratedAt
	if read.ChannelUpdatedAt.IsZero() {
		read.ChannelUpdatedAt = generation.Manifest.GeneratedAt
	}
	s.mu.Lock()
	s.lastGenerationID = state.GenerationID
	s.lastChannelUpdate = chain.ChannelUpdatedAt
	s.mu.Unlock()
	return read, nil
}

// start begins the subscriber lifecycle once. The lifetime outlives one
// refresh run, so the source detaches the cancellation of the read context and
// keeps its values.
func (s *Source) start(ctx context.Context) error {
	s.startOnce.Do(func() {
		lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
		s.cancel = cancel
		if err := s.subscriber.Start(lifetime); err != nil {
			cancel()
			s.startErr = err
		}
	})
	return s.startErr
}

// acceptChain rejects a cascade that names this runtime or repeats a node.
// URL comparison cannot detect a load-balancer or DNS alias, so the check runs
// on the disclosed identities and on the declared aliases.
func (s *Source) acceptChain(chain protocol.SourceChain) error {
	identities := chain.Identities()
	if len(identities) > s.maxHops {
		return &errors.ValidationError{
			Field:   "catalog_source.chain.hops",
			Value:   len(identities),
			Message: "exceeds the configured maximum hop count",
		}
	}
	seen := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		if s.namesSelf(identity) {
			return &errors.ConflictError{
				Resource: "catalog source chain",
				Expected: "a chain that excludes this runtime",
				Actual:   identity,
				Message:  "a Starmap source cannot read its own catalog",
			}
		}
		if _, repeated := seen[identity]; repeated {
			return &errors.ConflictError{
				Resource: "catalog source chain",
				Expected: "one entry per source identity",
				Actual:   identity,
				Message:  "a repeated identity is a cycle",
			}
		}
		seen[identity] = struct{}{}
	}
	return nil
}

// namesSelf reports whether an identity names this runtime or one of its
// declared aliases.
func (s *Source) namesSelf(identity string) bool {
	if identity == "" {
		return false
	}
	if s.self != "" && identity == s.self {
		return true
	}
	return slices.Contains(s.aliases, identity)
}

// gradeUpstream returns the health the upstream reported about itself. A
// missing disclosure and an excessive propagated channel age both reduce the
// grade, because the downstream then works with less evidence.
func (s *Source) gradeUpstream(
	chain protocol.SourceChain,
	disclosed bool,
	now time.Time,
) starmap.Health {
	if !disclosed {
		return starmap.HealthDegraded
	}
	health := chainHealth(worstChainHealth(chain))
	if s.maxAge <= 0 || chain.ChannelUpdatedAt.IsZero() {
		return health
	}
	// Every hop evaluates the propagated origin time, not its own last check,
	// so a stale origin degrades the whole cascade instead of one hop.
	if now.Sub(chain.ChannelUpdatedAt) > s.maxAge {
		return worseChainHealth(health, starmap.HealthDegraded)
	}
	return health
}

// chainHops returns the sanitized chain the runtime records, nearest hop
// first. The serving upstream is the first hop.
func chainHops(chain protocol.SourceChain, disclosed bool) []starmap.SourceHop {
	if !disclosed {
		return nil
	}
	hops := make([]starmap.SourceHop, 0, len(chain.Hops)+1)
	hops = append(hops, starmap.SourceHop{
		Identity:    chain.Identity,
		Health:      chainHealth(chain.Health),
		PublishedAt: chain.ChannelUpdatedAt,
		ObservedAt:  chain.ObservedAt,
	})
	for _, hop := range chain.Hops {
		hops = append(hops, starmap.SourceHop{
			Identity:    hop.Identity,
			Health:      chainHealth(hop.Health),
			PublishedAt: hop.PublishedAt,
			ObservedAt:  hop.ObservedAt,
		})
	}
	return hops
}

// worstChainHealth returns the most serious grade the document disclosed.
func worstChainHealth(chain protocol.SourceChain) string {
	worst := chain.Health
	for _, code := range append([]string{chain.UpstreamHealth}, hopHealth(chain)...) {
		if chainRank(code) > chainRank(worst) {
			worst = code
		}
	}
	return worst
}

func hopHealth(chain protocol.SourceChain) []string {
	codes := make([]string, 0, len(chain.Hops))
	for _, hop := range chain.Hops {
		codes = append(codes, hop.Health)
	}
	return codes
}

// chainHealth converts one closed-set chain code onto the runtime health.
func chainHealth(code string) starmap.Health {
	switch code {
	case protocol.SourceChainHealthOK:
		return starmap.HealthOK
	case protocol.SourceChainHealthDegraded:
		return starmap.HealthDegraded
	case protocol.SourceChainHealthUnavailable:
		return starmap.HealthUnavailable
	default:
		return starmap.HealthUnknown
	}
}

// ChainHealthCode converts one runtime health onto the closed chain code. A
// server uses it while it builds the document it serves.
func ChainHealthCode(health starmap.Health) string {
	switch health {
	case starmap.HealthOK:
		return protocol.SourceChainHealthOK
	case starmap.HealthDegraded:
		return protocol.SourceChainHealthDegraded
	case starmap.HealthUnavailable:
		return protocol.SourceChainHealthUnavailable
	default:
		return protocol.SourceChainHealthUnknown
	}
}

func chainRank(code string) int {
	switch code {
	case protocol.SourceChainHealthOK:
		return 0
	case protocol.SourceChainHealthDegraded:
		return 2
	case protocol.SourceChainHealthUnavailable:
		return 3
	default:
		return 1
	}
}

func worseChainHealth(left, right starmap.Health) starmap.Health {
	if chainRank(ChainHealthCode(right)) > chainRank(ChainHealthCode(left)) {
		return right
	}
	return left
}
