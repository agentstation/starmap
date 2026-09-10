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
