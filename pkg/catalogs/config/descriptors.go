package config

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

// SchemaVersion identifies the descriptor and presence contract.
const SchemaVersion = 1

// ValueType specifies the grammar of one configured value.
type ValueType string

const (
	// StringValue accepts a text value.
	StringValue ValueType = "string"
	// BooleanValue accepts a Go boolean literal.
	BooleanValue ValueType = "boolean"
	// DurationValue accepts a Go duration literal.
	DurationValue ValueType = "duration"
	// IntegerValue accepts a decimal integer.
	IntegerValue ValueType = "integer"
	// ListValue accepts comma-separated identities.
	ListValue ValueType = "string-list"
	// ProviderBindingsValue accepts a JSON array of provider acquisition bindings.
	ProviderBindingsValue ValueType = "provider-bindings"
)

// Scope identifies the authority that owns a setting.
type Scope string

const (
	// DeploymentScope belongs to the selected deployment configuration authority.
	DeploymentScope Scope = "deployment"
	// NodeScope belongs to one process and never inherits another product's value.
	NodeScope Scope = "node"
)

// Descriptor identifies a setting, its grammar, default and ownership.
// Default uses the Type grammar. DefaultMeaning explains host-derived values.
// Environment names describe syntax, not permission to inherit another product's values.
type Descriptor struct {
	Description    string    `json:"description"`
	AllowedValues  []string  `json:"allowed_values,omitempty"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Key            string    `json:"key"`
	Flag           string    `json:"flag"`
	Environment    []string  `json:"environment"`
	Type           ValueType `json:"type"`
	Unit           string    `json:"unit"`
	Default        string    `json:"default"`
	DefaultMeaning string    `json:"default_meaning"`
	AllowEmpty     bool      `json:"allow_empty"`
	AllowZero      bool      `json:"allow_zero"`
	Sensitive      bool      `json:"sensitive"`
	Scope          Scope     `json:"scope"`
	SourceBinding  string    `json:"source_binding"`
	Applicability  []string  `json:"applicability"`
	Mutability     string    `json:"mutability"`
	Introduced     int       `json:"introduced"`
	Compatibility  string    `json:"compatibility"`
	Anchor         string    `json:"anchor"`
}

// Descriptors returns independent metadata in canonical setting order.
func Descriptors() []Descriptor {
	entries := table()
	result := make([]Descriptor, 0, len(entries))
	for _, entry := range entries {
		result = append(result, describe(entry))
	}
	return result
}

func describe(entry setting) Descriptor {
	key := strings.ToLower(strings.TrimPrefix(entry.name, Prefix))
	d := Descriptor{ID: strings.ReplaceAll(key, "_", "."), Name: entry.name, Key: key,
		Flag: entry.flag, Environment: []string{entry.name}, Type: StringValue,
		Scope: DeploymentScope, Applicability: []string{"all"}, Mutability: "runtime-replacement",
		Introduced: SchemaVersion, Compatibility: "supported", Anchor: entry.flag}
	source := runtime.DefaultSourcePolicy()
	acquisition := runtime.DefaultAcquisitionPolicy()
	switch entry.name {
	case Source:
		d.Description = "Selects the upstream catalog source."
		d.Default = string(source.Kind)
		d.AllowedValues = []string{"public", "github", "starmap", "file", "embedded"}
		d.SourceBinding = "catalog-source"
	case SourceURL:
		d.Description = "Names the Starmap endpoint or catalog file."
		d.SourceBinding, d.Applicability = "catalog-source", []string{"starmap", "file"}
		d.DefaultMeaning = "required for a custom URL or file source"
	case SourceRepository:
		d.Description = "Names the GitHub repository that publishes the catalog channel."
		d.Default, d.SourceBinding, d.Applicability = source.Repository, "catalog-source", []string{"public", "github"}
	case SourceChannel:
		d.Description = "Names the branch that holds the catalog channel document."
		d.Default, d.SourceBinding, d.Applicability = source.Channel, "catalog-source", []string{"public", "github"}
	case SourceSignerWorkflow:
		d.Description = "Pins the accepted build provenance workflow."
		d.SourceBinding, d.Applicability = "catalog-source", []string{"public", "github"}
		d.DefaultMeaning = "source adapter's default signing workflow"
	case SourceAPIKey, SourceToken:
		d.Description = "Authenticates transport to the selected catalog source. It is separate from provider credentials."
		d.AllowEmpty, d.Sensitive, d.SourceBinding = true, true, "catalog-source"
		d.DefaultMeaning = "no transport credential"
		d.Applicability = []string{"starmap"}
		if entry.name == SourceToken {
			d.Applicability = []string{"public", "github"}
		}
	case SourcePollInterval:
		d.Description = "Sets the period between automatic catalog checks. Zero disables periodic catalog checks."
		d.Type, d.Unit, d.Default, d.AllowZero = DurationValue, "duration", source.PollInterval.String(), true
	case SourceStartupPolicy:
		d.Description = "Selects catalog availability before the first upstream reply."
		d.Default = string(source.StartupPolicy)
		d.AllowedValues = []string{string(runtime.StartupPreferSource), string(runtime.StartupRequireSource), string(runtime.StartupRequireAuthority), string(runtime.StartupPreferLocal)}
	case SourceAuthorityID, SourcePolicyID:
		d.Description = "Pins an internal authority or permission policy. The require_authority policy needs both identities."
		d.Applicability = []string{"starmap"}
		d.SourceBinding = "catalog-source"
		d.Mutability = "restart"
		d.DefaultMeaning = "no internal authority selected"
	case SourceMaxAge:
		d.Description = "Sets the source freshness warning threshold. Zero keeps the default channel freshness thresholds."
		d.Type, d.Unit, d.Default, d.AllowZero = DurationValue, "duration", source.MaxAge.String(), true
	case SourceMaxHops:
		d.Description = "Limits the accepted source chain length."
		d.Type, d.Unit, d.Default = IntegerValue, "hops", strconv.Itoa(source.MaxHops)
	case SourceAliases:
		d.Description = "Names other identities of this runtime for source cycle detection."
		d.Type, d.AllowEmpty, d.Scope = ListValue, true, NodeScope
	case AcquisitionEnabled:
		d.Description = "Enables automatic acquisition from configured provider and metadata sources."
		d.Type, d.Default = BooleanValue, strconv.FormatBool(acquisition.Enabled)
	case AcquisitionSources:
		d.AllowedValues = []string{string(sources.ProvidersID), string(sources.LocalCatalogID), string(sources.ModelsDevHTTPID), string(sources.ModelsDevGitID)}
		d.Description = "Selects permitted local acquisition inputs. An empty list excludes every acquisition source."
		d.Type, d.AllowEmpty, d.Mutability = ListValue, true, "restart"
		d.DefaultMeaning = "omission keeps host acquisition defaults and existing retained source evidence"
	case ModelsDevGitCommit:
		d.Description = "Pins models.dev Git acquisition to one exact hexadecimal commit. An empty value clears the pin."
		d.AllowEmpty, d.Mutability = true, "restart"
		d.DefaultMeaning = "omission keeps the collector pin. Git acquisition requires an exact commit"
	case AcquisitionInterval:
		d.Description = "Sets the acquisition period. Zero permits one startup pass when automatic acquisition is on."
		d.Type, d.Unit, d.Default, d.AllowZero = DurationValue, "duration", acquisition.Interval.String(), true
	case ProviderBindings:
		d.Description = "Selects the complete active provider binding set. An empty array permits no local provider acquisition."
		d.Type, d.Mutability = ProviderBindingsValue, "restart"
		d.DefaultMeaning = "omission retains legacy unscoped acquisition; an explicit array selects only its declared bindings"
	case CoalesceWindow:
		d.Description = "Bounds the wait before completed provider observations publish."
		d.Type, d.Unit, d.Default = DurationValue, "duration", runtime.DefaultCoalesceWindow.String()
	case StartupSpread:
		d.Description = "Spreads cold automatic work across a stable time window. Zero disables the spread."
		d.Type, d.Unit, d.Default, d.AllowZero = DurationValue, "duration", fleet.DefaultStartupSpread.String(), true
	case TransferIdleTimeout:
		d.Description = "Bounds a transfer that makes no progress."
		d.Type, d.Unit, d.Default = DurationValue, "duration", runtime.DefaultTransferIdleTimeout.String()
	case TransferMaxDuration:
		d.Description = "Bounds one finite HTTP body transfer. It does not bound an open event stream."
		d.Type, d.Unit, d.Default = DurationValue, "duration", runtime.DefaultTransferMaxDuration.String()
	case RefreshTimeout:
		d.Description = "Limits the time for one refresh. Zero adds no cap."
		d.Type, d.Unit, d.Default, d.AllowZero = DurationValue, "duration", runtime.DefaultRefreshTimeout.String(), true
	case WorkspacePath:
		d.Description = "Names the reviewed operator catalog input."
		d.Scope, d.AllowEmpty, d.DefaultMeaning = NodeScope, true, "hosting application's workspace"
	case StateDirectory:
		d.Description = "Names the runtime state directory for this process."
		d.Scope, d.DefaultMeaning = NodeScope, "hosting application's state directory"
	case SchedulerIdentity:
		d.Description = "Sets a stable identity for this runtime instance."
		d.Scope, d.AllowEmpty, d.DefaultMeaning = NodeScope, true, "runtime-derived instance identity"
	}
	if d.Scope == NodeScope {
		d.Mutability = "restart"
	}
	return d
}

func validateValue(d Descriptor, value string) error {
	invalid := func(message string) error { return &errors.ValidationError{Field: d.Name, Message: message} }
	if value == "" {
		if !d.AllowEmpty {
			return invalid("must not be empty")
		}
		return nil
	}
	switch d.Type {
	case DurationValue:
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return invalid("must be a duration such as 4h or 30s")
		}
		if parsed < 0 || parsed == 0 && !d.AllowZero {
			return invalid("is outside the permitted duration range")
		}
	case IntegerValue:
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return invalid("must be a positive integer")
		}
	case BooleanValue:
		if _, err := strconv.ParseBool(value); err != nil {
			return invalid("must be true or false")
		}
	case ListValue:
		values := splitList(value)
		if len(values) > runtime.MaxSourceAliases {
			return invalid("exceeds the alias count limit")
		}
		for _, alias := range values {
			if len(alias) > runtime.MaxSourceAliasBytes {
				return invalid("exceeds the alias length limit")
			}
		}
	}
	if d.Name == SourceStartupPolicy {
		_, err := runtime.ParseStartupPolicy(value)
		return err
	}
	return nil
}

// Parse rejects unknown canonical names and preserves explicit values.
// It never reads the process environment, a file, or the network.
func Parse(values map[string]string) (Config, error) {
	if err := validateNames(values); err != nil {
		return Config{}, err
	}
	return Load(func(name string) (string, bool) { value, found := values[name]; return value, found })
}

// Value returns a supplied value and its presence. Credential values are secret.
// An absent value does not become present because a descriptor has a default.
func (c Config) Value(name string) (string, bool) {
	value, present := c.values[name]
	return value, present
}

func validateNames(values map[string]string) error {
	names := Names()
	for name := range values {
		if !slices.Contains(names, name) {
			return &errors.ValidationError{Field: name, Message: "is not a supported catalog setting"}
		}
	}
	return nil
}
