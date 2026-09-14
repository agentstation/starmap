package publication

import (
	"bytes"
	"io"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/sources"
)

// MaxPublicationProfileBytes bounds a publisher's explicit source policy.
const MaxPublicationProfileBytes = 4 << 20

// ParseProfile reads one strict YAML source policy with explicit boolean settings.
// Durations use Go duration syntax. Unknown and duplicate fields are invalid.
func ParseProfile(data []byte) (Profile, error) {
	if len(data) == 0 || len(data) > MaxPublicationProfileBytes {
		return Profile{}, admissionError("profile", "requires bounded policy data")
	}
	var wire struct {
		Version string `yaml:"policy_version"`
		Scopes  []struct {
			Source         sources.ID                          `yaml:"source"`
			Binding        *sources.ProviderAcquisitionBinding `yaml:"binding"`
			Required       *bool                               `yaml:"required"`
			Enabled        *bool                               `yaml:"enabled"`
			AllowMissing   *bool                               `yaml:"allow_missing"`
			MaxRetainedAge string                              `yaml:"max_retained_age"`
			DisabledAction DisabledAction                      `yaml:"disabled_action"`
		} `yaml:"scopes"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data), yaml.Strict())
	if err := decoder.Decode(&wire); err != nil {
		return Profile{}, admissionError("profile", "contains invalid YAML or unknown fields")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Profile{}, admissionError("profile", "requires exactly one policy document")
	}
	if len(wire.Scopes) > maxScopes {
		return Profile{}, admissionError("profile.scopes", "exceeds the source scope bound")
	}
	profile := Profile{Version: wire.Version}
	for _, scope := range wire.Scopes {
		if scope.Required == nil || scope.Enabled == nil || scope.AllowMissing == nil {
			return Profile{}, admissionError("profile.scope", "requires explicit required, enabled, and allow_missing settings")
		}
		age, err := time.ParseDuration(scope.MaxRetainedAge)
		if err != nil {
			return Profile{}, admissionError("profile.max_retained_age", "requires an explicit duration")
		}
		profile.Scopes = append(profile.Scopes, ScopePolicy{Scope: Scope{Source: scope.Source, Binding: scope.Binding}, Required: *scope.Required, Enabled: *scope.Enabled, AllowMissing: *scope.AllowMissing, MaxRetainedAge: age, DisabledAction: scope.DisabledAction})
	}
	if _, err := validateProfile(profile); err != nil {
		return Profile{}, err
	}
	return profile, nil
}
