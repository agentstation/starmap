package runtime

import (
	"context"
	stderrors "errors"
	"github.com/agentstation/starmap"
	"slices"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/internal/sources/github"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// runKind names refresh and manual publication operations.
type runKind string

const (
	// runKindRefresh reads the upstream and then observes acquisition sources.
	runKindRefresh runKind = "refresh"

	// runKindSource reads the selected upstream source only.
	runKindSource runKind = "source"

	// runKindAcquisition observes configured acquisition sources.
	runKindAcquisition runKind = "acquisition"

	// Manual batches have distinct inputs and must never join another caller's run.
	runKindManual runKind = "manual"
)

// SourceRefreshReport says what one upstream source read produced.
type SourceRefreshReport struct {
	// RunID is the identity of the run that produced this report.
	RunID string

	// StartedAt and CompletedAt bound the run.
	StartedAt   time.Time
	CompletedAt time.Time

	// SourceIdentity is the safe identity of the source that answered.
	SourceIdentity string

	// Changed reports whether the upstream generation moved.
	Changed bool

	// Published reports whether the runtime published a new effective catalog.
	Published bool

	// GenerationID identifies the upstream generation the runtime retained.
	GenerationID string

	// PublishedAt is the upstream publication time.
	PublishedAt time.Time

	// Health is what this runtime observed while it read the source. It grades
	// the transfer only.
	Health Health

	// UpstreamHealth is the health the upstream reported about itself. It stays
	// independent of Health, so a healthy transfer still carries a degraded
	// upstream report.
	UpstreamHealth Health

	// Reason is the safe reason code of a failed read.
	Reason string

	// Chain is the sanitized upstream source chain.
	Chain []SourceHop
}

// AcquisitionReport says what one acquisition run produced. A partial
// failure still publishes: the report names the providers that kept their own
// last-known-good observation.
type AcquisitionReport struct {
	// RunID is the identity of the run that produced this report.
	RunID string

	// StartedAt and CompletedAt bound the run.
	StartedAt   time.Time
	CompletedAt time.Time

	// Eligible counts provider targets, or bindings when the active set is explicit.
	Eligible int

	// Succeeded, Skipped, and Failed count the terminal attempts.
	Succeeded int
	Skipped   int
	Failed    int

	// Attempts holds one terminal attempt per eligible provider or binding.
	Attempts []sources.ProviderAttempt

	// SourceObservations binds each non-provider result to its original receipt.
	SourceObservations []catalogs.SourceObservationLink

	// SourceActivities holds the reported selection and attempts for this run.
	SourceActivities []sources.SourceActivity

	// Published reports whether the runtime published a new effective catalog.
	Published bool

	// GenerationID identifies the published effective catalog.
	GenerationID string

	// Retained names the providers that kept their previous last-known-good
	// observation in at least one scope that this run did not replace.
	Retained []catalogs.ProviderID

	// Health grades the run. A failed provider degrades the run.
	Health Health
}

// RefreshReport says what one whole refresh produced. A source-only run leaves
// the acquisition report empty, and an acquisition-only run leaves the source
// report empty.
type RefreshReport struct {
	// RunID is the identity of the run.
	RunID string

	// Kind names the work the run did.
	Kind string

	// StartedAt and CompletedAt bound the run.
	StartedAt   time.Time
	CompletedAt time.Time

	// Source reports the upstream read.
	Source SourceRefreshReport

	// Acquisition reports provider attempts and non-provider observations.
	Acquisition AcquisitionReport

	// Published reports whether the runtime published a new effective catalog.
	Published bool

	// GenerationID identifies the published effective catalog.
	GenerationID string
}

// activeRun is one operation in flight. Equal refresh kinds share its result.
// Manual callers wait for completion and then start their own operation.
type activeRun struct {
	id   string
	kind runKind

	// epoch is the lease epoch the run started under. It fences every commit
	// the run makes, so a run that loses the lease cannot publish.
	epoch uint64

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	report RefreshReport
	err    error
}

// join waits for the run and returns its report. The caller keeps its own
// cancellation: leaving the join never cancels the run.
func (a *activeRun) join(ctx context.Context) (RefreshReport, error) {
	select {
	case <-a.done:
		return a.report, a.err
	case <-ctx.Done():
		return RefreshReport{}, errors.WrapResource("join", "refresh run", a.id, ctx.Err())
	}
}

// runGroup keeps refresh single-flight. One run of each kind exists at a time,
// and a caller of a different kind waits for the slot.
type runGroup struct {
	mu     sync.Mutex
	active *activeRun
	closed bool
}

// start returns the run this caller must use. The second result reports
// whether this caller owns the run and must execute it.
func (g *runGroup) start(
	ctx, parent context.Context,
	kind runKind,
	id string,
	epoch uint64,
) (*activeRun, bool, error) {
	for {
		g.mu.Lock()
		if g.closed {
			g.mu.Unlock()
			return nil, false, &errors.ConflictError{Resource: "runtime", Message: "runtime is closed"}
		}
		if g.active == nil {
			runCtx, cancel := context.WithCancel(parent)
			run := &activeRun{
				id:     id,
				kind:   kind,
				epoch:  epoch,
				ctx:    runCtx,
				cancel: cancel,
				done:   make(chan struct{}),
			}
			g.active = run
			g.mu.Unlock()
			return run, true, nil
		}
		existing := g.active
		g.mu.Unlock()
		if existing.kind == kind && kind != runKindManual {
			return existing, false, nil
		}
		select {
		case <-existing.done:
		case <-ctx.Done():
			return nil, false, errors.WrapResource("await", "refresh run", existing.id, ctx.Err())
		}
	}
}

// finish publishes the result of one run and frees the slot.
func (g *runGroup) finish(run *activeRun, report RefreshReport, err error) {
	g.mu.Lock()
	if g.active == run {
		g.active = nil
	}
	g.mu.Unlock()
	run.report = report
	run.err = err
	run.cancel()
	close(run.done)
}

// close refuses new runs and returns the completion signal of the current run.
func (g *runGroup) close() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed = true
	if g.active != nil {
		g.active.cancel()
		return g.active.done
	}
	done := make(chan struct{})
	close(done)
	return done
}

// cancelActive cancels the run in flight. The runtime calls it on close and
// after it loses the lease.
func (g *runGroup) cancelActive() {
	g.mu.Lock()
	run := g.active
	g.mu.Unlock()
	if run != nil {
		run.cancel()
	}
}

// Refresh reads the upstream and then observes configured acquisition sources.
// It changes the upstream layer and acquisition inputs in one run.
func (r *Runtime) Refresh(ctx context.Context) (RefreshReport, error) {
	return r.execute(ctx, runKindRefresh, func(runCtx context.Context, report *RefreshReport, epoch uint64) error {
		sourceErr := r.readSource(runCtx, report, epoch)
		if !r.hasAcquisition() {
			return sourceErr
		}
		acquireErr := r.acquire(runCtx, report, nil, epoch)
		return stderrors.Join(sourceErr, acquireErr)
	})
}

// RefreshSource reads the upstream source only. It changes the source layer.
func (r *Runtime) RefreshSource(ctx context.Context) (SourceRefreshReport, error) {
	report, err := r.execute(ctx, runKindSource, func(runCtx context.Context, report *RefreshReport, epoch uint64) error {
		return r.readSource(runCtx, report, epoch)
	})
	return report.Source, err
}

// Sync observes configured provider and non-provider sources.
// An empty provider list observes every eligible provider scope.
func (r *Runtime) Sync(ctx context.Context, providers ...catalogs.ProviderID) (AcquisitionReport, error) {
	report, err := r.execute(ctx, runKindAcquisition, func(runCtx context.Context, report *RefreshReport, epoch uint64) error {
		if !r.hasAcquisition() {
			return &errors.ConfigError{
				Component: "acquirer",
				Message:   "no configured acquisition source is eligible",
			}
		}
		return r.acquire(runCtx, report, providers, epoch)
	})
	return report.Acquisition, err
}

// execute joins operation lifetime to the runtime. Equal refresh kinds share a run.
// Manual calls wait for prior work because their input batches can differ.
func (r *Runtime) execute(
	ctx context.Context,
	kind runKind,
	work func(context.Context, *RefreshReport, uint64) error,
) (RefreshReport, error) {
	if r == nil {
		return RefreshReport{}, &errors.ValidationError{Field: "runtime", Message: "is required"}
	}
	if ctx == nil {
		return RefreshReport{}, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return RefreshReport{}, err
	}
	id, err := r.client.NextID()
	if err != nil {
		return RefreshReport{}, err
	}

	run, owner, err := r.runs.start(ctx, r.ctx, kind, id, 0)
	if err != nil {
		return RefreshReport{}, err
	}
	if !owner {
		return run.join(ctx)
	}

	// The run belongs to the runtime, not to one caller. Caller cancellation
	// still stops the run. A caller deadline adds no deadline of its own, so
	// the configured refresh timeout is the only deadline a run carries.
	stopPropagation := context.AfterFunc(ctx, run.cancel)
	defer stopPropagation()

	runCtx := run.ctx
	if timeout := r.config.refreshTimeout; timeout > 0 {
		bounded, cancel := context.WithTimeout(runCtx, timeout)
		defer cancel()
		runCtx = bounded
	}

	report := RefreshReport{RunID: run.id, Kind: string(kind), StartedAt: r.config.now()}
	// Directory ownership covers lease acquisition and all publication work.
	workErr := r.lease.ensureHeld(runCtx)
	if workErr == nil {
		run.epoch = r.lease.epoch()
		workErr = work(runCtx, &report, run.epoch)
	}
	report.CompletedAt = r.config.now()

	r.mu.Lock()
	r.report.lastRunID = run.id
	r.mu.Unlock()

	r.runs.finish(run, report, workErr)
	return report, workErr
}

// readSource reads the upstream source, retains the generation, and publishes
// the rebuilt effective catalog.
func (r *Runtime) readSource(ctx context.Context, report *RefreshReport, epoch uint64) error {
	source := r.source
	if source == nil {
		return &errors.ConfigError{Component: "catalog source", Message: "is not selected"}
	}
	result := SourceRefreshReport{
		RunID:          report.RunID,
		StartedAt:      r.config.now(),
		SourceIdentity: source.Identity(),
	}

	read, err := r.readWithRetry(ctx, source)
	if canceled := ctx.Err(); canceled != nil {
		err = stderrors.Join(err, canceled)
	}
	result.CompletedAt = r.config.now()
	if err != nil {
		result.Health = HealthUnavailable
		result.Reason = safeSourceReason(err)
		r.recordSourceRead(result, false)
		report.Source = result
		return err
	}

	// A cascade check runs before retention, so a loop never reaches the
	// served catalog. The runtime owns the check, because only the runtime
	// knows its own instance identity and its declared aliases.
	if chainErr := acceptSourceChain(
		r.schedule.identity.Instance,
		r.config.source.Aliases,
		r.config.source.MaxHops,
		read.Chain,
	); chainErr != nil {
		result.Health = HealthUnavailable
		result.Reason = chainRejected
		result.Chain = read.Chain
		r.recordSourceRead(result, false)
		report.Source = result
		return chainErr
	}

	// A completed read grades the transfer healthy. The upstream report stays
	// separate, so a degraded upstream never hides a working transfer, and a
	// working transfer never hides a degraded upstream.
	result.Health = HealthOK
	result.UpstreamHealth = orUnknown(read.Health)
	result.Chain = read.Chain
	result.Changed = read.Changed
	result.PublishedAt = read.PublishedAt
	if !read.Changed {
		r.recordSourceRead(result, true)
		report.Source = result
		return nil
	}

	layer := sourceLayer{
		Identity:         source.Identity(),
		GenerationID:     read.Generation.Manifest.GenerationID,
		Checksum:         read.Generation.Manifest.Payload.Checksum,
		Payload:          read.Generation.Payload,
		PublishedAt:      read.PublishedAt,
		ChannelUpdatedAt: read.ChannelUpdatedAt,
		ObservedAt:       result.CompletedAt,
		Chain:            read.Chain,
	}
	state, err := r.publishInputChanges(ctx, &layer, nil, epoch)
	if err != nil {
		result.Health = HealthDegraded
		result.Reason = "publication_failed"
		if state.GenerationID != "" {
			result.Reason = "retention_pending"
			result.Published = true
			result.GenerationID = state.GenerationID
			report.Published = true
			report.GenerationID = state.GenerationID
		}
		r.recordSourceRead(result, false)
		report.Source = result
		return err
	}
	result.Published = true
	result.GenerationID = state.GenerationID
	report.Published = true
	report.GenerationID = state.GenerationID
	r.recordSourceRead(result, true)
	report.Source = result
	return nil
}

// readWithRetry reads the source under the fleet retry policy. It honors a
// declared not-before boundary and warns while the request budget nears its
// bound, so a refused fleet does not retry at one instant.
func (r *Runtime) readWithRetry(ctx context.Context, source Source) (SourceRead, error) {
	policy := fleet.DefaultRetryPolicy()
	random := fleet.Random(r.config.random)
	delay := time.Duration(0)
	var lastErr error
	for retries := 0; ; retries++ {
		read, err := source.Read(ctx)
		if err == nil {
			return read, nil
		}
		lastErr = err
		if ctx.Err() != nil || !policy.Allows(retries) {
			return SourceRead{}, lastErr
		}

		var refusal *github.RefusalError
		if stderrors.As(err, &refusal) {
			if refusal.Budget.Warn() {
				logging.Warn().
					Int("used_percent", refusal.Budget.UsedPercent()).
					Str("resource", refusal.Resource).
					Msg("Catalog source request budget is nearly spent")
			}
			now := r.config.now()
			boundary := fleet.NotBefore(now, refusal.NotBefore, random)
			delay = boundary.Sub(now)
			if delay < 0 {
				delay = 0
			}
		} else {
			delay, err = policy.Next(delay, random)
			if err != nil {
				return SourceRead{}, err
			}
		}
		if !waitFor(ctx, delay) {
			return SourceRead{}, lastErr
		}
	}
}

// waitFor sleeps for the delay. It reports false when the context ended first.
func waitFor(ctx context.Context, delay time.Duration) bool {
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

// recordSourceRead stores what Status reports about the source layer.
func (r *Runtime) recordSourceRead(result SourceRefreshReport, reached bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.report.sourceCheckedAt = result.CompletedAt
	r.report.sourceHealth = result.Health
	r.report.sourceReason = result.Reason
	if reached {
		r.report.upstreamHealth = orUnknown(result.UpstreamHealth)
	}
	if len(result.Chain) > 0 {
		r.report.upstreamChain = result.Chain
		r.report.upstreamHealth = worseHealth(r.report.upstreamHealth, result.Chain[0].Health)
	}
	if reached && result.Changed {
		r.report.sourceChangedAt = result.CompletedAt
	}
}

// acquireProviders observes providers, retains every completed observation, and
// publishes. A failed provider keeps its own last-known-good layer, so one
// failure never removes records from the effective catalog.
func (r *Runtime) acquireProviders(
	ctx context.Context,
	report *RefreshReport,
	providers []catalogs.ProviderID,
	epoch uint64,
) error {
	result := AcquisitionReport{RunID: report.RunID, StartedAt: r.config.now()}
	current := r.State().Catalog

	// Each closed coalescing window publishes the layers it collected. The
	// acquirer calls this from its own run goroutine, one window at a time.
	windows := &windowPublisher{runtime: r, epoch: epoch}
	observed, attempted, err := r.acquireSelectedProviders(ctx, AcquisitionRequest{
		RunID:          report.RunID,
		Current:        current,
		Providers:      providers,
		CoalesceWindow: r.config.coalesceWindow,
		Publish:        windows.publish,
	})
	result.SourceActivities = []sources.SourceActivity{providerSourceActivity(observed, attempted, err)}
	result.CompletedAt = r.config.now()
	result.Eligible = observed.Eligible
	result.Attempts = observed.Attempts
	for _, attempt := range observed.Attempts {
		switch attempt.Outcome {
		case sources.ProviderOutcomeSucceeded:
			result.Succeeded++
		case sources.ProviderOutcomeSkippedNotConfigured:
			result.Skipped++
		case sources.ProviderOutcomeFailed:
			result.Failed++
		}
	}

	result.Health = HealthOK
	switch {
	case err != nil:
		result.Health = HealthUnavailable
	case result.Failed > 0 || result.Skipped > 0 || hasDegradedProviderReceipt(observed.Layers):
		result.Health = HealthDegraded
	}

	// The windows that closed inside the run already published their layers.
	// Only the rest needs one final publication.
	result.Published = windows.publications() > 0
	result.GenerationID = windows.generationID()
	if result.Published {
		report.Published = true
		report.GenerationID = result.GenerationID
	}
	prepared, validationErr := prepareProviderEvidence(observed.Layers)
	if validationErr != nil {
		result.Health = HealthDegraded
		combined := stderrors.Join(err, validationErr)
		report.Acquisition = result
		return combined
	}
	answered := make(map[providerEvidenceKey]bool, len(prepared))
	var unpublished []ProviderLayer
	for _, layer := range prepared {
		answered[layer.evidenceKey()] = true
		if windows.published(layer) {
			continue
		}
		unpublished = append(unpublished, layer)
	}

	// A partial failure still publishes. The layers that answered move forward
	// and the layers that did not keep their retained records.
	if len(unpublished) > 0 {
		state, publishErr := r.publishProviders(ctx, unpublished, epoch)
		if state.GenerationID != "" {
			result.Published = true
			result.GenerationID = state.GenerationID
			report.Published = true
			report.GenerationID = state.GenerationID
		}
		if publishErr != nil {
			result.Health = HealthDegraded
			combined := stderrors.Join(err, publishErr)
			report.Acquisition = result
			return combined
		}
		result.Published = true
		result.GenerationID = state.GenerationID
	}
	if result.Published {
		report.Published = true
		report.GenerationID = result.GenerationID
	}

	r.mu.RLock()
	for _, id := range r.layers.activeProviderOrder() {
		if !answered[id] {
			result.Retained = append(result.Retained, id.providerID)
		}
	}
	r.mu.RUnlock()
	result.Retained = slices.Compact(result.Retained)

	report.Acquisition = result
	return err
}

// recordAcquisition stores what Status reports about the last acquisition run.
func (r *Runtime) recordAcquisition(result AcquisitionReport, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.report.acquisitionStartedAt = result.StartedAt
	r.report.acquisitionHealth = result.Health
	r.report.attempts = slices.Clone(result.Attempts)
	r.report.sourceObservations = slices.Clone(result.SourceObservations)
	r.report.sourceActivities = slices.Clone(result.SourceActivities)
	if err == nil {
		r.report.acquisitionSucceededAt = result.CompletedAt
	}
}

// publishProviders retains the supplied layers and publishes one effective
// catalog that holds every retained layer.
func (r *Runtime) publishProviders(
	ctx context.Context,
	layers []ProviderLayer,
	epoch uint64,
) (starmap.CatalogState, error) {
	return r.publishInputChanges(ctx, nil, layers, epoch)
}

// windowPublisher publishes the layers of one closed coalescing window. It
// records what it published, so the run publishes each layer one time.
type windowPublisher struct {
	runtime *Runtime
	epoch   uint64

	mu         sync.Mutex
	seen       map[providerPublicationKey]bool
	count      int
	generation string
}

// publish retains and publishes the layers of one closed window.
func (w *windowPublisher) publish(ctx context.Context, layers []ProviderLayer) error {
	if len(layers) == 0 {
		return nil
	}
	prepared, err := prepareProviderEvidence(layers)
	if err != nil {
		return err
	}
	state, err := w.runtime.publishProviders(ctx, prepared, w.epoch)
	if err != nil && state.GenerationID == "" {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.seen == nil {
		w.seen = make(map[providerPublicationKey]bool, len(prepared))
	}
	for _, layer := range prepared {
		w.seen[publicationKey(layer)] = true
	}
	w.count++
	w.generation = state.GenerationID
	return err
}

// providerPublicationKey identifies exact evidence within one retained scope.
type providerPublicationKey struct {
	scope         providerEvidenceKey
	observationID string
}

func publicationKey(layer ProviderLayer) providerPublicationKey {
	return providerPublicationKey{scope: layer.evidenceKey(), observationID: layer.Receipt.Link.ObservationID}
}

// published reports whether a closed window published this validated observation.
func (w *windowPublisher) published(layer ProviderLayer) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.seen[publicationKey(layer)]
}

// publications returns how many windows the run published.
func (w *windowPublisher) publications() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

// generationID returns the last generation that a closed window published.
func (w *windowPublisher) generationID() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.generation
}

// safeSourceReason maps one source failure onto a safe reason code. The code
// names no URL, no host, and no credential. A refusal that declares a
// not-before boundary or a spent budget is a rate limit. GitHub answers a
// secondary rate limit with status 403 and a Retry-After header. A bare 403
// without either signal is a credential refusal.
func safeSourceReason(err error) string {
	var refusal *github.RefusalError
	if stderrors.As(err, &refusal) {
		switch {
		case refusal.Status == 429,
			!refusal.NotBefore.IsZero(),
			refusal.Budget.Exhausted():
			return string(sources.ProviderReasonRateLimited)
		default:
			return string(sources.ProviderReasonCredentialRejected)
		}
	}
	return string(sources.ClassifyProviderReason(err))
}

func hasDegradedProviderReceipt(layers []ProviderLayer) bool {
	for _, layer := range layers {
		if layer.Receipt.Link.Status == sources.ObservationStatusDegraded {
			return true
		}
	}
	return false
}
