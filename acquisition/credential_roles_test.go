package acquisition_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestStarportCredentialRolePrecedence(t *testing.T) {
	names := []string{"STARPORT_CATALOG_AUDIT_API_KEY", "STARMAP_AUDIT_API_KEY", "STARPORT_AUDIT_API_KEY", "AUDIT_API_KEY"}
	for first := range names {
		t.Run(names[first], func(t *testing.T) {
			values := map[string]string{}
			for _, name := range names[first:] {
				values[name] = name
			}
			reads := 0
			resolver, err := acquisition.OpenCredentialResolver(t.Context(), acquisition.CredentialResolverConfig{
				Product: acquisition.CredentialProductStarport,
				Lookup:  func(name string) (string, bool) { reads++; v, ok := values[name]; return v, ok },
			})
			if err != nil {
				t.Fatal(err)
			}
			if reads != 0 {
				t.Fatal("constructor read credentials")
			}
			provider := publicCredentialProvider()
			material, err := resolver.ResolveCatalog(t.Context(), &provider)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := material.Value("api-key"); value != names[first] {
				t.Fatal("wrong role precedence")
			}
		})
	}
}

func TestStarportCredentialEmptyOrInvalidSelectionIsTerminal(t *testing.T) {
	for _, value := range []string{"", "   ", "invalid"} {
		t.Run(value, func(t *testing.T) {
			provider := publicCredentialProvider()
			provider.Credentials.Fields[0].Pattern = "^valid-"
			resolver, err := acquisition.OpenCredentialResolver(t.Context(), acquisition.CredentialResolverConfig{
				Product: acquisition.CredentialProductStarport,
				Lookup: func(name string) (string, bool) {
					switch name {
					case "STARPORT_CATALOG_AUDIT_API_KEY":
						return value, true
					case "AUDIT_API_KEY":
						t.Fatal("read lower-priority credential after invalid selection")
					}
					return "", false
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := resolver.ResolveCatalog(t.Context(), &provider); err == nil {
				t.Fatal("invalid selection permitted fallback")
			}
		})
	}
}

func TestStarportCredentialPolicyMigration(t *testing.T) {
	for _, test := range []struct {
		name                       string
		legacy, equal, secondField bool
		conflict                   bool
	}{
		{name: "fresh"},
		{name: "upgrade changed key", legacy: true, conflict: true},
		{name: "upgrade same key", legacy: true, equal: true},
		{name: "upgrade changed project", legacy: true, equal: true, secondField: true, conflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{"STARPORT_CATALOG_AUDIT_API_KEY": "new", "STARPORT_AUDIT_API_KEY": "old", "EXPLICIT_AUDIT_KEY": "selected"}
			if test.equal {
				values["STARPORT_AUDIT_API_KEY"] = "new"
			}
			provider := publicCredentialProvider()
			if test.secondField {
				provider.Credentials.Fields = append(provider.Credentials.Fields, catalogs.ProviderCredentialField{ID: "project", Required: true})
				provider.Credentials.Profiles[0].Fields = append(provider.Credentials.Profiles[0].Fields, "project")
				values["STARPORT_CATALOG_AUDIT_PROJECT"] = "new-project"
				values["STARPORT_AUDIT_PROJECT"] = "old-project"
			}
			config := acquisition.CredentialResolverConfig{Product: acquisition.CredentialProductStarport,
				Lookup: func(name string) (string, bool) { v, ok := values[name]; return v, ok },
				State:  &acquisition.CredentialPolicyState{Directory: filepath.Join(t.TempDir(), "policy"), Product: "starport", DeploymentID: "test", InstanceID: "one", LegacyInstallation: test.legacy},
			}
			resolver, err := acquisition.OpenCredentialResolver(t.Context(), config)
			if err != nil {
				t.Fatal(err)
			}
			_, err = resolver.ResolveCatalog(t.Context(), &provider)
			if test.conflict {
				if !errors.IsConflict(err) {
					t.Fatalf("migration did not refuse changed complete handle: %v", err)
				}
				config.References = []acquisition.CredentialReference{{ProviderID: "audit", FieldID: "api-key", Reference: "env:EXPLICIT_AUDIT_KEY"}}
				if test.secondField {
					config.References = append(config.References, acquisition.CredentialReference{ProviderID: "audit", FieldID: "project", Reference: "env:STARPORT_CATALOG_AUDIT_PROJECT"})
				}
				resolver, err = acquisition.OpenCredentialResolver(t.Context(), config)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = resolver.ResolveCatalog(t.Context(), &provider); err != nil {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			config.References = nil
			config.State.LegacyInstallation = true
			values["STARPORT_AUDIT_API_KEY"] = "changed-after-acceptance"
			resolver, err = acquisition.OpenCredentialResolver(t.Context(), config)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = resolver.ResolveCatalog(t.Context(), &provider); err != nil {
				t.Fatalf("accepted policy lost on restart: %v", err)
			}
			config.Product = acquisition.CredentialProductStarmap
			if _, err = acquisition.OpenCredentialResolver(t.Context(), config); err == nil {
				t.Fatal("wrong policy family reused persisted state")
			}
		})
	}
}

func TestStarportCredentialAliasCollisionPrecedesLookup(t *testing.T) {
	provider := publicCredentialProvider()
	provider.Credentials.Fields = append(provider.Credentials.Fields, catalogs.ProviderCredentialField{
		ID: "project", Environment: []string{"STARPORT_CATALOG_AUDIT_API_KEY"},
	})
	resolver, err := acquisition.OpenCredentialResolver(t.Context(), acquisition.CredentialResolverConfig{
		Product: acquisition.CredentialProductStarport,
		Lookup:  func(string) (string, bool) { t.Fatal("read credentials before alias validation"); return "", false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveCatalog(t.Context(), &provider); err == nil {
		t.Fatal("ambiguous environment alias accepted")
	}
}

func TestCredentialProductValidationPrecedesStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy")
	_, err := acquisition.OpenCredentialResolver(t.Context(), acquisition.CredentialResolverConfig{
		Product: "unknown",
		Lookup:  func(string) (string, bool) { t.Fatal("invalid product read credentials"); return "", false },
		State:   &acquisition.CredentialPolicyState{Directory: path, Product: "host", DeploymentID: "test", InstanceID: "one"},
	})
	if err == nil {
		t.Fatal("unsupported product accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid product created state")
	}
}
