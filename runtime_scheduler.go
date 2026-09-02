package starmap

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Controller names of the two periodic runtime workers.
const (
	controllerSource      = "source"
	controllerAcquisition = "acquisition"
)

const (
	// instanceSeedFileName holds the durable part of the instance identity.
	instanceSeedFileName = "instance-seed"

	// instanceSeedBytes is the length of a generated seed.
	instanceSeedBytes = 16

	// maxInstanceSeedBytes bounds a loaded seed file, so an unsafe file fails
	// instead of reaching the identity hash.
	maxInstanceSeedBytes = 256

	// identityDigestLength keeps the derived identity short and stable.
	identityDigestLength = 16
)

// scheduler holds the stable pacing of the two periodic runtime workers. The
// phase survives a restart, so a fleet keeps its spread across deployments.
type scheduler struct {
	identity          fleet.Identity
	sourcePhase       time.Duration
	sourceOffset      time.Duration
	acquisitionPhase  time.Duration
	acquisitionOffset time.Duration
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
	return nil
}

// instanceIdentity returns the stable identity of this process. The configured
// override wins. Otherwise the identity combines a durable seed, the host
// name, and the listen address. A copied state directory then produces two
// identities instead of one.
func (r *Runtime) instanceIdentity() (string, error) {
	if identity := strings.TrimSpace(r.config.schedulerIdentity); identity != "" {
		return identity, nil
	}
	seed, err := r.store.instanceSeed()
	if err != nil {
		return "", err
	}
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	digest := sha256.New()
	for _, field := range []string{seed, host, r.config.listenAddress} {
		_, _ = digest.Write([]byte(field))
		_, _ = digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))[:identityDigestLength], nil
}

// instanceSeed returns the durable identity seed. It creates the seed on first
// use. A runtime without a state directory derives an empty seed, so its
// identity rests on the host name and the listen address alone.
func (s *layerStore) instanceSeed() (string, error) {
	if !s.durable() {
		return "", nil
	}
	path := filepath.Join(s.root, instanceSeedFileName)
	info, err := os.Stat(path)
	switch {
	case err == nil && info.Size() > maxInstanceSeedBytes:
		return "", &errors.ValidationError{
			Field: "instance_seed", Value: info.Size(), Message: "exceeds the seed bound",
		}
	case err == nil:
		raw, readErr := os.ReadFile(path) //nolint:gosec // The path is runtime-owned state.
		if readErr != nil {
			return "", errors.WrapIO("read", path, readErr)
		}
		if seed := strings.TrimSpace(string(raw)); seed != "" {
			return seed, nil
		}
	case !os.IsNotExist(err):
		return "", errors.WrapIO("stat", path, err)
	}

	var generated [instanceSeedBytes]byte
	if _, err := rand.Read(generated[:]); err != nil {
		return "", errors.WrapIO("generate", path, err)
	}
	seed := hex.EncodeToString(generated[:])
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(seed), constants.SecureFilePermissions); err != nil {
		return "", errors.WrapIO("write", temporary, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return "", errors.WrapIO("rename", path, err)
	}
	return seed, nil
}

// startSchedules launches the periodic source and acquisition workers. A
// runtime without an acquirer runs source refresh only.
func (r *Runtime) startSchedules() {
	if interval := r.config.source.PollInterval; r.source != nil && interval > 0 {
		offset := r.schedule.sourceOffset
		phase := r.schedule.sourcePhase
		r.work.Go(func() {
			r.runSchedule(controllerSource, interval, offset, phase, func(ctx context.Context) {
				if _, err := r.RefreshSource(ctx); err != nil {
					logging.Warn().Err(err).Msg("Scheduled source refresh failed")
				}
			})
		})
	}
	if r.config.acquisition.Enabled && r.config.acquirer != nil {
		interval := r.config.acquisition.Interval
		offset := r.schedule.acquisitionOffset
		phase := r.schedule.acquisitionPhase
		r.work.Go(func() {
			r.runSchedule(controllerAcquisition, interval, offset, phase, func(ctx context.Context) {
				if _, err := r.Sync(ctx); err != nil {
					logging.Warn().Err(err).Msg("Scheduled provider acquisition failed")
				}
			})
		})
	}
}

// runSchedule paces one periodic worker. It waits the startup offset, then
// wakes on every interval boundary that its stable phase selects.
func (r *Runtime) runSchedule(
	controller string,
	interval, offset, phase time.Duration,
	work func(context.Context),
) {
	if !r.sleep(offset) {
		return
	}
	for {
		wait := untilNextRun(r.config.now(), interval, phase)
		if !r.sleep(wait) {
			return
		}
		logging.Debug().
			Str("controller", controller).
			Msg("Runtime schedule woke a periodic worker")
		work(r.ctx)
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
