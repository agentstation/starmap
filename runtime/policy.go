package runtime

import (
	"slices"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// SourceKind names one supported upstream catalog source. Selection is
// terminal: a deployment that names a custom source never falls back to the
// public GitHub channel.
type SourceKind string

const (
	// SourcePublic reads the attested public GitHub catalog channel of the
	// Starmap project. It is the default.
	SourcePublic SourceKind = "public"

	// SourceGitHub reads an attested catalog channel from a named repository.
	SourceGitHub SourceKind = "github"

	// SourceStarmap reads from another Starmap runtime. The cascade client is
	// an injected composition, because the root package holds no HTTP client.
	SourceStarmap SourceKind = "starmap"

	// SourceFile reads one immutable generation from a local path.
	SourceFile SourceKind = "file"

	// SourceEmbedded pins the runtime to the verified embedded bootstrap and
	// contacts no external system.
	SourceEmbedded SourceKind = "embedded"
)

// sourceKinds lists every accepted source name in declaration order.
var sourceKinds = []SourceKind{
	SourcePublic,
	SourceGitHub,
	SourceStarmap,
	SourceFile,
	SourceEmbedded,
}

// SourceKinds returns a caller-owned copy of every accepted source name.
func SourceKinds() []SourceKind { return slices.Clone(sourceKinds) }

// Valid reports whether the kind is one of the accepted source names.
func (k SourceKind) Valid() bool { return slices.Contains(sourceKinds, k) }

// String returns the wire value of the source kind.
func (k SourceKind) String() string { return string(k) }

// Custom reports whether the kind names a deployment-owned source. A custom
// source never falls back to the public channel.
func (k SourceKind) Custom() bool { return k.Valid() && k != SourcePublic }

// ParseSourceKind converts one configured name into a source kind. It rejects
// every unknown name with a typed validation error, so a typo never selects a
// silent default.
func ParseSourceKind(name string) (SourceKind, error) {
	kind := SourceKind(name)
	if !kind.Valid() {
		return "", &errors.ValidationError{
			Field:   "catalog_source",
			Value:   name,
			Message: "must be public, github, starmap, file, or embedded",
		}
	}
	return kind, nil
}

// StartupPolicy decides what the runtime serves while the first source read is
// still outstanding.
type StartupPolicy string

const (
	// StartupPreferSource keeps the embedded baseline active and replaces it
	// with the first verified upstream generation. It is the default.
	StartupPreferSource StartupPolicy = "prefer_source"

	// StartupRequireSource keeps the runtime unusable until one verified
	// upstream generation is active. Open reads the source one time and fails
	// when that read fails. The policy names the evidence the runtime needs,
	// not the lease. A replica that another instance owns therefore opens
	// without a read and consumes the state that the owner publishes.
	StartupRequireSource StartupPolicy = "require_source"

	// StartupPreferLocal keeps the retained local generation active and
	// applies an upstream generation only on an explicit refresh.
	StartupPreferLocal StartupPolicy = "prefer_local"
)

// startupPolicies lists every accepted startup policy.
var startupPolicies = []StartupPolicy{
	StartupPreferSource,
	StartupRequireSource,
	StartupPreferLocal,
}

// Valid reports whether the policy is one of the accepted names.
func (p StartupPolicy) Valid() bool { return slices.Contains(startupPolicies, p) }

// String returns the wire value of the startup policy.
func (p StartupPolicy) String() string { return string(p) }

// ParseStartupPolicy converts one configured name into a startup policy.
func ParseStartupPolicy(name string) (StartupPolicy, error) {
	policy := StartupPolicy(name)
	if !policy.Valid() {
		return "", &errors.ValidationError{
			Field:   "catalog_source_startup_policy",
			Value:   name,
			Message: "must be prefer_source, require_source, or prefer_local",
		}
	}
	return policy, nil
}

// Source policy defaults. They match the canonical CATALOG_SOURCE_* settings.
const (
	// DefaultSourceRepository is the public catalog repository.
	DefaultSourceRepository = "agentstation/starmap"

	// DefaultSourceChannel is the mutable release that names the current
	// immutable catalog release.
	DefaultSourceChannel = "catalog-latest"

	// DefaultSourcePollInterval is how often the runtime checks the channel.
	DefaultSourcePollInterval = time.Hour

	// DefaultSourceMaxAge is the age at which a served catalog is stale.
	DefaultSourceMaxAge = 6 * time.Hour

	// DefaultSourceMaxHops bounds a cascade of Starmap runtimes.
	DefaultSourceMaxHops = 8

	// MaxSourceAliases bounds the declared alias list. A deployment names a
	// few addresses of one node, so a longer list is a configuration error.
	MaxSourceAliases = 16

	// MaxSourceAliasBytes bounds one declared alias.
	MaxSourceAliasBytes = 128
)

// SourcePolicy selects and bounds the upstream catalog source. It holds no
// token and no API key, so a policy value is safe to log and to serve.
type SourcePolicy struct {
	// Kind selects the source implementation.
	Kind SourceKind

	// URL is the deployment-owned source address. It applies to the starmap
	// and file kinds.
	URL string

	// Repository names the catalog repository for the public and github kinds.
	Repository string

	// Channel names the mutable release that selects the current catalog.
	Channel string

	// SignerWorkflow pins the build provenance the source accepts.
	SignerWorkflow string

	// PollInterval is the channel check period.
	PollInterval time.Duration

	// StartupPolicy decides what the runtime serves before the first reply.
	StartupPolicy StartupPolicy

	// MaxAge is the age at which the active catalog counts as stale.
	MaxAge time.Duration

	// MaxHops bounds a cascade of Starmap runtimes.
	MaxHops int

	// Aliases are the other stable identities that name this same runtime,
	// such as a load-balancer name or a second deployment name. A served
	// source chain that names one of them is a self reference, and address
	// comparison alone cannot detect it.
	Aliases []string
}

// DefaultSourcePolicy returns the canonical public-channel source policy.
func DefaultSourcePolicy() SourcePolicy {
	return SourcePolicy{
		Kind:          SourcePublic,
		Repository:    DefaultSourceRepository,
		Channel:       DefaultSourceChannel,
		PollInterval:  DefaultSourcePollInterval,
		StartupPolicy: StartupPreferSource,
		MaxAge:        DefaultSourceMaxAge,
		MaxHops:       DefaultSourceMaxHops,
	}
}

// Validate checks the policy fields that the runtime depends on.
func (p SourcePolicy) Validate() error {
	if !p.Kind.Valid() {
		return &errors.ValidationError{
			Field: "source_policy.kind", Value: p.Kind, Message: "is not a supported source",
		}
	}
	if !p.StartupPolicy.Valid() {
		return &errors.ValidationError{
			Field: "source_policy.startup_policy", Value: p.StartupPolicy,
			Message: "is not a supported startup policy",
		}
	}
	if p.PollInterval < 0 {
		return &errors.ValidationError{
			Field: "source_policy.poll_interval", Value: p.PollInterval, Message: "must not be negative",
		}
	}
	if p.MaxAge < 0 {
		return &errors.ValidationError{
			Field: "source_policy.max_age", Value: p.MaxAge, Message: "must not be negative",
		}
	}
	if p.MaxHops < 1 {
		return &errors.ValidationError{
			Field: "source_policy.max_hops", Value: p.MaxHops, Message: "must be at least one",
		}
	}
	if len(p.Aliases) > MaxSourceAliases {
		return &errors.ValidationError{
			Field: "source_policy.aliases", Value: len(p.Aliases),
			Message: "exceeds the alias budget",
		}
	}
	for _, alias := range p.Aliases {
		if alias == "" || len(alias) > MaxSourceAliasBytes {
			return &errors.ValidationError{
				Field: "source_policy.aliases", Value: len(alias),
				Message: "must be a bounded nonempty identity",
			}
		}
	}
	switch p.Kind {
	case SourcePublic, SourceGitHub:
		if p.Repository == "" {
			return &errors.ValidationError{
				Field: "source_policy.repository", Message: "is required for a GitHub channel",
			}
		}
		if p.Channel == "" {
			return &errors.ValidationError{
				Field: "source_policy.channel", Message: "is required for a GitHub channel",
			}
		}
	case SourceFile, SourceStarmap:
		if p.URL == "" {
			return &errors.ValidationError{
				Field:   "source_policy.url",
				Message: "is required for the file and starmap sources",
			}
		}
	case SourceEmbedded:
	}
	return nil
}

// SafeIdentity returns the source identity that status and logs may show. It
// names the kind and, for a GitHub channel, the repository and the channel. It
// never names a custom URL, a host, or a credential.
func (p SourcePolicy) SafeIdentity() string {
	switch p.Kind {
	case SourcePublic:
		return "public_github"
	case SourceGitHub:
		return "github:" + p.Repository + "#" + p.Channel
	case SourceStarmap:
		return "starmap_cascade"
	case SourceFile:
		return "local_file"
	case SourceEmbedded:
		return "embedded"
	default:
		return "unknown"
	}
}

// Acquisition policy defaults. Acquisition has exactly two settings: whether
// it runs, and how often. No start-time or mode setting exists.
const (
	// DefaultAcquisitionInterval is the provider acquisition period.
	DefaultAcquisitionInterval = 4 * time.Hour
)

// AcquisitionPolicy configures scheduled provider acquisition. The policy is
// exactly one switch and one period.
type AcquisitionPolicy struct {
	// Enabled reports whether scheduled provider acquisition runs.
	Enabled bool

	// Interval is the acquisition period. The scheduler places each instance
	// at a stable phase inside the interval. Zero means one startup pass and
	// no periodic work.
	Interval time.Duration
}

// DefaultAcquisitionPolicy returns the canonical acquisition policy.
func DefaultAcquisitionPolicy() AcquisitionPolicy {
	return AcquisitionPolicy{Enabled: true, Interval: DefaultAcquisitionInterval}
}

// Validate checks the acquisition period. Zero is valid and selects one
// startup pass, so an operator schedules a single run without a cadence.
func (p AcquisitionPolicy) Validate() error {
	if p.Interval < 0 {
		return &errors.ValidationError{
			Field: "acquisition_policy.interval", Value: p.Interval,
			Message: "must not be negative",
		}
	}
	return nil
}

// Freshness thresholds. They follow the four-hour publication cadence, the
// one-hour channel poll, and the six-hour end-to-end objective.
const (
	// FreshnessChannelWarnAge is the served-catalog age that warns. A source
	// maximum age above zero replaces it, unless the caller supplies an
	// explicit freshness policy.
	FreshnessChannelWarnAge = 6 * time.Hour

	// FreshnessChannelCriticalAge is the served-catalog age that is critical.
	// A source maximum age above zero replaces it with the scaled age, unless
	// the caller supplies an explicit freshness policy.
	FreshnessChannelCriticalAge = 10 * time.Hour

	// FreshnessSourceCheckWarnAge is the source-check age that warns.
	FreshnessSourceCheckWarnAge = 90 * time.Minute

	// FreshnessSourceCheckCriticalAge is the source-check age that is critical.
	FreshnessSourceCheckCriticalAge = 2 * time.Hour

	// FreshnessAcquisitionWarnAge is the acquisition-success age that warns.
	FreshnessAcquisitionWarnAge = 5 * time.Hour

	// FreshnessAcquisitionCriticalAge is the acquisition-success age that is
	// critical.
	FreshnessAcquisitionCriticalAge = 8 * time.Hour
)

// FreshnessPolicy holds the age thresholds that turn observed timestamps into
// a freshness level.
type FreshnessPolicy struct {
	ChannelWarnAge     time.Duration
	ChannelCriticalAge time.Duration

	SourceCheckWarnAge     time.Duration
	SourceCheckCriticalAge time.Duration

	AcquisitionWarnAge     time.Duration
	AcquisitionCriticalAge time.Duration
}

// DefaultFreshnessPolicy returns the canonical freshness thresholds.
func DefaultFreshnessPolicy() FreshnessPolicy {
	return FreshnessPolicy{
		ChannelWarnAge:         FreshnessChannelWarnAge,
		ChannelCriticalAge:     FreshnessChannelCriticalAge,
		SourceCheckWarnAge:     FreshnessSourceCheckWarnAge,
		SourceCheckCriticalAge: FreshnessSourceCheckCriticalAge,
		AcquisitionWarnAge:     FreshnessAcquisitionWarnAge,
		AcquisitionCriticalAge: FreshnessAcquisitionCriticalAge,
	}
}

// Channel threshold derivation. The source maximum age names the age at which
// the served catalog is stale, so it names the channel warning age too. The
// critical age keeps the ratio that the two defaults hold, in the lowest
// terms: ten hours over six hours reduces to five over three.
const (
	channelCriticalNumerator   = 5
	channelCriticalDenominator = 3

	// maxDuration is the largest representable duration. A derivation that
	// cannot order the two thresholds returns it, so no channel age ever
	// reaches the critical grade.
	maxDuration = time.Duration(1<<63 - 1)
)

// withChannelMaxAge returns the policy with the channel thresholds that a
// source maximum age implies. The warning age becomes the maximum age, and the
// critical age scales it by the default ratio. A maximum age of zero or less
// returns the policy unchanged.
func (p FreshnessPolicy) withChannelMaxAge(maxAge time.Duration) FreshnessPolicy {
	if maxAge <= 0 {
		return p
	}
	critical := maxAge / channelCriticalDenominator * channelCriticalNumerator
	remainder := maxAge % channelCriticalDenominator
	critical += remainder * channelCriticalNumerator / channelCriticalDenominator
	// A tiny age rounds down to the warning age, and a huge age overflows to a
	// negative one. Both keep the pair ordered by never reaching critical.
	if critical <= maxAge {
		critical = maxDuration
	}
	p.ChannelWarnAge = maxAge
	p.ChannelCriticalAge = critical
	return p
}

// Validate checks that every warning threshold precedes its critical partner.
func (p FreshnessPolicy) Validate() error {
	pairs := []struct {
		field    string
		warn     time.Duration
		critical time.Duration
	}{
		{"channel", p.ChannelWarnAge, p.ChannelCriticalAge},
		{"source_check", p.SourceCheckWarnAge, p.SourceCheckCriticalAge},
		{"acquisition", p.AcquisitionWarnAge, p.AcquisitionCriticalAge},
	}
	for _, pair := range pairs {
		if pair.warn <= 0 || pair.critical <= 0 {
			return &errors.ValidationError{
				Field: "freshness_policy." + pair.field, Message: "thresholds must be positive",
			}
		}
		if pair.warn >= pair.critical {
			return &errors.ValidationError{
				Field:   "freshness_policy." + pair.field,
				Message: "the warning threshold must precede the critical threshold",
			}
		}
	}
	return nil
}

// Freshness is the evaluated age of one observed timestamp.
type Freshness string

const (
	// FreshnessUnknown means no observation supports an evaluation yet.
	FreshnessUnknown Freshness = "unknown"

	// FreshnessCurrent means the age is inside every threshold.
	FreshnessCurrent Freshness = "current"

	// FreshnessWarn means the age passed the warning threshold.
	FreshnessWarn Freshness = "warn"

	// FreshnessCritical means the age passed the critical threshold.
	FreshnessCritical Freshness = "critical"
)

// String returns the wire value of the freshness level.
func (f Freshness) String() string { return string(f) }

// evaluate turns one age into a freshness level.
func evaluateFreshness(age, warn, critical time.Duration) Freshness {
	switch {
	case age < 0:
		return FreshnessUnknown
	case age >= critical:
		return FreshnessCritical
	case age >= warn:
		return FreshnessWarn
	default:
		return FreshnessCurrent
	}
}

// Health is the operator-facing state of one runtime component.
type Health string

const (
	// HealthUnknown means the component has not reported yet.
	HealthUnknown Health = "unknown"

	// HealthOK means the component reached its last objective.
	HealthOK Health = "ok"

	// HealthDegraded means the component works with reduced evidence.
	HealthDegraded Health = "degraded"

	// HealthUnavailable means the component cannot reach its dependency.
	HealthUnavailable Health = "unavailable"
)

// String returns the wire value of the health state.
func (h Health) String() string { return string(h) }

// worseHealth returns the more serious of two health values.
func worseHealth(left, right Health) Health {
	rank := map[Health]int{
		HealthOK:          0,
		HealthUnknown:     1,
		HealthDegraded:    2,
		HealthUnavailable: 3,
	}
	if rank[right] > rank[left] {
		return right
	}
	return left
}
