package settings_test

import (
	"testing"

	"github.com/agentstation/starmap/internal/catalog/settings"
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestCompositionPublishesConfiguredAuthorityOrigin(t *testing.T) {
	parsed, err := config.Parse(map[string]string{
		config.AuthorityOrigin: `{"enabled":true,"authority_id":"company","policy_id":"production","permission_lifetime":"1m"}`,
		config.Source:          "embedded", config.AcquisitionEnabled: "false", config.SourcePollInterval: "0s",
		config.PermissionClockSource: "native", config.PermissionClockRefreshInterval: "10s",
		config.PermissionClockMaxAge: "1m", config.PermissionClockMaxDriftPPM: "500",
		config.PermissionClockCounterUncertainty: "1us", config.PermissionClockWindowsMaxSourceAge: "1h",
		config.PermissionClockWindowsMaxSourceDriftPPM: "500", config.PermissionClockWindowsSourceUncertainty: "1ms",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := storage.NewMemory()
	composition := settings.Composition{Config: parsed, OriginStore: store, Base: clockRuntimeBase(t)}
	if _, err := composition.Options(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Current(t.Context()); err == nil {
		t.Fatal("option construction published a catalog")
	}
	connected, err := composition.Open(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connected.Close(); err != nil {
			t.Error(err)
		}
	}()
	current, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	head := current.Manifest.AuthorityHead
	if head.AuthorityID != "company" || head.PolicyID != "production" || head.Sequence != 1 {
		t.Fatalf("configured authority = %+v", head)
	}
	if connected.State().GenerationID != current.Manifest.GenerationID {
		t.Fatal("configured origin serves another catalog store")
	}
	if !connected.PermissionClockStatus().Running {
		t.Fatal("origin did not own its clock lifecycle")
	}
}

func TestCompositionOriginRequiresStoreAndNativeClock(t *testing.T) {
	parsed, err := config.Parse(map[string]string{config.AuthorityOrigin: `{"enabled":true,"authority_id":"company","policy_id":"production"}`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (settings.Composition{Config: parsed, OriginStore: storage.NewMemory()}).Options(); err == nil {
		t.Fatal("origin accepted absent native clock")
	}
	disabled, err := config.Parse(map[string]string{config.AuthorityOrigin: `{"enabled":false}`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (settings.Composition{Config: disabled}).Options(); err != nil {
		t.Fatal(err)
	}
}
