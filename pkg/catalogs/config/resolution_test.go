package config_test

import (
	stderrors "errors"
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestSourceReplacementDoesNotBorrowTransportCredentials(t *testing.T) {
	resolved, err := config.Resolve(
		config.Layer{Name: "flags", Values: map[string]string{config.Source: "starmap", config.SourceURL: "https://internal.example"}},
		config.Layer{Name: "file", Values: map[string]string{config.Source: "github", config.SourceRepository: "private/catalog", config.SourceToken: "private-token", config.SourceAPIKey: "old-server-key", config.SourcePollInterval: "30m"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Config.SourceAPIKey != "" {
		t.Fatal("replaced source retained its old API key")
	}
	if _, present := resolved.Config.Value(config.SourceToken); present {
		t.Fatal("replaced source retained its token")
	}
	if _, present := resolved.Config.Value(config.SourceRepository); present {
		t.Fatal("replaced source retained its repository")
	}
	if value, present := resolved.Config.Value(config.SourcePollInterval); !present || value != "30m" {
		t.Fatal("source replacement dropped an independent refresh policy")
	}
	if resolved.Origins[config.SourceURL] != "flags" || len(resolved.Ignored) == 0 {
		t.Fatal("source replacement lost origin diagnostics")
	}
}

func TestEmptyCredentialOverridesLowerLayer(t *testing.T) {
	resolved, err := config.Resolve(
		config.Layer{Name: "environment", Values: map[string]string{config.SourceAPIKey: ""}},
		config.Layer{Name: "file", Values: map[string]string{config.Source: "starmap", config.SourceURL: "https://internal.example", config.SourceAPIKey: "file-key"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if value, present := resolved.Config.Value(config.SourceAPIKey); !present || value != "" {
		t.Fatal("empty credential did not disable lower fallback")
	}
	if resolved.Origins[config.SourceAPIKey] != "environment" {
		t.Fatal("empty credential lost its winning origin")
	}
}

func TestFileKeysUseTheCanonicalParser(t *testing.T) {
	values, err := config.CanonicalValues(map[string]string{"catalog_acquisition_enabled": "false", "catalog.source.api.key": ""})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := config.Parse(values)
	if err != nil {
		t.Fatal(err)
	}
	if value, present := parsed.Value(config.AcquisitionEnabled); !present || value != "false" {
		t.Fatal("file value lost its boolean presence")
	}
	if _, err := config.CanonicalValues(map[string]string{"catalog_acquisition_enabld": "false"}); err == nil {
		t.Fatal("unknown file key was accepted")
	}
	if _, err := config.CanonicalValues(map[string]string{"catalog_acquisition_enabled": "false", config.AcquisitionEnabled: "true"}); err == nil {
		t.Fatal("conflicting aliases were accepted")
	}
}

func TestSourceReplacementRequiresACompleteApplicableGroup(t *testing.T) {
	for _, values := range []map[string]string{
		{config.SourceURL: "https://internal.example"},
		{config.Source: "starmap"},
		{config.Source: "embedded", config.SourceToken: "unused-secret"},
	} {
		if _, err := config.Resolve(config.Layer{Name: "file", Values: values}); err == nil {
			t.Fatal("incomplete or mismatched source group was accepted")
		}
	}
}

func TestHigherAuthorityCanReplaceInvalidLowerValue(t *testing.T) {
	resolved, err := config.Resolve(
		config.Layer{Name: "flags", Values: map[string]string{config.SourcePollInterval: "0s"}},
		config.Layer{Name: "environment", Values: map[string]string{config.SourcePollInterval: "invalid-duration"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if value, present := resolved.Config.Value(config.SourcePollInterval); !present || value != "0s" {
		t.Fatal("higher-priority value did not replace invalid lower value")
	}
}

func TestResolveAuthorityIgnoresLocalDeploymentValues(t *testing.T) {
	environmentState, fileState := t.TempDir(), t.TempDir()
	resolved, err := config.ResolveAuthority(
		config.Layer{Name: "shared", Values: map[string]string{config.Source: "starmap", config.SourceURL: "https://shared.example", config.SourcePollInterval: "30m"}},
		config.Layer{Name: "environment", Values: map[string]string{config.SourcePollInterval: "5m", config.StateDirectory: environmentState}},
		config.Layer{Name: "file", Values: map[string]string{config.Source: "github", config.SourceToken: "file-token", config.StateDirectory: fileState, config.SourceAliases: "node-a"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Authority != "shared" {
		t.Fatalf("authority = %q, want shared", resolved.Authority)
	}
	for name, want := range map[string]string{config.Source: "starmap", config.SourceURL: "https://shared.example", config.SourcePollInterval: "30m"} {
		if value, present := resolved.Config.Value(name); !present || value != want || resolved.Origins[name] != "shared" {
			t.Fatalf("%s = %q from %q, want %q from shared", name, value, resolved.Origins[name], want)
		}
	}
	if _, present := resolved.Config.Value(config.SourceToken); present {
		t.Fatal("local source credential entered the shared source group")
	}
	if resolved.Config.StateDirectory != environmentState || resolved.Origins[config.StateDirectory] != "environment" {
		t.Fatal("node-scope value did not resolve from the higher local layer")
	}
	if value, _ := resolved.Config.Value(config.SourceAliases); value != "node-a" || resolved.Origins[config.SourceAliases] != "file" {
		t.Fatal("node-scope value did not resolve from the local file")
	}
	for _, want := range []config.Ignored{
		{Name: config.SourcePollInterval, Origin: "environment", Reason: config.IgnoredSharedAuthority},
		{Name: config.Source, Origin: "file", Reason: config.IgnoredSharedAuthority},
		{Name: config.SourceToken, Origin: "file", Reason: config.IgnoredSharedAuthority},
		{Name: config.StateDirectory, Origin: "file", Reason: "higher-precedence-value"},
	} {
		if !slices.Contains(resolved.Ignored, want) {
			t.Fatalf("ignored = %+v, missing %+v", resolved.Ignored, want)
		}
	}
	if len(resolved.Ignored) != 4 {
		t.Fatalf("ignored = %+v, want 4 entries", resolved.Ignored)
	}
	plain, err := config.Resolve(config.Layer{Name: "file", Values: map[string]string{config.SourcePollInterval: "5m"}})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Authority != "" {
		t.Fatal("plain resolution reported a shared authority")
	}
}

func TestResolveAuthorityRejectsNodeScopeValues(t *testing.T) {
	for _, name := range []string{config.StateDirectory, config.Prefix + "CATALOG_UNKNOWN", config.AuthorityOrigin} {
		t.Run(name, func(t *testing.T) {
			resolved, err := config.ResolveAuthority(
				config.Layer{Name: "shared", Values: map[string]string{config.SourcePollInterval: "30m", name: `{"enabled":false}`}},
				config.Layer{Name: "file", Values: map[string]string{config.SchedulerIdentity: "node-a"}},
			)
			var validation *errors.ValidationError
			if !stderrors.As(err, &validation) || validation.Field != name {
				t.Fatalf("error = %v, want a validation error that names %s", err, name)
			}
			if resolved.Authority != "" || resolved.Origins != nil || resolved.Ignored != nil {
				t.Fatal("rejected shared layer partially resolved")
			}
		})
	}
}
