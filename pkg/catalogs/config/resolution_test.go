package config_test

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/config"
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
