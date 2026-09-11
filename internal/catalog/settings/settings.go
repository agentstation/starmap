// Package settings adapts the public catalog configuration to Starmap commands.
package settings

import (
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/spf13/pflag"
)

// Canonical names remain available to application composition.
const (
	Prefix                                  = catalogconfig.Prefix
	AuthorityOrigin                         = catalogconfig.AuthorityOrigin
	Source                                  = catalogconfig.Source
	SourceURL                               = catalogconfig.SourceURL
	SourceAPIKey                            = catalogconfig.SourceAPIKey
	SourceRepository                        = catalogconfig.SourceRepository
	SourceChannel                           = catalogconfig.SourceChannel
	SourceSignerWorkflow                    = catalogconfig.SourceSignerWorkflow
	SourceToken                             = catalogconfig.SourceToken
	SourceRefreshMode                       = catalogconfig.SourceRefreshMode
	GenerationPin                           = catalogconfig.GenerationPin
	NetworkMode                             = catalogconfig.NetworkMode
	SourcePollInterval                      = catalogconfig.SourcePollInterval
	SourceStartupPolicy                     = catalogconfig.SourceStartupPolicy
	SourceAuthorityID                       = catalogconfig.SourceAuthorityID
	SourcePolicyID                          = catalogconfig.SourcePolicyID
	SourceMaxAge                            = catalogconfig.SourceMaxAge
	SourceMaxHops                           = catalogconfig.SourceMaxHops
	SourceAliases                           = catalogconfig.SourceAliases
	AcquisitionSources                      = catalogconfig.AcquisitionSources
	ModelsDevGitCommit                      = catalogconfig.ModelsDevGitCommit
	AcquisitionEnabled                      = catalogconfig.AcquisitionEnabled
	AcquisitionInterval                     = catalogconfig.AcquisitionInterval
	ProviderBindings                        = catalogconfig.ProviderBindings
	CoalesceWindow                          = catalogconfig.CoalesceWindow
	WorkspacePath                           = catalogconfig.WorkspacePath
	StartupSpread                           = catalogconfig.StartupSpread
	TransferIdleTimeout                     = catalogconfig.TransferIdleTimeout
	TransferMaxDuration                     = catalogconfig.TransferMaxDuration
	RefreshTimeout                          = catalogconfig.RefreshTimeout
	StateDirectory                          = catalogconfig.StateDirectory
	SchedulerIdentity                       = catalogconfig.SchedulerIdentity
	PermissionClockSource                   = catalogconfig.PermissionClockSource
	PermissionClockRefreshInterval          = catalogconfig.PermissionClockRefreshInterval
	PermissionClockMaxAge                   = catalogconfig.PermissionClockMaxAge
	PermissionClockMaxDriftPPM              = catalogconfig.PermissionClockMaxDriftPPM
	PermissionClockCounterUncertainty       = catalogconfig.PermissionClockCounterUncertainty
	PermissionClockWindowsMaxSourceAge      = catalogconfig.PermissionClockWindowsMaxSourceAge
	PermissionClockWindowsMaxSourceDriftPPM = catalogconfig.PermissionClockWindowsMaxSourceDriftPPM
	PermissionClockWindowsSourceUncertainty = catalogconfig.PermissionClockWindowsSourceUncertainty
)

// Config is the shared parsed catalog configuration.
type Config = catalogconfig.Config

// Lookup is a caller-supplied setting reader.
type Lookup = catalogconfig.Lookup

// Names returns the canonical environment names.
func Names() []string { return catalogconfig.Names() }

// Flags returns the canonical command flags.
func Flags() []string { return catalogconfig.Flags() }

// Load reads the canonical settings through the supplied reader.
func Load(lookup Lookup) (Config, error) { return catalogconfig.Load(lookup) }

// Chain returns the first supplied value, including an explicit empty value.
func Chain(lookups ...Lookup) Lookup { return catalogconfig.Chain(lookups...) }

// RegisterFlags registers one string flag for every canonical catalog setting.
// A flag carries the kebab-case name of its setting. A flag value follows the
// same grammar as the environment value, so one parser reads both.
func RegisterFlags(flags *pflag.FlagSet) error {
	if flags == nil {
		return &errors.ValidationError{Field: "settings.flags", Message: "is required"}
	}
	for _, entry := range catalogconfig.Descriptors() {
		if flags.Lookup(entry.Flag) != nil {
			continue
		}
		flags.String(entry.Flag, "", entry.Description+" Environment: "+entry.Name)
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
		for _, entry := range catalogconfig.Descriptors() {
			if entry.Name != name {
				continue
			}
			flag := flags.Lookup(entry.Flag)
			if flag == nil || !flag.Changed {
				return "", false
			}
			return flag.Value.String(), true
		}
		return "", false
	}
}
