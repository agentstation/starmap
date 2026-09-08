package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
)

func TestServiceConfigurationDoesNotLoadIncidentalDotenv(t *testing.T) {
	t.Chdir(t.TempDir())
	setTestHome(t, t.TempDir())
	t.Setenv("CSP_DOTENV_INCIDENTAL", "")
	if err := os.Unsetenv("CSP_DOTENV_INCIDENTAL"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte("CSP_DOTENV_INCIDENTAL=must-not-load\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err != nil {
		t.Fatal(err)
	}
	if _, present := os.LookupEnv("CSP_DOTENV_INCIDENTAL"); present {
		t.Fatal("service configuration loaded an incidental dotenv file")
	}
}

func TestExplicitDotenvPrecedenceAndConflictDiagnostics(t *testing.T) {
	directory := t.TempDir()
	base, local := filepath.Join(directory, ".env"), filepath.Join(directory, ".env.local")
	t.Setenv("CSP_DOTENV_VALUE", "")
	if err := os.Unsetenv("CSP_DOTENV_VALUE"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CSP_DOTENV_PROCESS", "process-value")
	t.Setenv("CSP_DOTENV_EMPTY", "")
	if err := os.WriteFile(base, []byte("CSP_DOTENV_VALUE=base-value\nCSP_DOTENV_PROCESS=file-value\nCSP_DOTENV_EMPTY=file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("CSP_DOTENV_VALUE=local-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := loadExplicitEnvFiles([]string{base, local})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("CSP_DOTENV_VALUE") != "local-value" || os.Getenv("CSP_DOTENV_PROCESS") != "process-value" || os.Getenv("CSP_DOTENV_EMPTY") != "" {
		t.Fatal("dotenv precedence changed")
	}
	if len(result.conflicts) != 1 || result.conflicts[0].Name != "CSP_DOTENV_VALUE" {
		t.Fatal("dotenv conflict lacks a diagnostic")
	}
}

func TestCommandFlagsOverrideInvalidEnvironmentSettings(t *testing.T) {
	setTestHome(t, t.TempDir())
	t.Setenv(catalogconfig.SourcePollInterval, "invalid-duration")
	application := NewForCommand("test", "test", "test", "test")
	if err := application.Execute(t.Context(), []string{"--catalog-source", "embedded", "--catalog-source-poll-interval", "0s", "version"}); err != nil {
		t.Fatal(err)
	}
	if value, present := application.CatalogSettings().Value(catalogconfig.SourcePollInterval); !present || value != "0s" {
		t.Fatal("flag did not override the invalid lower value")
	}
}

func TestCommandDotenvSourceReplacementPreservesAuthority(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(strconv.FormatBool(legacy), func(t *testing.T) {
			clearCatalogEnvironment(t)
			setTestHome(t, t.TempDir())
			directory := t.TempDir()
			base, local := filepath.Join(directory, ".env"), filepath.Join(directory, ".env.local")
			contents := "STARMAP_CATALOG_SOURCE=starmap\nSTARMAP_CATALOG_SOURCE_URL=https://old.example.test\nSTARMAP_CATALOG_SOURCE_API_KEY=old-transport-secret\n"
			if legacy {
				contents = "REMOTE_SERVER_URL=https://old.example.test\nREMOTE_SERVER_API_KEY=old-transport-secret\n"
			}
			if err := os.WriteFile(base, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(local, []byte("STARMAP_CATALOG_SOURCE=starmap\nSTARMAP_CATALOG_SOURCE_URL=https://new.example.test\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			application := NewForCommand("test", "test", "test", "test")
			if err := application.Execute(t.Context(), []string{"--env-file", base, "--env-file", local, "version"}); err != nil {
				t.Fatal(err)
			}
			parsed := application.CatalogSettings()
			if parsed.SourceURL != "https://new.example.test" || parsed.SourceAPIKey != "" {
				t.Fatal("the replacement source inherited a lower transport credential")
			}
			if application.config.CatalogOrigins[catalogconfig.SourceURL] != "dotenv:"+local {
				t.Fatal("the selected source lost its file origin")
			}
			if legacy && len(application.config.LegacyCatalogNames) != 2 {
				t.Fatal("legacy source settings lost their migration diagnostic")
			}
			// A later command must not promote old file credentials into process authority.
			if err := application.Execute(t.Context(), []string{"--env-file", base, "--env-file", local, "version"}); err != nil {
				t.Fatal(err)
			}
			if application.CatalogSettings().SourceAPIKey != "" {
				t.Fatal("a repeated command promoted a dotenv credential into process authority")
			}
			for _, name := range []string{catalogconfig.Source, catalogconfig.SourceAPIKey, "REMOTE_SERVER_API_KEY"} {
				if _, present := os.LookupEnv(name); present {
					t.Fatal("catalog dotenv values escaped their explicit file authority")
				}
			}
		})
	}
}

func TestExplicitDotenvFailureDoesNotMutateEnvironmentOrExposeValues(t *testing.T) {
	for name, contents := range map[string]string{"syntax": "BAD='private-sentinel", "nul": "BAD=private-sentinel\x00", "duplicate": ""} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CSP_DOTENV_ATOMIC", "")
			if err := os.Unsetenv("CSP_DOTENV_ATOMIC"); err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			base, bad := filepath.Join(directory, "base.env"), filepath.Join(directory, "bad.env")
			if err := os.WriteFile(base, []byte("CSP_DOTENV_ATOMIC=private-sentinel\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(bad, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if name == "duplicate" {
				bad = base
			}
			_, err := loadExplicitEnvFiles([]string{base, bad})
			if err == nil || strings.Contains(err.Error(), "private-sentinel") {
				t.Fatal("invalid dotenv input did not produce a redacted refusal")
			}
			if _, present := os.LookupEnv("CSP_DOTENV_ATOMIC"); present {
				t.Fatal("failed dotenv loading applied part of the input")
			}
		})
	}
}

func clearCatalogEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range append(catalogconfig.Names(), "REMOTE_SERVER_URL", "REMOTE_SERVER_API_KEY") {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}
