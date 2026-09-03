package starmap

import (
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/errors"
)

// Transfer and refresh defaults. They match the canonical CATALOG_TRANSFER_*
// and CATALOG_REFRESH_TIMEOUT settings.
const (
	// DefaultTransferIdleTimeout bounds a stalled transfer.
	DefaultTransferIdleTimeout = 2 * time.Minute

	// DefaultTransferMaxDuration bounds one whole transfer.
	DefaultTransferMaxDuration = 60 * time.Minute

	// DefaultRefreshTimeout is zero, so a refresh carries no deadline of its
	// own. Transfer bounds and caller cancellation stop a stalled run.
	DefaultRefreshTimeout time.Duration = 0

	// DefaultCoalesceWindow bounds how long completed provider observations
	// wait for a slower sibling before they publish.
	DefaultCoalesceWindow = 30 * time.Second
)

// runtimeOptions holds every setting that belongs to the connected runtime.
// The offline Client rejects all of them.
type runtimeOptions struct {
	source       SourcePolicy
	sourceToken  string
	sourceAPIKey string

	acquisition AcquisitionPolicy
	freshness   FreshnessPolicy

	stateDirectory string
	startupSpread  time.Duration

	transferIdleTimeout time.Duration
	transferMaxDuration time.Duration
	refreshTimeout      time.Duration
	coalesceWindow      time.Duration

	schedulerIdentity string
	listenAddress     string

	customSource Source
	acquirer     Acquirer
	leaseStore   LeaseStore

	now    func() time.Time
	random Random

	// scheduleTimer paces the periodic workers. It stays unexported and nil in
	// every deployment, so production keeps one stopped timer per wait. A test
	// injects it to drive a schedule without a real delay.
	scheduleTimer func(time.Duration) <-chan time.Time
}

// Random returns a uniform value in the half-open interval [0, 1). The
// runtime uses it to spread scheduled work across a fleet.
type Random func() float64

// runtimeDefaults returns the canonical connected-runtime configuration.
func runtimeDefaults() runtimeOptions {
	return runtimeOptions{
		source:              DefaultSourcePolicy(),
		acquisition:         DefaultAcquisitionPolicy(),
		freshness:           DefaultFreshnessPolicy(),
		startupSpread:       fleet.DefaultStartupSpread,
		transferIdleTimeout: DefaultTransferIdleTimeout,
		transferMaxDuration: DefaultTransferMaxDuration,
		refreshTimeout:      DefaultRefreshTimeout,
		coalesceWindow:      DefaultCoalesceWindow,
		now:                 time.Now,
		random:              Random(fleet.SystemRandom()),
	}
}

// transferPolicy maps the configured transfer bounds onto the remote transport
// policy that the source uses.
func (r runtimeOptions) transferPolicy() remote.TransferPolicy {
	policy := remote.DefaultTransferPolicy()
	policy.IdleTimeout = r.transferIdleTimeout
	policy.MaxDuration = r.transferMaxDuration
	return policy
}

// validate checks every runtime setting before Open starts any work.
func (r runtimeOptions) validate() error {
	if err := r.source.Validate(); err != nil {
		return err
	}
	if err := r.acquisition.Validate(); err != nil {
		return err
	}
	if err := r.freshness.Validate(); err != nil {
		return err
	}
	if r.startupSpread < 0 {
		return &errors.ValidationError{
			Field: "startup_spread", Value: r.startupSpread, Message: "must not be negative",
		}
	}
	if r.refreshTimeout < 0 {
		return &errors.ValidationError{
			Field: "refresh_timeout", Value: r.refreshTimeout, Message: "must not be negative",
		}
	}
	if r.coalesceWindow <= 0 {
		return &errors.ValidationError{
			Field: "coalesce_window", Value: r.coalesceWindow, Message: "must be positive",
		}
	}
	return r.transferPolicy().Validate()
}

// runtimeOption marks one option as runtime-only and applies it.
func runtimeOption(name string, apply func(*runtimeOptions) error) Option {
	return func(o *options) error {
		o.runtimeOptions = append(o.runtimeOptions, name)
		return apply(&o.runtime)
	}
}

// WithCatalogSource selects the upstream catalog source by name. It accepts
// public, github, starmap, file, and embedded. A named custom source is
// terminal: the runtime never falls back to the public channel.
func WithCatalogSource(name string) Option {
	return runtimeOption("WithCatalogSource", func(r *runtimeOptions) error {
		kind, err := ParseSourceKind(name)
		if err != nil {
			return err
		}
		r.source.Kind = kind
		return nil
	})
}

// WithSourcePolicy replaces the whole upstream source policy.
func WithSourcePolicy(policy SourcePolicy) Option {
	return runtimeOption("WithSourcePolicy", func(r *runtimeOptions) error {
		if err := policy.Validate(); err != nil {
			return err
		}
		r.source = policy
		return nil
	})
}

// WithSourceURL sets the deployment-owned source address used by the starmap
// and file sources.
func WithSourceURL(url string) Option {
	return runtimeOption("WithSourceURL", func(r *runtimeOptions) error {
		r.source.URL = url
		return nil
	})
}

// WithSourceRepository names the catalog repository of a GitHub channel.
func WithSourceRepository(repository string) Option {
	return runtimeOption("WithSourceRepository", func(r *runtimeOptions) error {
		r.source.Repository = repository
		return nil
	})
}

// WithSourceChannel names the mutable release that selects the current catalog.
func WithSourceChannel(channel string) Option {
	return runtimeOption("WithSourceChannel", func(r *runtimeOptions) error {
		r.source.Channel = channel
		return nil
	})
}

// WithSourceSignerWorkflow pins the build provenance the source accepts.
func WithSourceSignerWorkflow(workflow string) Option {
	return runtimeOption("WithSourceSignerWorkflow", func(r *runtimeOptions) error {
		r.source.SignerWorkflow = workflow
		return nil
	})
}

// WithSourceToken supplies the source access token. The runtime keeps the
// token out of status, logs, and errors.
func WithSourceToken(token string) Option {
	return runtimeOption("WithSourceToken", func(r *runtimeOptions) error {
		r.sourceToken = token
		return nil
	})
}

// WithSourceAPIKey supplies the source API key. The runtime keeps the key out
// of status, logs, and errors.
func WithSourceAPIKey(key string) Option {
	return runtimeOption("WithSourceAPIKey", func(r *runtimeOptions) error {
		r.sourceAPIKey = key
		return nil
	})
}

// WithSourceAliases declares the other stable identities that name this same
// runtime. A served source chain that names one of them is a self reference.
// The runtime then refuses the read instead of serving its own catalog back to
// itself.
func WithSourceAliases(aliases ...string) Option {
	return runtimeOption("WithSourceAliases", func(r *runtimeOptions) error {
		trimmed := make([]string, 0, len(aliases))
		for _, alias := range aliases {
			if value := strings.TrimSpace(alias); value != "" {
				trimmed = append(trimmed, value)
			}
		}
		r.source.Aliases = trimmed
		return nil
	})
}

// WithSourcePollInterval sets the channel check period.
func WithSourcePollInterval(interval time.Duration) Option {
	return runtimeOption("WithSourcePollInterval", func(r *runtimeOptions) error {
		if interval < 0 {
			return &errors.ValidationError{
				Field: "source_poll_interval", Value: interval, Message: "must not be negative",
			}
		}
		r.source.PollInterval = interval
		return nil
	})
}

// WithSourceStartupPolicy decides what the runtime serves before the first
// upstream reply.
func WithSourceStartupPolicy(name string) Option {
	return runtimeOption("WithSourceStartupPolicy", func(r *runtimeOptions) error {
		policy, err := ParseStartupPolicy(name)
		if err != nil {
			return err
		}
		r.source.StartupPolicy = policy
		return nil
	})
}

// WithSourceMaxAge sets the age at which the served catalog counts as stale.
func WithSourceMaxAge(maxAge time.Duration) Option {
	return runtimeOption("WithSourceMaxAge", func(r *runtimeOptions) error {
		if maxAge < 0 {
			return &errors.ValidationError{
				Field: "source_max_age", Value: maxAge, Message: "must not be negative",
			}
		}
		r.source.MaxAge = maxAge
		return nil
	})
}

// WithSourceMaxHops bounds a cascade of Starmap runtimes.
func WithSourceMaxHops(hops int) Option {
	return runtimeOption("WithSourceMaxHops", func(r *runtimeOptions) error {
		if hops < 1 {
			return &errors.ValidationError{
				Field: "source_max_hops", Value: hops, Message: "must be at least one",
			}
		}
		r.source.MaxHops = hops
		return nil
	})
}

// WithSource injects a deployment-owned upstream source. It replaces every
// built-in source implementation.
func WithSource(source Source) Option {
	return runtimeOption("WithSource", func(r *runtimeOptions) error {
		if source == nil {
			return &errors.ValidationError{Field: "source", Message: "is required"}
		}
		r.customSource = source
		return nil
	})
}

// WithAcquisitionEnabled turns scheduled provider acquisition on or off.
func WithAcquisitionEnabled(enabled bool) Option {
	return runtimeOption("WithAcquisitionEnabled", func(r *runtimeOptions) error {
		r.acquisition.Enabled = enabled
		return nil
	})
}

// WithAcquisitionInterval sets the provider acquisition period. Zero selects
// one startup pass and no periodic work.
func WithAcquisitionInterval(interval time.Duration) Option {
	return runtimeOption("WithAcquisitionInterval", func(r *runtimeOptions) error {
		if interval < 0 {
			return &errors.ValidationError{
				Field: "acquisition_interval", Value: interval, Message: "must not be negative",
			}
		}
		r.acquisition.Interval = interval
		return nil
	})
}

// WithAcquirer injects the provider acquisition composition. The root package
// selects no concrete provider client, so a runtime without an acquirer runs
// source refresh only.
func WithAcquirer(acquirer Acquirer) Option {
	return runtimeOption("WithAcquirer", func(r *runtimeOptions) error {
		if acquirer == nil {
			return &errors.ValidationError{Field: "acquirer", Message: "is required"}
		}
		r.acquirer = acquirer
		return nil
	})
}

// WithStateDirectory selects the durable directory that retains layers, the
// scheduler identity seed, and source discovery state.
func WithStateDirectory(directory string) Option {
	return runtimeOption("WithStateDirectory", func(r *runtimeOptions) error {
		if directory == "" {
			return &errors.ValidationError{Field: "state_directory", Message: "is required"}
		}
		r.stateDirectory = directory
		return nil
	})
}

// WithStartupSpread bounds the random delay before the first scheduled run.
func WithStartupSpread(spread time.Duration) Option {
	return runtimeOption("WithStartupSpread", func(r *runtimeOptions) error {
		if spread < 0 {
			return &errors.ValidationError{
				Field: "startup_spread", Value: spread, Message: "must not be negative",
			}
		}
		r.startupSpread = spread
		return nil
	})
}

// WithTransferIdleTimeout bounds a stalled transfer.
func WithTransferIdleTimeout(timeout time.Duration) Option {
	return runtimeOption("WithTransferIdleTimeout", func(r *runtimeOptions) error {
		if timeout <= 0 {
			return &errors.ValidationError{
				Field: "transfer_idle_timeout", Value: timeout, Message: "must be positive",
			}
		}
		r.transferIdleTimeout = timeout
		return nil
	})
}

// WithTransferMaxDuration bounds one whole transfer.
func WithTransferMaxDuration(duration time.Duration) Option {
	return runtimeOption("WithTransferMaxDuration", func(r *runtimeOptions) error {
		if duration <= 0 {
			return &errors.ValidationError{
				Field: "transfer_max_duration", Value: duration, Message: "must be positive",
			}
		}
		r.transferMaxDuration = duration
		return nil
	})
}

// WithRefreshTimeout bounds one whole refresh run. Zero, the default, adds no
// deadline, so a long transfer inside its own bounds is not cut short.
func WithRefreshTimeout(timeout time.Duration) Option {
	return runtimeOption("WithRefreshTimeout", func(r *runtimeOptions) error {
		if timeout < 0 {
			return &errors.ValidationError{
				Field: "refresh_timeout", Value: timeout, Message: "must not be negative",
			}
		}
		r.refreshTimeout = timeout
		return nil
	})
}

// WithCoalesceWindow bounds how long completed provider observations wait for
// a slower sibling before they publish.
func WithCoalesceWindow(window time.Duration) Option {
	return runtimeOption("WithCoalesceWindow", func(r *runtimeOptions) error {
		if window <= 0 {
			return &errors.ValidationError{
				Field: "coalesce_window", Value: window, Message: "must be positive",
			}
		}
		r.coalesceWindow = window
		return nil
	})
}

// WithSchedulerIdentity overrides the derived instance identity. Use it when
// the deployment already owns a stable instance name.
func WithSchedulerIdentity(identity string) Option {
	return runtimeOption("WithSchedulerIdentity", func(r *runtimeOptions) error {
		r.schedulerIdentity = identity
		return nil
	})
}

// WithListenAddress records the server listen address. It separates two
// instances that share a copied state directory.
func WithListenAddress(address string) Option {
	return runtimeOption("WithListenAddress", func(r *runtimeOptions) error {
		r.listenAddress = address
		return nil
	})
}

// WithLeaseStore injects the shared-storage lease that fences durable commits.
// A deployment without shared storage needs no lease.
func WithLeaseStore(store LeaseStore) Option {
	return runtimeOption("WithLeaseStore", func(r *runtimeOptions) error {
		if store == nil {
			return &errors.ValidationError{Field: "lease_store", Message: "is required"}
		}
		r.leaseStore = store
		return nil
	})
}

// WithFreshnessPolicy replaces the freshness thresholds.
func WithFreshnessPolicy(policy FreshnessPolicy) Option {
	return runtimeOption("WithFreshnessPolicy", func(r *runtimeOptions) error {
		if err := policy.Validate(); err != nil {
			return err
		}
		r.freshness = policy
		return nil
	})
}

// WithClock injects the runtime clock. Tests use it to keep timing exact.
func WithClock(now func() time.Time) Option {
	return runtimeOption("WithClock", func(r *runtimeOptions) error {
		if now == nil {
			return &errors.ValidationError{Field: "clock", Message: "is required"}
		}
		r.now = now
		return nil
	})
}

// WithRandom injects the jitter source that spreads scheduled work.
func WithRandom(random Random) Option {
	return runtimeOption("WithRandom", func(r *runtimeOptions) error {
		if random == nil {
			return &errors.ValidationError{Field: "random", Message: "is required"}
		}
		r.random = random
		return nil
	})
}
