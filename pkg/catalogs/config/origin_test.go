package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/config"
)

func TestAuthorityOriginSettingsAcceptCompleteDeclarations(t *testing.T) {
	for name, value := range map[string]string{
		"enabled":  `{"enabled":true,"authority_id":"company","policy_id":"production","permission_lifetime":"1m"}`,
		"disabled": `{"enabled":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			parsed, err := config.Parse(map[string]string{"STARMAP_CATALOG_AUTHORITY_ORIGIN": value})
			if err != nil {
				t.Fatal(err)
			}
			if got, present := parsed.Value("STARMAP_CATALOG_AUTHORITY_ORIGIN"); !present || got != value {
				t.Fatal("origin declaration lost its value or presence")
			}
		})
	}
}

func TestAuthorityOriginSettingsRejectIncompleteOrAmbiguousDeclarations(t *testing.T) {
	for _, value := range []string{
		"", "null", "[]", "{}", `{"enabled":true}`, `{"enabled":false,"authority_id":"company"}`,
		`{"enabled":null}`, `{"enabled":"true"}`, `{"enabled":false,"enabled":true}`,
		`{"enabled":false,"unknown":false}`, `{"enabled":false} {"enabled":true}`,
		`{"enabled":true,"authority_id":"company","policy_id":"production","permission_lifetime":"0s"}`,
		`{"enabled":true,"authority_id":"company","policy_id":"production","permission_lifetime":"6m"}`,
		`{"enabled":true,"authority_id":"company","policy_id":"production","bootstrap":null}`,
		`{"enabled":true,"authority_id":"company","policy_id":"production","permission_lifetime":60}`,
		strings.Repeat(" ", 4097) + `{"enabled":false,"unknown":"` + strings.Repeat("x", 4097) + `"}`,
	} {
		if _, err := config.Parse(map[string]string{config.AuthorityOrigin: value}); err == nil {
			t.Errorf("accepted invalid origin declaration: %s", value[:min(len(value), 80)])
		}
	}
}

func TestAuthorityOriginSettingsReplaceWholeDeclaration(t *testing.T) {
	lower := config.Layer{Name: "file", Values: map[string]string{
		config.AuthorityOrigin: `{"enabled":true,"authority_id":"company","policy_id":"production","bootstrap":true,"permission_lifetime":"1m"}`,
		config.Source:          "embedded",
	}}
	resolved, err := config.Resolve(lower)
	if err != nil {
		t.Fatal(err)
	}
	want := config.OriginSettings{Enabled: true, AuthorityID: "company", PolicyID: "production", Bootstrap: true, PermissionLifetime: time.Minute}
	if resolved.Config.AuthorityOrigin != want {
		t.Fatalf("origin = %+v", resolved.Config.AuthorityOrigin)
	}
	for _, value := range []string{`{"enabled":false}`, `{"enabled":true,"authority_id":"other","policy_id":"new"}`} {
		resolved, err := config.Resolve(config.Layer{Name: "environment", Values: map[string]string{config.AuthorityOrigin: value}}, lower)
		if err != nil {
			t.Fatal(err)
		}
		origin := resolved.Config.AuthorityOrigin
		if origin.Bootstrap || origin.PermissionLifetime != 0 || origin.AuthorityID == "company" || origin.PolicyID == "production" {
			t.Fatal("inherited an origin field from a replaced declaration")
		}
		if resolved.Config.SourceKind != "embedded" {
			t.Fatal("origin selection replaced independent upstream source")
		}
		if len(resolved.Ignored) != 1 || resolved.Ignored[0].Name != config.AuthorityOrigin {
			t.Fatal("missing ignored declaration diagnostic")
		}
	}
	if _, err := config.Resolve(config.Layer{Name: "environment", Values: map[string]string{config.AuthorityOrigin: `{"enabled":true,"authority_id":"other"}`}}, lower); err == nil {
		t.Fatal("incomplete override inherited lower policy")
	}
}
