// Package settings owns the canonical catalog settings of one Starmap process.
// It maps every STARMAP_ catalog name onto exactly one connected-runtime
// option. No other catalog setting name exists.
package settings

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/errors"
)

// Prefix is the canonical Starmap environment prefix. Starport uses STARPORT_
// with the same suffixes and the same defaults.
const Prefix = "STARMAP_"

// The canonical catalog setting names. Each name selects one runtime option.
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

	// SourceChannel names the mutable attested discovery channel.
	SourceChannel = Prefix + "CATALOG_SOURCE_CHANNEL"

	// SourceSignerWorkflow pins the accepted build provenance.
	SourceSignerWorkflow = Prefix + "CATALOG_SOURCE_SIGNER_WORKFLOW"

	// SourceToken is the optional GitHub API token. Public local use needs no
	// token.
	SourceToken = Prefix + "CATALOG_SOURCE_TOKEN"

	// SourcePollInterval is the conditional channel check period.
	SourcePollInterval = Prefix + "CATALOG_SOURCE_POLL_INTERVAL"

	// SourceStartupPolicy decides what the runtime serves before the first
	// upstream reply.
	SourceStartupPolicy = Prefix + "CATALOG_SOURCE_STARTUP_POLICY"

	// SourceMaxAge is the source freshness warning objective.
	SourceMaxAge = Prefix + "CATALOG_SOURCE_MAX_AGE"

	// SourceMaxHops bounds an accepted Starmap source chain.
	SourceMaxHops = Prefix + "CATALOG_SOURCE_MAX_HOPS"

	// SourceAliases lists the other stable identities of this same runtime, as
	// a comma-separated list. A served source chain that names one of them is
	// a self reference.
	SourceAliases = Prefix + "CATALOG_SOURCE_ALIASES"

	// AcquisitionEnabled turns automatic provider acquisition on or off.
	AcquisitionEnabled = Prefix + "CATALOG_ACQUISITION_ENABLED"

	// AcquisitionInterval is the acquisition period. Zero means one startup
	// pass while acquisition stays enabled.
	AcquisitionInterval = Prefix + "CATALOG_ACQUISITION_INTERVAL"

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

// setting is one canonical name and the runtime option that it selects.
type setting struct {
	name    string
	flag    string
	apply   func(value string) (starmap.Option, error)
	capture func(value string, config *Config) error
}

// Config holds the canonical catalog settings that one process read. It carries
// one runtime option for each supplied setting. The parser accepts every
// canonical name, the starmap source included. Only Composition rejects a
// source that this build supplies no implementation for.
type Config struct {
	// SourceKind is the selected upstream source. The default is public.
	SourceKind starmap.SourceKind

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

	// options holds one runtime option for each supplied setting.
	options []starmap.Option
}

// table returns every canonical setting in its documented order. The order is
// stable, so a report and a test read one sequence.
func table() []setting {
	return []setting{
		{
			name: Source, flag: "catalog-source", capture: captureSourceKind,
			apply: stringOption(starmap.WithCatalogSource),
		},
		{
			name: SourceURL, flag: "catalog-source-url", capture: captureSourceURL,
			apply: stringOption(starmap.WithSourceURL),
		},
		{
			name: SourceAPIKey, flag: "catalog-source-api-key",
			capture: captureSourceAPIKey,
			apply:   stringOption(starmap.WithSourceAPIKey),
		},
		{
			name: SourceRepository, flag: "catalog-source-repository",
			apply: stringOption(starmap.WithSourceRepository),
		},
		{
			name: SourceChannel, flag: "catalog-source-channel",
			apply: stringOption(starmap.WithSourceChannel),
		},
		{
			name: SourceSignerWorkflow, flag: "catalog-source-signer-workflow",
			apply: stringOption(starmap.WithSourceSignerWorkflow),
		},
		{
			name: SourceToken, flag: "catalog-source-token",
			apply: stringOption(starmap.WithSourceToken),
		},
		{
			name: SourcePollInterval, flag: "catalog-source-poll-interval",
			apply: durationOption(SourcePollInterval, starmap.WithSourcePollInterval),
		},
		{
			name: SourceStartupPolicy, flag: "catalog-source-startup-policy",
			apply: stringOption(starmap.WithSourceStartupPolicy),
		},
		{
			name: SourceMaxAge, flag: "catalog-source-max-age",
			capture: captureSourceMaxAge,
			apply:   durationOption(SourceMaxAge, starmap.WithSourceMaxAge),
		},
		{
			name: SourceMaxHops, flag: "catalog-source-max-hops",
			capture: captureSourceMaxHops,
			apply:   intOption(SourceMaxHops, starmap.WithSourceMaxHops),
		},
		{
			name: SourceAliases, flag: "catalog-source-aliases",
			capture: captureSourceAliases,
			apply:   listOption(starmap.WithSourceAliases),
		},
		{
			name: AcquisitionEnabled, flag: "catalog-acquisition-enabled",
			apply: boolOption(AcquisitionEnabled, starmap.WithAcquisitionEnabled),
		},
		{
			name: AcquisitionInterval, flag: "catalog-acquisition-interval",
			apply: durationOption(AcquisitionInterval, starmap.WithAcquisitionInterval),
		},
		{
			name: CoalesceWindow, flag: "catalog-coalesce-window",
			apply: durationOption(CoalesceWindow, starmap.WithCoalesceWindow),
		},
		{
			name: WorkspacePath, flag: "catalog-workspace-path", capture: captureWorkspacePath,
			apply: stringOption(starmap.WithCatalogPath),
		},
		{
			name: StartupSpread, flag: "catalog-startup-spread",
			capture: captureDuration(StartupSpread, func(c *Config, value time.Duration) {
				c.StartupSpread = value
			}),
			apply: durationOption(StartupSpread, starmap.WithStartupSpread),
		},
		{
			name: TransferIdleTimeout, flag: "catalog-transfer-idle-timeout",
			capture: captureDuration(TransferIdleTimeout, func(c *Config, value time.Duration) {
				c.TransferIdleTimeout = value
			}),
			apply: durationOption(TransferIdleTimeout, starmap.WithTransferIdleTimeout),
		},
		{
			name: TransferMaxDuration, flag: "catalog-transfer-max-duration",
			capture: captureDuration(TransferMaxDuration, func(c *Config, value time.Duration) {
				c.TransferMaxDuration = value
			}),
			apply: durationOption(TransferMaxDuration, starmap.WithTransferMaxDuration),
		},
		{
			name: RefreshTimeout, flag: "catalog-refresh-timeout",
			apply: durationOption(RefreshTimeout, starmap.WithRefreshTimeout),
		},
		{
			name: StateDirectory, flag: "state-dir", capture: captureStateDirectory,
			apply: stringOption(starmap.WithStateDirectory),
		},
		{
			name: SchedulerIdentity, flag: "scheduler-identity",
			capture: captureSchedulerIdentity,
			apply:   stringOption(starmap.WithSchedulerIdentity),
		},
	}
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

// RegisterFlags registers one string flag for every canonical catalog setting.
// A flag carries the kebab-case name of its setting. A flag value follows the
// same grammar as the environment value, so one parser reads both.
func RegisterFlags(flags *pflag.FlagSet) error {
	if flags == nil {
		return &errors.ValidationError{Field: "settings.flags", Message: "is required"}
	}
	for _, entry := range table() {
		if flags.Lookup(entry.flag) != nil {
			continue
		}
		flags.String(entry.flag, "", "sets "+entry.name)
	}
	return nil
}

// FlagLookup reads a canonical setting from a registered flag. It reports only
// a flag that the caller changed, so an untouched flag never replaces the
// environment value.
func FlagLookup(flags *pflag.FlagSet) Lookup {
	return func(name string) (string, bool) {
		if flags == nil {
			return "", false
		}
		for _, entry := range table() {
			if entry.name != name {
				continue
			}
			flag := flags.Lookup(entry.flag)
			if flag == nil || !flag.Changed {
				return "", false
			}
			return flag.Value.String(), true
		}
		return "", false
	}
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

// Load reads every canonical catalog setting through lookup. An absent or empty
// setting keeps its canonical default. A supplied setting that names no valid
// value returns a typed validation error.
func Load(lookup Lookup) (Config, error) {
	if lookup == nil {
		return Config{}, &errors.ValidationError{
			Field: "settings.lookup", Message: "is required",
		}
	}
	config := Config{SourceKind: starmap.SourcePublic}
	for _, entry := range table() {
		value, found := lookup(entry.name)
		value = strings.TrimSpace(value)
		if !found || value == "" {
			continue
		}
		option, err := entry.apply(value)
		if err != nil {
			return Config{}, err
		}
		if entry.capture != nil {
			if err := entry.capture(value, &config); err != nil {
				return Config{}, err
			}
		}
		config.configured = append(config.configured, entry.name)
		config.options = append(config.options, option)
	}
	return config, nil
}

// Configured returns the setting names the process supplied, in table order.
func (c Config) Configured() []string { return slices.Clone(c.configured) }

// Options returns the runtime options that the supplied settings select.
func (c Config) Options() []starmap.Option { return slices.Clone(c.options) }

func captureSourceKind(value string, config *Config) error {
	kind, err := starmap.ParseSourceKind(value)
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
	parts := strings.Split(value, ",")
	list := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			list = append(list, trimmed)
		}
	}
	return list
}

// listOption passes a comma-separated list to its option.
func listOption(option func(...string) starmap.Option) func(string) (starmap.Option, error) {
	return func(value string) (starmap.Option, error) {
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

// stringOption passes the raw value to its option. The runtime validates it.
func stringOption(option func(string) starmap.Option) func(string) (starmap.Option, error) {
	return func(value string) (starmap.Option, error) {
		return option(value), nil
	}
}

// durationOption parses a Go duration and returns the option it selects.
func durationOption(
	name string,
	option func(time.Duration) starmap.Option,
) func(string) (starmap.Option, error) {
	return func(value string) (starmap.Option, error) {
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
	option func(bool) starmap.Option,
) func(string) (starmap.Option, error) {
	return func(value string) (starmap.Option, error) {
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
	option func(int) starmap.Option,
) func(string) (starmap.Option, error) {
	return func(value string) (starmap.Option, error) {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, &errors.ValidationError{
				Field: name, Value: value, Message: "must be a whole number",
			}
		}
		return option(parsed), nil
	}
}
