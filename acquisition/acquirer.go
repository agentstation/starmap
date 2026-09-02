package acquisition

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// DefaultCoalesceWindow bounds how long a completed provider observation waits
// for a slower sibling. The window keeps one publication for a run that
// finishes together, and it bounds the wait for a run that does not.
const DefaultCoalesceWindow = 30 * time.Second

// ProviderObservation is the terminal result of one provider attempt. A
// skipped or failed attempt carries no layer.
type ProviderObservation struct {
	// Layer is the observed provider catalog. It is empty unless the attempt
	// succeeded.
	Layer starmap.ProviderLayer

	// Attempt is the terminal attempt record of the provider.
	Attempt sources.ProviderAttempt
}

// ProviderObserver observes exactly one provider. The runtime acquirer runs
// one observation per provider, so a slow provider never holds the records of
// a provider that already answered.
type ProviderObserver interface {
	ObserveProvider(
		ctx context.Context,
		current *catalogs.Catalog,
		id catalogs.ProviderID,
	) (ProviderObservation, error)
}

// Acquirer observes providers for a connected runtime. It publishes every
// completed observation through one bounded coalescing window, so a partial
// failure still moves the catalog forward.
type Acquirer struct {
	observer ProviderObserver
	window   time.Duration
	now      func() time.Time
	after    func(time.Duration) <-chan time.Time
}

// AcquirerOption configures the runtime acquirer.
type AcquirerOption func(*Acquirer) error

// WithProviderObserver replaces the concrete per-provider observation. Use it
// for restricted deployments and for deterministic tests.
func WithProviderObserver(observer ProviderObserver) AcquirerOption {
	return func(a *Acquirer) error {
		if observer == nil {
			return &errors.ValidationError{
				Field: "acquisition.provider_observer", Message: "is required",
			}
		}
		a.observer = observer
		return nil
	}
}

// WithAcquirerCoalesceWindow bounds how long a completed observation waits for
// a slower sibling.
func WithAcquirerCoalesceWindow(window time.Duration) AcquirerOption {
	return func(a *Acquirer) error {
		if window <= 0 {
			return &errors.ValidationError{
				Field: "acquisition.coalesce_window", Value: window, Message: "must be positive",
			}
		}
		a.window = window
		return nil
	}
}

// WithAcquirerClock injects the clock that stamps every attempt.
func WithAcquirerClock(now func() time.Time) AcquirerOption {
	return func(a *Acquirer) error {
		if now == nil {
			return &errors.ValidationError{
				Field: "acquisition.clock", Message: "is required",
			}
		}
		a.now = now
		return nil
	}
}

// WithAcquirerCoalesceTimer injects the timer that closes the coalescing
// window. A test drives the window without a real delay.
func WithAcquirerCoalesceTimer(after func(time.Duration) <-chan time.Time) AcquirerOption {
	return func(a *Acquirer) error {
		if after == nil {
			return &errors.ValidationError{
				Field: "acquisition.coalesce_timer", Message: "is required",
			}
		}
		a.after = after
		return nil
	}
}

// NewAcquirer returns the concrete runtime acquirer. It starts no goroutine
// and reaches no provider until a run starts.
func NewAcquirer(opts ...AcquirerOption) (*Acquirer, error) {
	acquirer := &Acquirer{
		observer: newProviderSourceObserver(),
		window:   DefaultCoalesceWindow,
		now:      time.Now,
		after:    time.After,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(acquirer); err != nil {
			return nil, err
		}
	}
	return acquirer, nil
}

// AcquireProviders observes every eligible provider concurrently. It returns as
// soon as
// every provider answers, or as soon as the coalescing window closes after the
// first answer. A provider that did not answer inside the window keeps its own
// retained layer, so one blocked provider never removes records.
func (a *Acquirer) AcquireProviders(
	ctx context.Context,
	request starmap.AcquisitionRequest,
) (starmap.AcquisitionResult, error) {
	if a == nil || a.observer == nil {
		return starmap.AcquisitionResult{}, &errors.ValidationError{
			Field: "acquisition.acquirer", Message: "is required",
		}
	}
	eligible := eligibleProviders(request)
	result := starmap.AcquisitionResult{Eligible: len(eligible)}
	if len(eligible) == 0 {
		return result, nil
	}

	// The run owns its own cancellation. Leaving the run stops every provider
	// that still works, so a blocked provider frees its resources.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The channel holds one result per provider, so a late answer never blocks
	// the goroutine that produces it.
	answers := make(chan ProviderObservation, len(eligible))
	for _, id := range eligible {
		go func() {
			answers <- a.observe(runCtx, request.Current, id)
		}()
	}

	answered := make(map[catalogs.ProviderID]bool, len(eligible))
	var window <-chan time.Time
	for len(answered) < len(eligible) {
		select {
		case observation := <-answers:
			answered[observation.Attempt.ProviderID] = true
			result.Attempts = append(result.Attempts, observation.Attempt)
			if len(observation.Layer.Payload) > 0 {
				result.Layers = append(result.Layers, observation.Layer)
			}
			if window == nil {
				// The first answer opens the bounded window. Every later answer
				// joins the same publication while the window stays open.
				window = a.after(a.coalesceWindow(request))
			}
		case <-window:
			logging.Warn().
				Str("run_id", request.RunID).
				Int("answered", len(answered)).
				Int("eligible", len(eligible)).
				Msg("Acquisition published before every provider answered")
			return a.close(result, eligible, answered), nil
		case <-runCtx.Done():
			return a.close(result, eligible, answered), runCtx.Err()
		}
	}
	return a.close(result, eligible, answered), nil
}

// observe runs one provider attempt and turns every failure into a terminal
// attempt record, so one provider error never fails the whole run.
func (a *Acquirer) observe(
	ctx context.Context,
	current *catalogs.Catalog,
	id catalogs.ProviderID,
) ProviderObservation {
	started := a.now()
	observation, err := a.observer.ObserveProvider(ctx, current, id)
	if err != nil {
		return ProviderObservation{Attempt: sources.ProviderAttempt{
			ProviderID:  id,
			Outcome:     sources.ProviderOutcomeFailed,
			Reason:      sources.ClassifyProviderReason(err),
			Requested:   true,
			StartedAt:   started,
			CompletedAt: a.now(),
		}}
	}
	observation.Attempt.ProviderID = id
	if observation.Attempt.StartedAt.IsZero() {
		observation.Attempt.StartedAt = started
	}
	if observation.Attempt.CompletedAt.IsZero() {
		observation.Attempt.CompletedAt = a.now()
	}
	observation.Layer.ProviderID = id
	return observation
}

// close records one terminal attempt for every provider that did not answer
// inside the window. The run reports the provider, and the runtime keeps the
// retained layer of that provider.
func (a *Acquirer) close(
	result starmap.AcquisitionResult,
	eligible []catalogs.ProviderID,
	answered map[catalogs.ProviderID]bool,
) starmap.AcquisitionResult {
	now := a.now()
	for _, id := range eligible {
		if answered[id] {
			continue
		}
		result.Attempts = append(result.Attempts, sources.ProviderAttempt{
			ProviderID:  id,
			Outcome:     sources.ProviderOutcomeFailed,
			Reason:      sources.ProviderReasonRequestTimeout,
			Requested:   true,
			CompletedAt: now,
		})
	}
	slices.SortFunc(result.Attempts, func(left, right sources.ProviderAttempt) int {
		return strings.Compare(string(left.ProviderID), string(right.ProviderID))
	})
	slices.SortFunc(result.Layers, func(left, right starmap.ProviderLayer) int {
		return strings.Compare(string(left.ProviderID), string(right.ProviderID))
	})
	return result
}

// coalesceWindow returns the window the run uses. The request wins, so one
// deployment setting reaches both the runtime and the acquirer.
func (a *Acquirer) coalesceWindow(request starmap.AcquisitionRequest) time.Duration {
	if request.CoalesceWindow > 0 {
		return request.CoalesceWindow
	}
	return a.window
}

// eligibleProviders returns the providers one run observes, in stable order.
func eligibleProviders(request starmap.AcquisitionRequest) []catalogs.ProviderID {
	if len(request.Providers) > 0 {
		selected := slices.Clone(request.Providers)
		slices.Sort(selected)
		return slices.Compact(selected)
	}
	if request.Current == nil {
		return nil
	}
	providerList := request.Current.Providers().List()
	selected := make([]catalogs.ProviderID, 0, len(providerList))
	for _, provider := range providerList {
		selected = append(selected, provider.ID)
	}
	slices.Sort(selected)
	return slices.Compact(selected)
}

// providerSourceObserver observes one provider through the concrete provider
// source. It holds the repository provider clients, so a deployment that
// imports starmap alone still imports no provider SDK.
type providerSourceObserver struct {
	options []providers.SourceOption
}

// newProviderSourceObserver returns the default per-provider observation.
func newProviderSourceObserver() *providerSourceObserver {
	return &providerSourceObserver{options: []providers.SourceOption{
		providers.WithClientFactory(defaultProviderClientFactory),
		providers.WithCredentialResolver(auth.NewResolver()),
	}}
}

// ObserveProvider observes exactly one provider and encodes its records as one
// retained layer.
func (o *providerSourceObserver) ObserveProvider(
	ctx context.Context,
	current *catalogs.Catalog,
	id catalogs.ProviderID,
) (ProviderObservation, error) {
	scoped, err := scopeToProvider(current, id)
	if err != nil {
		return ProviderObservation{}, err
	}
	source := providers.New(scoped, o.options...)
	observed, attempts, err := source.ObserveAttempts(ctx)
	if err != nil {
		return ProviderObservation{}, err
	}
	result := ProviderObservation{}
	if len(attempts) > 0 {
		result.Attempt = attempts[0]
	}
	if result.Attempt.Outcome != sources.ProviderOutcomeSucceeded || observed.Catalog == nil {
		return result, nil
	}
	payload, err := catalogs.EncodeCatalogPayload(observed.Catalog)
	if err != nil {
		return ProviderObservation{}, errors.WrapResource(
			"encode", "provider observation", string(id), err)
	}
	result.Layer = starmap.ProviderLayer{
		ProviderID: id,
		Payload:    payload,
		Digest:     catalogs.DescribeCatalogPayload(payload).Checksum,
		ObservedAt: observed.ObservedAt,
	}
	return result, nil
}

// scopeToProvider returns a provider reader that holds exactly one provider.
func scopeToProvider(
	current *catalogs.Catalog,
	id catalogs.ProviderID,
) (catalogs.ProvidersReader, error) {
	if current == nil {
		return nil, &errors.ValidationError{
			Field: "acquisition.current_catalog", Message: "is required",
		}
	}
	provider, found := current.Providers().Get(id)
	if !found {
		return nil, &errors.NotFoundError{Resource: "provider", ID: string(id)}
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(*provider); err != nil {
		return nil, errors.WrapResource("select", "provider", string(id), err)
	}
	return builder.Providers(), nil
}
