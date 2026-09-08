package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
)

func TestCanonicalCatalogYAMLRetainsExplicitValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "catalog_source: embedded\ncatalog_acquisition_enabled: false\ncatalog_acquisition_interval: 0s\ncatalog_source_api_key: ''\ncatalog_source_aliases: []\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := loadCatalogSettings(loaded)
	if err != nil {
		t.Fatal(err)
	}
	for name, expected := range map[string]string{catalogconfig.Source: "embedded", catalogconfig.AcquisitionEnabled: "false", catalogconfig.AcquisitionInterval: "0s", catalogconfig.SourceAPIKey: "", catalogconfig.SourceAliases: ""} {
		actual, present := parsed.Value(name)
		if !present || actual != expected {
			t.Fatalf("YAML lost the value or presence of %s", name)
		}
	}
}

func TestUnknownCatalogYAMLKeyFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("catalog_acquisition_enabld: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(path); err == nil {
		t.Fatal("unknown YAML key was accepted")
	}
}

func TestEveryCanonicalSettingLoadsFromYAML(t *testing.T) {
	for _, name := range catalogconfig.Names() {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	for _, descriptor := range catalogconfig.Descriptors() {
		t.Run(descriptor.Key, func(t *testing.T) {
			expected := descriptor.Default
			if expected == "" && !descriptor.AllowEmpty {
				switch descriptor.Name {
				case catalogconfig.SourceURL:
					expected = "https://catalog.example.test"
				case catalogconfig.SourceSignerWorkflow:
					expected = ".github/workflows/catalog-generation.yaml"
				default:
					expected = t.TempDir()
				}
			}
			var value any = expected
			switch descriptor.Type {
			case catalogconfig.BooleanValue:
				value = false
				expected = "false"
			case catalogconfig.IntegerValue:
				parsed, err := strconv.Atoi(expected)
				if err != nil {
					t.Fatal(err)
				}
				value = parsed
			case catalogconfig.ListValue:
				value = []string{}
				expected = ""
			}
			// JSON is a YAML subset and keeps typed scalars exact in this fixture.
			fileValues := map[string]any{descriptor.Key: value}
			if descriptor.Name == catalogconfig.SourceURL {
				fileValues["catalog_source"] = "starmap"
			}
			contents, err := json.Marshal(fileValues)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			loaded, err := loadConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := loadCatalogSettings(loaded)
			if err != nil {
				t.Fatal(err)
			}
			actual, present := parsed.Value(descriptor.Name)
			if !present || actual != expected {
				t.Fatalf("YAML did not apply %s", descriptor.Name)
			}
		})
	}
}

func TestApplicationSourceReplacementClearsFileCredentials(t *testing.T) {
	t.Setenv(catalogconfig.Source, "starmap")
	t.Setenv(catalogconfig.SourceURL, "https://new.example.test")
	loaded := &Config{CatalogValues: map[string]string{catalogconfig.Source: "starmap", catalogconfig.SourceURL: "https://old.example.test", catalogconfig.SourceAPIKey: "old-key"}}
	parsed, err := loadCatalogSettings(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SourceAPIKey != "" || parsed.SourceURL != "https://new.example.test" {
		t.Fatal("application source replacement borrowed file credentials")
	}
	if loaded.CatalogOrigins[catalogconfig.SourceURL] != "environment" || len(loaded.CatalogIgnored) == 0 {
		t.Fatal("application source resolution lost origin diagnostics")
	}
}

func TestUnknownCatalogEnvironmentNameFails(t *testing.T) {
	t.Setenv("STARMAP_CATALOG_ACQUISITION_ENABLD", "false")
	if _, err := loadCatalogSettings(&Config{}); err == nil {
		t.Fatal("unknown catalog environment name was ignored")
	}
}
