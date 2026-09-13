// Package config defines the shared catalog settings contract.
// Parsing reads only caller-supplied values and starts no runtime work.
package config

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock/profile"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

// Prefix is the canonical Starmap environment prefix.
const Prefix = "STARMAP_"

// The canonical catalog setting names select runtime options or host configuration.
const (
	// Source selects the upstream catalog source. The default is public.
	Source = Prefix + "CATALOG_SOURCE"

	// SourceURL is the safe endpoint or file identity of a custom source.
	SourceURL = Prefix + "CATALOG_SOURCE_URL"

	// SourceAPIKey is the Starmap protocol credential. It stays separate from
	// every provider credential.
	SourceAPIKey = Prefix + "CATALOG_SOURCE_API_KEY"

	// SourceRepository names the GitHub repository that holds the channel.
	SourceRepository = Prefix + "CATALOG_SOURCE_REPOSITORY"

	// SourceChannel names the branch that carries the attested channel
	// document.
	SourceChannel = Prefix + "CATALOG_SOURCE_CHANNEL"

	// SourceSignerWorkflow pins the accepted build provenance.
	SourceSignerWorkflow = Prefix + "CATALOG_SOURCE_SIGNER_WORKFLOW"

	// SourceToken is the optional GitHub API token. Public local use needs no
	// token.
	SourceToken = Prefix + "CATALOG_SOURCE_TOKEN"

	// SourceRefreshMode selects automatic or explicit-only source refresh.
	SourceRefreshMode = Prefix + "CATALOG_SOURCE_REFRESH_MODE"

	// GenerationPin selects one retained catalog until the configuration changes.
	GenerationPin = Prefix + "CATALOG_GENERATION_PIN"

	// NetworkMode controls outbound catalog acquisition independently of inference.
	NetworkMode = Prefix + "CATALOG_NETWORK_MODE"

	// SourcePollInterval is the conditional channel check period.
	SourcePollInterval = Prefix + "CATALOG_SOURCE_POLL_INTERVAL"

	// SourceStartupPolicy decides what the runtime serves before the first
	// upstream reply.
	SourceStartupPolicy = Prefix + "CATALOG_SOURCE_STARTUP_POLICY"

	// SourceAuthorityID pins the internal authority used by require_authority.
	SourceAuthorityID = Prefix + "CATALOG_SOURCE_AUTHORITY_ID"

	// SourcePolicyID pins the permission policy within the internal authority.
	SourcePolicyID = Prefix + "CATALOG_SOURCE_POLICY_ID"

	// SourceMaxAge is the source freshness warning objective.
	SourceMaxAge = Prefix + "CATALOG_SOURCE_MAX_AGE"

	// SourceMaxHops bounds an accepted Starmap source chain.
	SourceMaxHops = Prefix + "CATALOG_SOURCE_MAX_HOPS"

	// SourceAliases lists the other stable identities of this same runtime, as
	// a comma-separated list. A served source chain that names one of them is
	// a self reference.
	SourceAliases = Prefix + "CATALOG_SOURCE_ALIASES"

	// AcquisitionEnabled turns all automatic acquisition on or off.
	AcquisitionEnabled = Prefix + "CATALOG_ACQUISITION_ENABLED"

	// AcquisitionSources selects permitted local acquisition source IDs.
	// An explicit empty value disables all local acquisition inputs.
	AcquisitionSources = Prefix + "CATALOG_ACQUISITION_SOURCES"

	// ModelsDevGitCommit pins models.dev Git acquisition to one exact commit.
	ModelsDevGitCommit = Prefix + "CATALOG_MODELS_DEV_GIT_COMMIT"

	// AcquisitionInterval is the acquisition period. Zero means one startup
	// pass while acquisition stays enabled.
	AcquisitionInterval = Prefix + "CATALOG_ACQUISITION_INTERVAL"

	// ProviderBindings selects the complete active provider binding set as a JSON array.
	// An explicit empty array permits no local provider acquisition.
	ProviderBindings = Prefix + "CATALOG_PROVIDER_BINDINGS"

	// CoalesceWindow bounds how long a completed provider observation waits
	// for a slower sibling before it publishes.
	CoalesceWindow = Prefix + "CATALOG_COALESCE_WINDOW"

	// WorkspacePath names the reviewed operator catalog input.
	WorkspacePath = Prefix + "CATALOG_WORKSPACE_PATH"

	// StartupSpread is the stable admission window for cold automatic work.
	StartupSpread = Prefix + "CATALOG_STARTUP_SPREAD"

	// TransferIdleTimeout bounds a transfer that makes no progress.
	TransferIdleTimeout = Prefix + "CATALOG_TRANSFER_IDLE_TIMEOUT"

	// TransferMaxDuration bounds one finite HTTP body transfer.
	TransferMaxDuration = Prefix + "CATALOG_TRANSFER_MAX_DURATION"

	// RefreshTimeout is the optional whole-operation wall-clock cap. Zero adds
	// no cap.
	RefreshTimeout = Prefix + "CATALOG_REFRESH_TIMEOUT"

	// StateDirectory names the process-local runtime state directory.
	StateDirectory = Prefix + "STATE_DIR"

	// SchedulerIdentity replaces the derived stable instance identity.
	SchedulerIdentity = Prefix + "SCHEDULER_IDENTITY"
)

// Lookup reads one setting and reports whether the setting is present.
// os.LookupEnv satisfies it.
type Lookup func(name string) (string, bool)

// setting defines one canonical name, optional runtime option, and parsed host value.
type setting struct {
	name    string
	flag    string
	apply   func(value string) (runtime.Option, error)
	capture func(value string, config *Config) error
}

// Config holds the canonical catalog settings that one process read. It carries
// runtime options and a separate host clock profile. The parser accepts every
// canonical name, the starmap source included. Only Composition rejects a
// source that this build supplies no implementation for.
type Config struct {
	// AuthorityOrigin holds the selected origin declaration. Value reports its presence.
	// The hosting application supplies its catalog store and qualified clock.
	AuthorityOrigin OriginSettings

	// PermissionClock selects native observations and declared bounds for this host.
	// The hosting application composes and validates this profile before runtime startup.
	PermissionClock profile.Config

	// SourceKind is the selected upstream source. The default is public.
	SourceKind runtime.SourceKind

	// GenerationPin names the retained generation selected by configuration.
	GenerationPin string

	// SourceRefreshMode selects automatic or manual source reads.
	SourceRefreshMode runtime.SourceRefreshMode

	// NetworkMode selects configured or offline catalog acquisition.
	NetworkMode runtime.NetworkMode

	// SourceURL is the safe endpoint or file identity of a custom source.
	SourceURL string

	// SchedulerIdentity is the explicit stable instance identity. It spreads
	// the cascade subscriber across the startup window of a fleet. An empty
	// value leaves one process to reconnect at once.
	SchedulerIdentity string

	// SourceAPIKey is the Starmap protocol credential. It never reaches
	// status, a log line, or an error, and only the composition step reads it.
	SourceAPIKey string

	// SourceMaxAge is the age at which the served catalog counts as stale.
	// Zero selects the runtime default.
	SourceMaxAge time.Duration

	// SourceMaxHops bounds an accepted Starmap source chain. Zero selects the
	// runtime default.
	SourceMaxHops int

	// SourceAliases lists the other stable identities of this same runtime.
	SourceAliases []string

	// StartupSpread is the window that admits cold automatic work. It spreads
	// the first cascade link of a fleet. Zero selects the default.
	StartupSpread time.Duration

	// TransferIdleTimeout bounds a transfer that makes no progress. It bounds
	// the cascade subscriber transport too.
	TransferIdleTimeout time.Duration

	// TransferMaxDuration bounds one finite HTTP body transfer of the cascade
	// subscriber. It never bounds an open event stream.
	TransferMaxDuration time.Duration

	// WorkspacePath is the reviewed operator catalog input.
	WorkspacePath string

	// StateDirectory is the process-local runtime state directory.
	StateDirectory string

	// configured names the settings the process supplied, in table order.
	configured []string

	// values retains presence independently from the parsed value.
	values map[string]string

	// options holds runtime options for settings that do not require host composition.
	options []runtime.Option
}

// table returns every canonical setting in its documented order. The order is
// stable, so a report and a test read one sequence.
func table() []setting {
	return append(append([]setting{
		{name: AuthorityOrigin, flag: "catalog-authority-origin", capture: captureAuthorityOrigin},
		{
			name: Source, flag: "catalog-source", capture: captureSourceKind,
			apply: stringOption(runtime.WithCatalogSource),
		},
		{
			name: SourceURL, flag: "catalog-source-url", capture: captureSourceURL,
			apply: stringOption(runtime.WithSourceURL),
		},
		{
			name: SourceAPIKey, flag: "catalog-source-api-key",
			capture: captureSourceAPIKey,
			apply:   stringOption(runtime.WithSourceAPIKey),
		},
		{
			name: SourceRepository, flag: "catalog-source-repository",
			apply: stringOption(runtime.WithSourceRepository),
		},
		{
			name: SourceChannel, flag: "catalog-source-channel",
			apply: stringOption(runtime.WithSourceChannel),
		},
		{
			name: SourceSignerWorkflow, flag: "catalog-source-signer-workflow",
			apply: stringOption(runtime.WithSourceSignerWorkflow),
		},
		{
			name: SourceToken, flag: "catalog-source-token",
			apply: stringOption(runtime.WithSourceToken),
		},
		{
			name: SourceRefreshMode, flag: "catalog-source-refresh-mode",
			apply: stringOption(runtime.WithSourceRefreshMode), capture: captureSourceRefreshMode,
		},
		{
			name: GenerationPin, flag: "catalog-generation-pin",
			apply: stringOption(runtime.WithGenerationPin), capture: func(value string, c *Config) error { c.GenerationPin = value; return nil },
		},
		{
			name: NetworkMode, flag: "catalog-network-mode",
			apply: stringOption(runtime.WithCatalogNetworkMode), capture: captureNetworkMode,
		},
		{
			name: SourcePollInterval, flag: "catalog-source-poll-interval",
			apply: durationOption(SourcePollInterval, runtime.WithSourcePollInterval),
		},
		{
			name: SourceStartupPolicy, flag: "catalog-source-startup-policy",
			apply: stringOption(runtime.WithSourceStartupPolicy),
		},
		{name: SourceAuthorityID, flag: "catalog-source-authority-id", apply: stringOption(runtime.WithSourceAuthorityID)},
		{name: SourcePolicyID, flag: "catalog-source-policy-id", apply: stringOption(runtime.WithSourcePolicyID)},
		{
			name: SourceMaxAge, flag: "catalog-source-max-age",
			capture: captureSourceMaxAge,
			apply:   durationOption(SourceMaxAge, runtime.WithSourceMaxAge),
		},
		{
			name: SourceMaxHops, flag: "catalog-source-max-hops",
			capture: captureSourceMaxHops,
			apply:   intOption(SourceMaxHops, runtime.WithSourceMaxHops),
		},
		{
			name: SourceAliases, flag: "catalog-source-aliases",
			capture: captureSourceAliases,
			apply:   listOption(runtime.WithSourceAliases),
		},
		{
			name: AcquisitionEnabled, flag: "catalog-acquisition-enabled",
			apply: boolOption(AcquisitionEnabled, runtime.WithAcquisitionEnabled),
		},
		{name: AcquisitionSources, flag: "catalog-acquisition-sources", apply: acquisitionSourcesOption},
		{name: ModelsDevGitCommit, flag: "catalog-models-dev-git-commit", apply: modelsDevGitCommitOption},
		{
			name: AcquisitionInterval, flag: "catalog-acquisition-interval",
			apply: durationOption(AcquisitionInterval, runtime.WithAcquisitionInterval),
		},
		{
			name: ProviderBindings, flag: "catalog-provider-bindings",
			apply: providerBindingsOption,
		},
		{
			name: CoalesceWindow, flag: "catalog-coalesce-window",
			apply: durationOption(CoalesceWindow, runtime.WithCoalesceWindow),
		},
		{
			name: WorkspacePath, flag: "catalog-workspace-path", capture: captureWorkspacePath,
			apply: stringOption(workspacePathOption),
		},
		{
			name: StartupSpread, flag: "catalog-startup-spread",
			capture: captureDuration(StartupSpread, func(c *Config, value time.Duration) {
				c.StartupSpread = value
			}),
			apply: durationOption(StartupSpread, runtime.WithStartupSpread),
		},
		{
			name: TransferIdleTimeout, flag: "catalog-transfer-idle-timeout",
			capture: captureDuration(TransferIdleTimeout, func(c *Config, value time.Duration) {
				c.TransferIdleTimeout = value
			}),
			apply: durationOption(TransferIdleTimeout, runtime.WithTransferIdleTimeout),
		},
		{
			name: TransferMaxDuration, flag: "catalog-transfer-max-duration",
			capture: captureDuration(TransferMaxDuration, func(c *Config, value time.Duration) {
				c.TransferMaxDuration = value
			}),
			apply: durationOption(TransferMaxDuration, runtime.WithTransferMaxDuration),
		},
		{
			name: RefreshTimeout, flag: "catalog-refresh-timeout",
			apply: durationOption(RefreshTimeout, runtime.WithRefreshTimeout),
		},
		{
			name: StateDirectory, flag: "state-dir", capture: captureStateDirectory,
			apply: stringOption(runtime.WithStateDirectory),
		},
		{
			name: SchedulerIdentity, flag: "scheduler-identity",
			capture: captureSchedulerIdentity,
			apply:   stringOption(runtime.WithSchedulerIdentity),
		},
	}, clockSettings()...), retentionSettings()...)
}

// Names returns every canonical catalog setting name in documented order.
func Names() []string {
	entries := table()
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.name)
	}
	return names
}

// Flags returns the kebab-case flag name of every canonical setting. A flag
// carries the name of its setting without the prefix.
func Flags() []string {
	entries := table()
	flags := make([]string, 0, len(entries))
	for _, entry := range entries {
		flags = append(flags, entry.flag)
	}
	return flags
}

// Chain reads each lookup in order and returns the first supplied value. An
// earlier lookup wins, so a flag overrides the environment.
func Chain(lookups ...Lookup) Lookup {
	return func(name string) (string, bool) {
		for _, lookup := range lookups {
			if lookup == nil {
				continue
			}
			if value, found := lookup(name); found {
				return value, true
			}
		}
		return "", false
	}
}

// Load reads known names from a caller-supplied lookup.
// An absent value keeps its default. Explicit empty values follow the descriptor.
// Use Parse to reject unknown keys in a complete input map.
func Load(lookup Lookup) (Config, error) {
	if lookup == nil {
		return Config{}, &errors.ValidationError{
			Field: "settings.lookup", Message: "is required",
		}
	}
	config := Config{SourceKind: runtime.SourcePublic, SourceRefreshMode: runtime.SourceRefreshAutomatic, NetworkMode: runtime.NetworkConfigured, values: make(map[string]string)}
	for _, entry := range table() {
		value, found := lookup(entry.name)
		value = strings.TrimSpace(value)
		if !found {
			continue
		}
		if err := validateValue(describe(entry), value); err != nil {
			return Config{}, err
		}
		if entry.apply != nil {
			option, err := entry.apply(value)
			if err != nil {
				return Config{}, err
			}
			config.options = append(config.options, option)
		}
		if entry.capture != nil {
			if err := entry.capture(value, &config); err != nil {
				return Config{}, err
			}
		}
		config.values[entry.name] = value
		config.configured = append(config.configured, entry.name)
	}
	return config, nil
}

// Configured returns the setting names the process supplied, in table order.
func (c Config) Configured() []string { return slices.Clone(c.configured) }

// Options returns the runtime options that the supplied settings select.
// The host must separately compose PermissionClock with the native clock adapter.
func (c Config) Options() []runtime.Option { return slices.Clone(c.options) }

func captureSourceKind(value string, config *Config) error {
	kind, err := runtime.ParseSourceKind(value)
	if err != nil {
		return err
	}
	config.SourceKind = kind
	return nil
}

func captureSourceURL(value string, config *Config) error {
	config.SourceURL = value
	return nil
}

func captureSchedulerIdentity(value string, config *Config) error {
	config.SchedulerIdentity = value
	return nil
}

func captureSourceAPIKey(value string, config *Config) error {
	config.SourceAPIKey = value
	return nil
}

// captureDuration returns the capture of one duration setting. The composition
// step reads the parsed value, so a setting reaches both the runtime option and
// the cascade subscriber.
func captureDuration(
	name string,
	assign func(*Config, time.Duration),
) func(string, *Config) error {
	return func(value string, config *Config) error {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return &errors.ValidationError{
				Field: name, Value: value,
				Message: "must be a duration such as 4h or 30s",
			}
		}
		assign(config, parsed)
		return nil
	}
}

func captureSourceMaxAge(value string, config *Config) error {
	maxAge, err := time.ParseDuration(value)
	if err != nil {
		return &errors.ValidationError{
			Field: SourceMaxAge, Value: value,
			Message: "must be a duration such as 4h or 30s",
		}
	}
	config.SourceMaxAge = maxAge
	return nil
}

func captureSourceMaxHops(value string, config *Config) error {
	hops, err := strconv.Atoi(value)
	if err != nil {
		return &errors.ValidationError{
			Field: SourceMaxHops, Value: value, Message: "must be a whole number",
		}
	}
	config.SourceMaxHops = hops
	return nil
}

func captureSourceAliases(value string, config *Config) error {
	config.SourceAliases = splitList(value)
	return nil
}

// splitList parses a comma-separated identity list. It drops empty entries, so
// a trailing comma is not an error.
func splitList(value string) []string {
	list := make([]string, 0, strings.Count(value, ",")+1)
	for part := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			list = append(list, trimmed)
		}
	}
	return list
}

// listOption passes a comma-separated list to its option.
func listOption(option func(...string) runtime.Option) func(string) (runtime.Option, error) {
	return func(value string) (runtime.Option, error) {
		return option(splitList(value)...), nil
	}
}

func captureWorkspacePath(value string, config *Config) error {
	config.WorkspacePath = value
	return nil
}

func captureStateDirectory(value string, config *Config) error {
	config.StateDirectory = value
	return nil
}

// workspacePathOption adapts the offline workspace-path option to the runtime
// option list, so the one settings table carries both kinds of setting.
func workspacePathOption(path string) runtime.Option {
	return runtime.WithClientOptions(starmap.WithCatalogPath(path))
}

// stringOption passes the raw value to its option. The runtime validates it.
func stringOption(option func(string) runtime.Option) func(string) (runtime.Option, error) {
	return func(value string) (runtime.Option, error) {
		return option(value), nil
	}
}

// durationOption parses a Go duration and returns the option it selects.
func durationOption(
	name string,
	option func(time.Duration) runtime.Option,
) func(string) (runtime.Option, error) {
	return func(value string) (runtime.Option, error) {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return nil, &errors.ValidationError{
				Field: name, Value: value,
				Message: "must be a duration such as 4h or 30s",
			}
		}
		return option(parsed), nil
	}
}

// boolOption parses a boolean and returns the option it selects.
func boolOption(
	name string,
	option func(bool) runtime.Option,
) func(string) (runtime.Option, error) {
	return func(value string) (runtime.Option, error) {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, &errors.ValidationError{
				Field: name, Value: value, Message: "must be true or false",
			}
		}
		return option(parsed), nil
	}
}

// intOption parses a whole number and returns the option it selects.
func intOption(
	name string,
	option func(int) runtime.Option,
) func(string) (runtime.Option, error) {
	return func(value string) (runtime.Option, error) {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, &errors.ValidationError{
				Field: name, Value: value, Message: "must be a whole number",
			}
		}
		return option(parsed), nil
	}
}

func captureSourceRefreshMode(value string, config *Config) error {
	mode, err := runtime.ParseSourceRefreshMode(value)
	config.SourceRefreshMode = mode
	return err
}

func captureNetworkMode(value string, config *Config) error {
	mode, err := runtime.ParseNetworkMode(value)
	config.NetworkMode = mode
	return err
}
