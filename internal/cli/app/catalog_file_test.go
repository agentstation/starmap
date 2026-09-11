package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spf13/pflag"

	"github.com/agentstation/starmap/internal/catalog/settings"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/sources"
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
				case catalogconfig.SourceAuthorityID:
					expected = "authority-from-yaml"
				case catalogconfig.SourcePolicyID:
					expected = "policy-from-yaml"
				case catalogconfig.ProviderBindings:
					expected = "[]"
				default:
					switch descriptor.Type {
					case catalogconfig.DurationValue:
						expected = "1s"
					case catalogconfig.IntegerValue:
						expected = "500"
					default:
						expected = t.TempDir()
					}
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
			case catalogconfig.AuthorityOriginValue:
				value = map[string]any{"enabled": false}
				expected = `{"enabled":false}`
			case catalogconfig.ProviderBindingsValue:
				value = []any{}
				expected = "[]"
			}
			// JSON is a YAML subset and keeps typed scalars exact in this fixture.
			fileValues := map[string]any{descriptor.Key: value}
			switch descriptor.Name {
			case catalogconfig.SourceURL:
				fileValues["catalog_source"] = "starmap"
			case catalogconfig.SourceAuthorityID, catalogconfig.SourcePolicyID:
				fileValues["catalog_source"] = "starmap"
				fileValues["catalog_source_url"] = "https://catalog.example.test"
				fileValues["catalog_source_startup_policy"] = "require_authority"
				fileValues["catalog_source_authority_id"] = "authority-from-yaml"
				fileValues["catalog_source_policy_id"] = "policy-from-yaml"
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

func TestProviderBindingsLoadFromYAMLAndFlagsReplaceEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := `catalog_provider_bindings:
  - schema_version: 1
    id: team-catalog
    revision: '1'
    provider_id: openai
    account_id: team-account
    region: global
    api_surface: models.list
    credential_role: catalog_acquisition
    credential_profile_id: api-key
`
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
	value, present := parsed.Value(catalogconfig.ProviderBindings)
	var bindings []sources.ProviderAcquisitionBinding
	if !present || json.Unmarshal([]byte(value), &bindings) != nil || len(bindings) != 1 || bindings[0].AccountID != "team-account" {
		t.Fatal("YAML lost the declared binding scope")
	}
	if loaded.CatalogOrigins[catalogconfig.ProviderBindings] != "configuration-file" {
		t.Fatal("binding diagnostics lost their file origin")
	}

	t.Setenv(catalogconfig.ProviderBindings, "[]")
	parsed, err = loadCatalogSettings(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if selected, _ := parsed.Value(catalogconfig.ProviderBindings); selected != "[]" {
		t.Fatal("an empty environment array inherited file bindings")
	}
	flags := pflag.NewFlagSet("bindings", pflag.ContinueOnError)
	if err := settings.RegisterFlags(flags); err != nil {
		t.Fatal(err)
	}
	if err := flags.Parse([]string{"--catalog-provider-bindings", value}); err != nil {
		t.Fatal(err)
	}
	parsed, err = loadCatalogSettings(loaded, settings.FlagLookup(flags))
	if err != nil {
		t.Fatal(err)
	}
	if selected, _ := parsed.Value(catalogconfig.ProviderBindings); selected != value {
		t.Fatal("an explicit flag failed to replace environment bindings")
	}
	if loaded.CatalogOrigins[catalogconfig.ProviderBindings] != "override-1" {
		t.Fatal("binding diagnostics lost their flag origin")
	}
}

func TestAuthorityOriginYAMLAndEnvironmentUseWholeDeclaration(t *testing.T) {
	clearCatalogEnvironment(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "catalog_authority_origin:\n  enabled: true\n  authority_id: company\n  policy_id: production\n  bootstrap: true\n  permission_lifetime: 1m\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
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
	if !parsed.AuthorityOrigin.Enabled || !parsed.AuthorityOrigin.Bootstrap || parsed.AuthorityOrigin.AuthorityID != "company" {
		t.Fatal("YAML origin not applied")
	}
	t.Setenv(catalogconfig.AuthorityOrigin, `{"enabled":false}`)
	parsed, err = loadCatalogSettings(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.AuthorityOrigin != (catalogconfig.OriginSettings{}) {
		t.Fatal("environment disable inherited YAML origin fields")
	}
}
