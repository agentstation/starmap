package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/pkg/logging"
)

// Controller names of the periodic runtime workers.
const (
	controllerSource      = "source"
	controllerAcquisition = "acquisition"
	controllerAccepted    = "accepted"
)

// scheduler holds the stable pacing of the periodic runtime workers. The
// phase survives a restart, so a fleet keeps its spread across deployments.
type scheduler struct {
	identity          fleet.Identity
	sourcePhase       time.Duration
	sourceOffset      time.Duration
	acquisitionPhase  time.Duration
	acquisitionOffset time.Duration
	acceptedPhase     time.Duration
	acceptedOffset    time.Duration
}

// initializeSchedule derives the instance identity and the stable phases.
func (r *Runtime) initializeSchedule() error {
	instance, err := r.instanceIdentity()
	if err != nil {
		return err
	}
	safeSource := r.config.source.SafeIdentity()
	if r.source != nil {
		safeSource = r.source.Identity()
	}
	r.schedule.identity = fleet.Identity{
		Instance:   instance,
		Controller: controllerSource,
		Source:     safeSource,
	}

	sourceIdentity := r.schedule.identity
	if interval := r.config.source.PollInterval; interval > 0 {
		r.schedule.sourcePhase, err = fleet.StablePhase(sourceIdentity, interval)
		if err != nil {
			return err
		}
	}
	r.schedule.sourceOffset, err = fleet.StartupOffset(sourceIdentity, r.config.startupSpread)
	if err != nil {
		return err
	}

	acquisitionIdentity := sourceIdentity
	acquisitionIdentity.Controller = controllerAcquisition
	if interval := r.config.acquisition.Interval; interval > 0 {
		r.schedule.acquisitionPhase, err = fleet.StablePhase(acquisitionIdentity, interval)
		if err != nil {
			return err
		}
	}
	r.schedule.acquisitionOffset, err = fleet.StartupOffset(acquisitionIdentity, r.config.startupSpread)
	if err != nil {
		return err
	}
	if r.config.origin != nil && r.config.leaseStore != nil {
		acceptedIdentity := sourceIdentity
		acceptedIdentity.Controller = controllerAccepted
		r.schedule.acceptedPhase, err = fleet.StablePhase(acceptedIdentity, originAcceptedPollInterval)
		if err != nil {
			return err
		}
		r.schedule.acceptedOffset, err = fleet.StartupOffset(acceptedIdentity, r.config.startupSpread)
		if err != nil {
			return err
		}
	}
	return nil
}

// instanceIdentity binds the seed to the recorded owner. Listen addresses do not define identity.
func (r *Runtime) instanceIdentity() (string, error) {
	if identity := strings.TrimSpace(r.config.schedulerIdentity); identity != "" {
		return identity, nil
	}
	return persistentInstanceIdentity(r.instanceSeed, r.config.directoryOwner), nil
}

func persistentInstanceIdentity(seed string, owner DirectoryOwner) string {
	digest := sha256.New()
	for _, field := range []string{"starmap-runtime-v1", seed, owner.Product, owner.Deployment, owner.Instance} {
		_, _ = digest.Write([]byte(field))
		_, _ = digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// adoptSourceIdentity hands the derived instance identity to a source that
// takes one. The source then spreads its own work on the identity of this
// runtime, so one replica keeps one phase across every controller it owns.
func (r *Runtime) adoptSourceIdentity() {
	adopter, ok := r.source.(SourceIdentityAdopter)
	if !ok {
		return
	}
	adopter.AdoptInstanceIdentity(r.schedule.identity.Instance)
}

// sourceChanges returns the upstream wake channel of a reactive source. A
// source that reports no change returns nil, and its worker then wakes on the
// poll interval alone.
func (r *Runtime) sourceChanges() <-chan struct{} {
	watcher, ok := r.source.(SourceWatcher)
	if !ok {
		return nil
	}
	return watcher.Changes()
}

// startSchedules launches the periodic source and acquisition workers. A
// runtime without an acquirer runs source refresh only.
func (r *Runtime) startSchedules() {
	r.startPermissionSchedule()
	if r.config.generationPin != "" {
		return
	}
	if r.config.origin != nil && r.config.leaseStore != nil {
		r.work.Go(func() {
			r.runSchedule(controllerAccepted, originAcceptedPollInterval, r.schedule.acceptedOffset, r.schedule.acceptedPhase, false, nil, func(ctx context.Context) {
				if r.lease.status() != leaseLost {
					return
				}
				if err := r.refreshOriginAccepted(ctx); err != nil {
					r.logScheduledFailure(controllerAccepted, err)
				}
			})
		})
	}
	interval := r.config.source.PollInterval
	var wake <-chan struct{}
	if r.automaticSourceReads() {
		wake = r.sourceChanges()
	}
	if r.automaticSourceReads() && r.source != nil && (interval > 0 || wake != nil) {
		offset := r.schedule.sourceOffset
		phase := r.schedule.sourcePhase
		startup := r.sourceNeedsStartupPass()
		r.work.Go(func() {
			r.runSchedule(controllerSource, interval, offset, phase, startup, wake, func(ctx context.Context) {
				if _, err := r.RefreshSource(ctx); err != nil {
					r.logScheduledFailure(controllerSource, err)
				}
			})
		})
	}
	if r.config.updatePolicy.NetworkMode != NetworkOffline && r.config.acquisition.Enabled && r.hasAcquisition() {
		interval := r.config.acquisition.Interval
		offset := r.schedule.acquisitionOffset
		phase := r.schedule.acquisitionPhase
		startup := interval <= 0 || r.acquisitionNeedsStartupPass()
		r.work.Go(func() {
			r.runSchedule(controllerAcquisition, interval, offset, phase, startup, nil, func(ctx context.Context) {
				if _, err := r.Sync(ctx); err != nil {
					r.logScheduledFailure(controllerAcquisition, err)
				}
			})
		})
	}
}

// sourceNeedsStartupPass reports whether the source worker reads once right
// after its startup offset. A runtime with no retained source layer, or with a
// layer past the source-check warning age, is cold and reads early. A runtime
// with fresh durable evidence waits for its stable full-interval phase. The
// require_source policy needs no pass, because Open already read the source.
func (r *Runtime) sourceNeedsStartupPass() bool {
	if r.config.source.StartupPolicy == StartupRequireSource {
		return false
	}
	r.mu.RLock()
	layer := r.layers.source
	observed := time.Time{}
	if layer != nil {
		observed = layer.ObservedAt
	}
	r.mu.RUnlock()
	if layer == nil {
		return true
	}
	return r.pastWarnAge(observed, r.config.freshness.SourceCheckWarnAge)
}

// acquisitionNeedsStartupPass selects a pass after the startup offset.
// A source acquirer rechecks its configured inputs because retained provider
// timestamps cannot prove freshness for local or metadata sources.
// Provider-only acquisition checks retained layers against the warning age.
func (r *Runtime) acquisitionNeedsStartupPass() bool {
	if r.config.sourceAcquirer != nil && r.config.acquisitionSources.permitsMetadata() {
		return true
	}
	r.mu.RLock()
	var oldest time.Time
	retained := 0
	for _, id := range r.layers.activeProviderOrder() {
		layer := r.layers.providers[id]
		if oldest.IsZero() || layer.ObservedAt.Before(oldest) {
			oldest = layer.ObservedAt
		}
		retained++
	}
	r.mu.RUnlock()
	if retained == 0 {
		return true
	}
	return r.pastWarnAge(oldest, r.config.freshness.AcquisitionWarnAge)
}

// pastWarnAge reports whether one retained observation passed its warning age.
// An observation without a timestamp counts as stale.
func (r *Runtime) pastWarnAge(observed time.Time, warn time.Duration) bool {
	if observed.IsZero() {
		return true
	}
	return r.config.now().Sub(observed) >= warn
}

// logScheduledFailure reports one failed scheduled run. A replica that another
// instance owns logs at debug, because a refused non-owner is normal fleet
// behavior rather than a fault.
func (r *Runtime) logScheduledFailure(controller string, err error) {
	if isLeaseRefusal(err) {
		logging.Debug().
			Str("controller", controller).
			Msg("Another instance owns the refresh lease; the scheduled run skipped")
		return
	}
	logging.Warn().
		Err(err).
		Str("controller", controller).
		Msg("Scheduled runtime work failed")
}

// Wake reasons of one scheduled worker. An operator reads the reason to tell a
// reactive refresh from a periodic one.
const (
	wakeInterval = "interval"
	wakeUpstream = "upstream_change"
	wakeClosed   = "change_stream_closed"
)

// runSchedule paces one periodic worker. It waits the startup offset and runs
// the startup pass that a cold runtime needs. It then wakes on every interval
// boundary that its stable phase selects, and on every upstream change that
// the wake channel reports. A zero interval with no wake channel means one
// startup pass and no periodic work.
func (r *Runtime) runSchedule(
	controller string,
	interval, offset, phase time.Duration,
	startupPass bool,
	wake <-chan struct{},
	work func(context.Context),
) {
	if !r.sleep(offset) {
		return
	}
	if startupPass {
		logging.Debug().
			Str("controller", controller).
			Msg("Runtime schedule started its startup pass")
		work(r.ctx)
	}
	for {
		if interval <= 0 && wake == nil {
			return
		}
		reason, alive := r.waitForRun(untilNextRun(r.config.now(), interval, phase), wake)
		if !alive {
			return
		}
		if reason == wakeClosed {
			wake = nil
			continue
		}
		logging.Debug().
			Str("controller", controller).
			Str("reason", reason).
			Msg("Runtime schedule woke a periodic worker")
		work(r.ctx)
	}
}

// waitForRun blocks until the next interval boundary or an upstream change. It
// reports the reason it returned, and it reports false when the runtime
// closed. A zero wait means the worker has no interval, so only a change or a
// shutdown ends the wait.
func (r *Runtime) waitForRun(
	wait time.Duration,
	wake <-chan struct{},
) (string, bool) {
	if wake == nil {
		return wakeInterval, r.sleep(wait)
	}
	var boundary <-chan time.Time
	if wait > 0 {
		if after := r.config.scheduleTimer; after != nil {
			boundary = after(wait)
		} else {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			boundary = timer.C
		}
	}
	select {
	case <-r.ctx.Done():
		return "", false
	case <-boundary:
		return wakeInterval, true
	case _, open := <-wake:
		if !open {
			return wakeClosed, true
		}
		return wakeUpstream, true
	}
}

// sleep waits for the given delay. It reports false when the runtime closed.
func (r *Runtime) sleep(delay time.Duration) bool {
	if delay <= 0 {
		select {
		case <-r.ctx.Done():
			return false
		default:
			return true
		}
	}
	if after := r.config.scheduleTimer; after != nil {
		select {
		case <-r.ctx.Done():
			return false
		case <-after(delay):
			return true
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-r.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// untilNextRun returns the wait before the next interval boundary that carries
// the given phase. The result is stable across restarts, because it depends on
// the wall clock and the phase only.
func untilNextRun(now time.Time, interval, phase time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}
	elapsed := time.Duration(now.UnixNano()) % interval
	wait := phase - elapsed
	for wait <= 0 {
		wait += interval
	}
	return wait
}
