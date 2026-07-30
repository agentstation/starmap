package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/logging"
)

func TestVersionCommandWritesNormalOutputToStdout(t *testing.T) {
	application, err := New("0.1.1", "abc123", "2026-07-12", "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	command := application.NewVersionCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, want := stdout.String(), "starmap 0.1.1\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestManCommandWritesRootManualToStdout(t *testing.T) {
	application, err := New("0.1.1", "abc123", "2026-07-12", "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	root := application.createRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"man"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		`.TH "STARMAP" "1"`,
		".SH NAME",
		"starmap - AI Model Catalog CLI",
		".SH SYNOPSIS",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("manual missing %q:\n%s", want, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestCommandExecutionInstallsConfiguredDefaultLogger(t *testing.T) {
	original := logging.Default()
	t.Cleanup(func() {
		logging.SetDefault(original)
	})

	application, err := New("0.1.1", "abc123", "2026-07-12", "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	root := application.createRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--quiet", "version"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	if got := logging.Default().GetLevel(); got != zerolog.WarnLevel {
		t.Fatalf("default logger level = %v, want %v", got, zerolog.WarnLevel)
	}
}

func TestCommandConstructionPreservesLoadedConfiguration(t *testing.T) {
	application, err := New("0.1.1", "abc123", "2026-07-12", "test", WithConfig(&Config{
		Verbose: true,
		Output:  "yaml",
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	root := application.createRootCommand()
	if !application.Config().Verbose || application.Config().Output != "yaml" {
		t.Fatalf("command construction changed config: %#v", application.Config())
	}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"version"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	if !application.Config().Verbose || application.Config().Output != "yaml" {
		t.Fatalf("flag defaults replaced config: %#v", application.Config())
	}
}

func TestExplicitConfigFileLoadsAfterFlagParsing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	configPath := filepath.Join(t.TempDir(), "selected.yaml")
	if err := os.WriteFile(configPath, []byte(
		"catalog_path: /from-selected-file\noutput: yaml\n",
	), constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	application, err := New("0.1.1", "abc123", "2026-07-12", "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	root := application.createRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", configPath, "--output", "json", "version"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}

	config := application.Config()
	if config.ConfigFile != configPath {
		t.Fatalf("ConfigFile = %q, want %q", config.ConfigFile, configPath)
	}
	if config.CatalogPath != "/from-selected-file" {
		t.Fatalf("CatalogPath = %q, want /from-selected-file", config.CatalogPath)
	}
	if config.Output != "json" {
		t.Fatalf("Output = %q, want explicit flag json", config.Output)
	}
}

func TestExplicitConfigFileMustExistAndParse(t *testing.T) {
	malformed := filepath.Join(t.TempDir(), "malformed.yaml")
	if err := os.WriteFile(malformed, []byte("catalog_path: [\n"), constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{name: "missing", path: filepath.Join(t.TempDir(), "missing.yaml")},
		{name: "malformed", path: malformed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			application, err := New("0.1.1", "abc123", "2026-07-12", "test")
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			root := application.createRootCommand()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{"--config", test.path, "version"})
			if err := root.ExecuteContext(context.Background()); err == nil {
				t.Fatal("ExecuteContext succeeded with unusable explicit config file")
			}
		})
	}
}
