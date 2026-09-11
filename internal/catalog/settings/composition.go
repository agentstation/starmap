package settings

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/remote"
	"github.com/agentstation/starmap/runtime"
)

// cascadeFallbackAfterFailures is how many consecutive stream failures the
// cascade accepts before it polls the upstream manifest. A short streaming
// outage recovers on its own, so polling starts only after the stream proved
// it cannot hold.
const cascadeFallbackAfterFailures = 3

// Composition builds the connected runtime of one process. It joins the parsed
// canonical settings with the roles that the process injects: the catalog
// store, the provider acquirer, and an optional upstream source
// implementation.
//
// The root package cannot construct the Starmap cascade source, because the
// cascade subscriber imports the root package. This composition step is
// therefore the one place that builds it from the canonical settings.
type Composition struct {
	// Config holds the canonical settings the process read.
	Config Config

	// Source supplies an upstream source implementation. It overrides the
	// source that the settings select, so a test injects a hermetic source.
	Source runtime.Source

	// Acquirer collects provider observations. A runtime without one observes
	// no provider.
	Acquirer runtime.Acquirer

	// LeaseStore fences the durable generation commit of a replicated
	// deployment. A single instance needs none.
	LeaseStore runtime.LeaseStore

	// OriginStore supplies the selected catalog store for an explicit origin declaration.
	// Composition constructs no store and starts no storage I/O.
	OriginStore storage.Store

	// Base holds options that the process supplies before every setting. A
	// canonical setting overrides a base option of the same name.
	Base []runtime.Option

	// Extra holds options that one command supplies last. The serve command
	// supplies its listen address here, so two replicas on one host take
	// separate schedule phases.
	Extra []runtime.Option
}

// Options returns the runtime options in precedence order: the process base
// options first, then canonical runtime options and the host clock, then the
// injected roles. A later option replaces an earlier one, so a canonical
// setting always wins over a base default.
func (c Composition) Options() ([]runtime.Option, error) {
	monitor, err := hostclock.NewMonitor(c.Config.PermissionClock)
	if err != nil {
		return nil, err
	}
	source := c.Source
	if source == nil && c.Config.SourceKind == runtime.SourceStarmap {
		built, err := c.cascadeSource()
		if err != nil {
			return nil, err
		}
		source = built
	}
	options := make([]runtime.Option, 0, len(c.Base)+len(c.Config.Options())+len(c.Extra)+3)
	options = append(options, c.Base...)
	options = append(options, c.Config.Options()...)
	if c.Config.PermissionClock.Source != "" {
		options = append(options, runtime.WithoutPermissionClock())
	}
	if monitor != nil {
		options = append(options, runtime.WithPermissionClockMonitor(monitor))
	}
	if _, present := c.Config.Value(AuthorityOrigin); present {
		options = append(options, runtime.WithoutAuthorityOrigin())
		origin := c.Config.AuthorityOrigin
		if origin.Enabled {
			if c.OriginStore == nil || monitor == nil {
				return nil, &errors.ConfigError{Component: "catalog authority origin", Message: "an enabled origin requires a catalog store and a configured native permission clock"}
			}
			options = append(options, runtime.WithAuthorityOrigin(c.OriginStore, runtime.OriginConfig{
				AuthorityID: origin.AuthorityID, PolicyID: origin.PolicyID, Bootstrap: origin.Bootstrap,
				PermissionLifetime: origin.PermissionLifetime, Clock: monitor.Read,
			}))
		}
	}
	if source != nil {
		options = append(options, runtime.WithSource(source))
	}
	if c.Acquirer != nil {
		options = append(options, runtime.WithAcquirer(c.Acquirer))
	}
	if c.LeaseStore != nil {
		options = append(options, runtime.WithLeaseStore(c.LeaseStore))
	}
	options = append(options, c.Extra...)
	return options, nil
}

// cascadeSource builds the Starmap cascade source from the canonical settings.
// It opens no connection, because the runtime reads the source on its own
// schedule and the first read starts the subscription.
//
// The subscriber keeps its verified generations in memory. The runtime retains
// the accepted generation in its own state directory, so a restart serves the
// last upstream catalog without a second durable copy here.
func (c Composition) cascadeSource() (runtime.Source, error) {
	subscriber, err := cascadeSubscriber(c.Config)
	if err != nil {
		return nil, err
	}
	return remote.NewSource(context.Background(), remote.SourceConfig{
		Subscriber: subscriber,
		MaxHops:    c.Config.SourceMaxHops,
		MaxAge:     c.Config.SourceMaxAge,
	})
}

// cascadeSubscriber maps the canonical settings onto the subscriber transport
// and pacing. The transfer bounds and the startup spread reach the subscriber,
// so one deployment paces its cascade exactly as it paces every other worker.
//
// The identity here is the configured scheduler identity only. Open hands the
// derived instance identity to the source. A deployment that configures no
// identity still spreads its cascade on the identity of its own runtime.
func cascadeSubscriber(config Config) (remote.Config, error) {
	if strings.TrimSpace(config.SourceURL) == "" {
		return remote.Config{}, &errors.ConfigError{
			Component: "catalog source",
			Message:   "the starmap catalog source needs " + SourceURL,
		}
	}
	transfer := protocol.DefaultTransferPolicy()
	if config.TransferIdleTimeout > 0 {
		transfer.IdleTimeout = config.TransferIdleTimeout
	}
	if config.TransferMaxDuration > 0 {
		transfer.MaxDuration = config.TransferMaxDuration
	}
	spread := config.StartupSpread
	if _, present := config.Value(StartupSpread); present && spread == 0 {
		// The subscriber uses a negative duration to disable its default spread.
		spread = -1
	}
	return remote.Config{
		BaseURL:        config.SourceURL,
		CatalogStore:   storage.NewMemory(),
		APIKey:         strings.TrimSpace(config.SourceAPIKey),
		Identity:       fleet.Identity{Instance: config.SchedulerIdentity},
		StartupSpread:  spread,
		TransferPolicy: &transfer,
		PollingFallback: &remote.PollingFallbackPolicy{
			AfterFailures: cascadeFallbackAfterFailures,
			Interval:      remote.DefaultFallbackPollInterval,
		},
	}, nil
}

// Open composes the options and opens the connected runtime. The runtime serves
// the verified embedded catalog immediately and pulls its source in the
// background.
func (c Composition) Open(ctx context.Context) (*runtime.Runtime, error) {
	options, err := c.Options()
	if err != nil {
		return nil, err
	}
	return runtime.Open(ctx, options...)
}
