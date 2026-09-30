package config_test

import (
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/runtime"
)

func TestPublicAuthoritySettingsReachRuntime(t *testing.T) {
	parsed, err := config.Parse(map[string]string{
		config.Source: "starmap", config.SourceURL: "https://authority.example",
		config.SourceStartupPolicy: "require_authority",
		config.SourceAuthorityID:   "enterprise", config.SourcePolicyID: "production",
		config.SourcePollInterval: "0s", config.AcquisitionEnabled: "false",
		config.StateDirectory: filepath.Join(t.TempDir(), "runtime"),
	})
	if err != nil {
		t.Fatal(err)
	}
	connected, err := runtime.Open(t.Context(), append(parsed.Options(), runtime.WithSource(aliasSource{}))...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connected.Close(); err != nil {
			t.Error(err)
		}
	}()
	status := connected.Status()
	if !status.AuthorityRequired || status.Usable || !status.CatalogAvailable {
		t.Fatalf("authority configuration lost at runtime: %+v", status)
	}
}

func TestCanonicalAuthorityOptionsResolveForPassiveInspection(t *testing.T) {
	parsed, err := config.Parse(map[string]string{
		config.Source: "starmap", config.SourceURL: "https://authority.example",
		config.SourceStartupPolicy: "require_authority", config.SourceAuthorityID: "enterprise", config.SourcePolicyID: "production",
		config.SourceAliases: "primary,secondary", config.SourcePollInterval: "0s", config.SourceMaxAge: "2h", config.SourceMaxHops: "3",
	})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := runtime.ResolveSourcePolicy(parsed.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Kind != runtime.SourceStarmap || policy.URL != "https://authority.example" || policy.StartupPolicy != runtime.StartupRequireAuthority ||
		policy.AuthorityID != "enterprise" || policy.PolicyID != "production" || policy.PollInterval != 0 || policy.MaxAge.String() != "2h0m0s" || policy.MaxHops != 3 || len(policy.Aliases) != 2 || policy.Aliases[0] != "primary" || policy.Aliases[1] != "secondary" {
		t.Fatal("canonical authority settings differ from passive inspection policy")
	}
}
