package app

import (
	"context"
	"os"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/catalog/settings"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

// CatalogSettings returns the canonical catalog settings that this process
// read. The report and the readiness answer name the same settings.
func (a *App) CatalogSettings() settings.Config {
	return a.catalogSettings
}

// Runtime returns the process-owned connected runtime. It opens the runtime
// once and reuses it. The runtime serves the verified embedded catalog at once
// and pulls the selected source in the background, so no command waits for the
// network. Extra options apply last, so a command overrides one setting.
func (a *App) Runtime(ctx context.Context, extra ...starmap.Option) (*starmap.Runtime, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtime != nil {
		return a.runtime, nil
	}
	composition, err := a.composition(extra)
	if err != nil {
		return nil, err
	}
	runtime, err := composition.Open(ctx)
	if err != nil {
		return nil, err
	}
	a.runtime = runtime

	// The runtime commits every effective generation through this client, so
	// the application publishes one client. A hook that a command registers
	// then observes runtime publications.
	a.mu.Lock()
	a.starmap = runtime.Client()
	a.mu.Unlock()
	return runtime, nil
}

// RuntimeStatus reports the connected-runtime status of this process. A process
// that opened no runtime reports the closed status.
func (a *App) RuntimeStatus() starmap.RuntimeStatus {
	a.runtimeMu.Lock()
	runtime := a.runtime
	a.runtimeMu.Unlock()
	if runtime == nil {
		return starmap.RuntimeStatus{}
	}
	return runtime.Status()
}

// closeRuntime stops the connected runtime. Close joins runtime-owned work
// inside the runtime's own five-second bound and releases the lease.
func (a *App) closeRuntime() error {
	a.runtimeMu.Lock()
	runtime := a.runtime
	a.runtime = nil
	a.runtimeMu.Unlock()
	if runtime == nil {
		return nil
	}
	return runtime.Close()
}

// composition builds the process composition from the canonical settings and
// the roles that this process owns.
func (a *App) composition(extra []starmap.Option) (settings.Composition, error) {
	base, err := a.baseCatalogOptions()
	if err != nil {
		return settings.Composition{}, err
	}
	acquirer, err := a.acquirer()
	if err != nil {
		return settings.Composition{}, err
	}
	return settings.Composition{
		Config:   a.catalogSettings,
		Acquirer: acquirer,
		Base:     base,
		Extra:    extra,
	}, nil
}

// baseCatalogOptions returns the options that this process supplies before any
// canonical setting. A supplied setting replaces the matching base option.
func (a *App) baseCatalogOptions() ([]starmap.Option, error) {
	storeOption, err := a.catalogStoreOption()
	if err != nil {
		return nil, err
	}
	statePath, err := a.runtimeStatePath()
	if err != nil {
		return nil, err
	}
	catalogOptions, err := a.catalogClientOptions()
	if err != nil {
		return nil, err
	}
	options := make([]starmap.Option, 0, 2+len(catalogOptions))
	options = append(options, storeOption, starmap.WithStateDirectory(statePath))
	return append(options, catalogOptions...), nil
}

// acquirer builds the provider acquisition role. The root package selects no
// provider client, so the application injects this role.
func (a *App) acquirer() (starmap.Acquirer, error) {
	resolver, err := a.CredentialResolver()
	if err != nil {
		return nil, err
	}
	return acquisition.NewAcquirer(
		acquisition.WithAcquirerCredentialResolver(resolver),
	)
}

// runtimeStatePath returns the process-local runtime state directory. The
// scheduler seed, the retained provider layers, and the source discovery state
// live there. It never joins the catalog store.
func (a *App) runtimeStatePath() (string, error) {
	if a.catalogSettings.StateDirectory != "" {
		return a.catalogSettings.StateDirectory, nil
	}
	return expandHomePath(constants.DefaultRuntimeStatePath)
}

// loadCatalogSettings reads the canonical catalog settings of this process. An
// override reads before the environment, so a flag replaces an exported value.
// The legacy remote-server keys map onto the canonical starmap source names, so
// one deployment carries one set of names.
func loadCatalogSettings(config *Config, overrides ...settings.Lookup) (settings.Config, error) {
	environment := func(name string) (string, bool) {
		if value, found := os.LookupEnv(name); found {
			return value, true
		}
		if config == nil || config.RemoteServerURL == "" {
			return "", false
		}
		switch name {
		case settings.Source:
			return string(starmap.SourceStarmap), true
		case settings.SourceURL:
			return config.RemoteServerURL, true
		case settings.SourceAPIKey:
			return config.RemoteServerAPIKey, true
		}
		return "", false
	}
	return settings.Load(settings.Chain(append(overrides, environment)...))
}
