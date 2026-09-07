package app

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/catalog/settings"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
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
func (a *App) Runtime(ctx context.Context, extra ...runtime.Option) (*runtime.Runtime, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	return a.openRuntimeLocked(ctx, extra)
}

// openRuntimeLocked requires runtimeMu throughout runtime creation and publication.
func (a *App) openRuntimeLocked(ctx context.Context, extra []runtime.Option) (*runtime.Runtime, error) {
	if a.runtime != nil {
		return a.runtime, nil
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		return nil, err
	}
	completion, err := a.checkLegacyRoots(ctx, paths)
	if err != nil {
		return nil, err
	}
	if completion != nil {
		extra = append(slices.Clone(extra), runtime.WithCompletedDirectoryMigration(*completion))
	}
	if err := runtime.ValidateDirectoryPermissions(ctx, paths.Runtime.Path); err != nil {
		return nil, err
	}
	if _, err := bootstrap.Export(ctx, paths.Baselines.Path); err != nil {
		return nil, err
	}
	composition, err := a.composition(extra)
	if err != nil {
		return nil, err
	}
	connected, err := composition.Open(ctx)
	if err != nil {
		return nil, err
	}
	a.runtime = connected

	// The runtime commits every effective generation through this client, so
	// the application publishes one client. A hook that a command registers
	// then observes runtime publications.
	a.mu.Lock()
	a.starmap = connected.Client()
	a.mu.Unlock()
	return connected, nil
}

// RuntimeStatus reports the connected-runtime status of this process. A process
// that opened no runtime reports the closed status.
func (a *App) RuntimeStatus() runtime.Status {
	a.runtimeMu.Lock()
	connected := a.runtime
	a.runtimeMu.Unlock()
	if connected == nil {
		return runtime.Status{}
	}
	return connected.Status()
}

// closeRuntime stops the connected runtime. Close joins runtime-owned work
// inside the runtime's own five-second bound and releases the lease.
func (a *App) closeRuntime() error {
	a.runtimeMu.Lock()
	connected := a.runtime
	a.runtime = nil
	a.runtimeMu.Unlock()
	if connected == nil {
		return nil
	}
	return connected.Close()
}

// composition builds the process composition from the canonical settings and
// the roles that this process owns.
func (a *App) composition(extra []runtime.Option) (settings.Composition, error) {
	base, err := a.baseCatalogOptions()
	if err != nil {
		return settings.Composition{}, err
	}
	acquirer, err := a.acquirer()
	if err != nil {
		return settings.Composition{}, err
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		return settings.Composition{}, err
	}
	resolvedExtras := make([]runtime.Option, 0, 4+len(extra))
	resolvedExtras = append(resolvedExtras, runtime.WithDirectoryOwner(runtime.DirectoryOwner{Product: "starmap", Deployment: paths.DeploymentID, Instance: paths.InstanceID}), runtime.WithStateDirectory(paths.Runtime.Path), runtime.WithClientOptions(starmap.WithCatalogPath(paths.Workspace.Path)))
	if paths.SourceFile.Path != "" {
		resolvedExtras = append(resolvedExtras, runtime.WithSourceURL(paths.SourceFile.Path))
	}
	resolvedExtras = append(resolvedExtras, extra...)
	return settings.Composition{Config: a.catalogSettings, Acquirer: acquirer, Base: base, Extra: resolvedExtras}, nil
}

// baseCatalogOptions returns the options that this process supplies before any
// canonical setting. A supplied setting replaces the matching base option.
func (a *App) baseCatalogOptions() ([]runtime.Option, error) {
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
	return []runtime.Option{
		runtime.WithClientOptions(storeOption),
		runtime.WithStateDirectory(statePath),
		runtime.WithClientOptions(catalogOptions...),
	}, nil
}

// acquirer builds the provider acquisition role. The root package selects no
// provider client, so the application injects this role.
func (a *App) acquirer() (runtime.Acquirer, error) {
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
	paths, err := a.ResolvedPaths()
	if err != nil {
		return "", err
	}
	return paths.Runtime.Path, nil
}

// loadCatalogSettings resolves flags, environment, canonical file values and legacy aliases.
// Source replacement clears lower transport credentials before runtime composition.
func loadCatalogSettings(config *Config, overrides ...settings.Lookup) (settings.Config, error) {
	layers := make([]catalogconfig.Layer, 0, len(overrides)+3)
	for index, lookup := range overrides {
		values := make(map[string]string)
		if lookup != nil {
			for _, name := range settings.Names() {
				if value, present := lookup(name); present {
					values[name] = value
				}
			}
		}
		layers = append(layers, catalogconfig.Layer{Name: "override-" + strconv.Itoa(index+1), Values: values})
	}
	environment := make(map[string]string)
	for _, name := range append(settings.Names(), "REMOTE_SERVER_URL", "REMOTE_SERVER_API_KEY") {
		if value, present := os.LookupEnv(name); present {
			environment[name] = value
		}
	}
	// Unknown catalog names must reach the schema even when no descriptor lists them.
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "STARMAP_CATALOG_") && !isPathEnvironmentName(name) {
			environment[name] = value
		}
	}
	input := catalogEnvironmentInput(environment)
	layers = append(layers, catalogconfig.Layer{Name: "environment", Values: input.values})
	if config != nil {
		for index := len(config.dotenvLayers) - 1; index >= 0; index-- {
			layers = append(layers, config.dotenvLayers[index])
		}
		values := config.CatalogValues
		if !config.catalogFileRead {
			legacy := make(map[string]string)
			if config.RemoteServerURL != "" {
				legacy[settings.SourceURL] = config.RemoteServerURL
			}
			if config.RemoteServerAPIKey != "" {
				legacy[settings.SourceAPIKey] = config.RemoteServerAPIKey
			}
			values = normalizeLegacyCatalogSource(values, legacy)
		}
		layers = append(layers, catalogconfig.Layer{Name: "configuration-file", Values: values})
		config.LegacyCatalogNames = append(config.LegacyCatalogNames, input.legacyNames...)
		slices.Sort(config.LegacyCatalogNames)
		config.LegacyCatalogNames = slices.Compact(config.LegacyCatalogNames)
	}
	resolved, err := catalogconfig.Resolve(layers...)
	if err != nil {
		return settings.Config{}, err
	}
	if config != nil {
		config.CatalogOrigins, config.CatalogIgnored = resolved.Origins, resolved.Ignored
	}
	return resolved.Config, nil
}
