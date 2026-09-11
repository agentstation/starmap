package config_test

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/config"
)

func TestPublicDescriptorsCoverCanonicalSettings(t *testing.T) {
	descriptors := config.Descriptors()
	if len(descriptors) != 38 {
		t.Fatalf("descriptor count = %d, want 38 canonical settings", len(descriptors))
	}
	seen := make(map[string]bool)
	for _, descriptor := range descriptors {
		if descriptor.ID == "" || descriptor.Key == "" || descriptor.Type == "" || descriptor.Scope == "" || descriptor.Mutability == "" || descriptor.Anchor == "" || descriptor.Introduced != 1 {
			t.Fatalf("descriptor lacks required metadata: %s", descriptor.Name)
		}
		if seen[descriptor.ID] || !slices.Contains(config.Names(), descriptor.Name) {
			t.Fatalf("duplicate or unsupported descriptor: %s", descriptor.ID)
		}
		seen[descriptor.ID] = true
	}
	// Descriptors returned to one consumer cannot change another consumer's schema.
	descriptors[0].Environment[0] = "CHANGED"
	if config.Descriptors()[0].Environment[0] == "CHANGED" {
		t.Fatal("descriptor metadata shares mutable storage")
	}
}

func TestPublicSettingsPreserveExplicitValues(t *testing.T) {
	values := map[string]string{
		config.AcquisitionEnabled:  "false",
		config.AcquisitionInterval: "0s",
		config.SourceAPIKey:        "",
		config.SourceAliases:       "",
	}
	parsed, err := config.Parse(values)
	if err != nil {
		t.Fatal(err)
	}
	for name, expected := range values {
		actual, present := parsed.Value(name)
		if !present || actual != expected || !slices.Contains(parsed.Configured(), name) {
			t.Fatalf("presence or value changed for %s", name)
		}
	}
	if len(parsed.Options()) != len(values) {
		t.Fatal("an explicit value lost its runtime option")
	}
	if _, present := parsed.Value(config.SourceToken); present {
		t.Fatal("absent credential became present")
	}
}

func TestPublicSettingsRejectUnknownAndInvalidValues(t *testing.T) {
	for name, value := range map[string]string{
		"STARMAP_CATALOG_UNKNWON":  "value",
		config.SourceURL:           "",
		config.StateDirectory:      "",
		config.AcquisitionEnabled:  "",
		config.SourceMaxHops:       "0",
		config.SourcePollInterval:  "-1s",
		config.TransferIdleTimeout: "0s",
		config.SourceStartupPolicy: "embedded",
	} {
		if _, err := config.Parse(map[string]string{name: value}); err == nil {
			t.Errorf("accepted invalid setting %s", name)
		}
	}
}

func TestPublicSettingsParseIsPassive(t *testing.T) {
	t.Setenv(config.Source, "starmap")
	t.Setenv(config.SourceURL, "http://127.0.0.1:1")
	parsed, err := config.Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Configured()) != 0 || parsed.SourceKind != "public" {
		t.Fatal("public parser read ambient configuration")
	}
}

func TestProviderBindingsDescriptorPreservesOmissionAndRestart(t *testing.T) {
	for _, descriptor := range config.Descriptors() {
		if descriptor.Name != config.ProviderBindings {
			continue
		}
		if descriptor.Type != config.ProviderBindingsValue || descriptor.Mutability != "restart" || descriptor.Scope != config.DeploymentScope || descriptor.SourceBinding != "" {
			t.Fatal("binding schema has the wrong grammar, change class, or authority")
		}
		if descriptor.AllowEmpty || descriptor.Default != "" || descriptor.DefaultMeaning == "" {
			t.Fatal("binding schema collapsed omission into an empty binding set")
		}
		return
	}
	t.Fatal("binding descriptor is absent")
}
