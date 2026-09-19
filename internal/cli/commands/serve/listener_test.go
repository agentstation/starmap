package serve

import (
	"bytes"
	"strings"
	"testing"
)

func TestExplicitListenerFlagsOverrideLegacyEnvironment(t *testing.T) {
	t.Setenv("HTTP_HOST", "127.0.0.1")
	t.Setenv("HTTP_PORT", "8123")
	t.Setenv("STARMAP_SERVER_HOST", "ignored.example")
	t.Setenv("STARMAP_SERVER_PORT", "invalid")
	command := NewCommand(nil)
	var diagnostics bytes.Buffer
	command.SetErr(&diagnostics)
	if err := command.Flags().Parse([]string{"--host", "localhost", "--port", "9876"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfig(command)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "localhost" || cfg.Port != 9876 {
		t.Fatalf("listener=%s:%d", cfg.Host, cfg.Port)
	}
	if !strings.Contains(diagnostics.String(), "HTTP_HOST is deprecated") || !strings.Contains(diagnostics.String(), "HTTP_PORT is deprecated") {
		t.Fatal("legacy variables have no migration diagnostic")
	}
}

func TestProductListenerEnvironmentAndConflicts(t *testing.T) {
	t.Setenv("STARMAP_SERVER_HOST", "127.0.0.1")
	t.Setenv("STARMAP_SERVER_PORT", "8123")
	t.Setenv("HTTP_HOST", "127.0.0.1")
	t.Setenv("HTTP_PORT", "8123")
	cfg, err := parseConfig(NewCommand(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 8123 {
		t.Fatal("product listener settings were ignored")
	}
	t.Setenv("HTTP_PORT", "9876")
	if _, err := parseConfig(NewCommand(nil)); err == nil {
		t.Fatal("conflicting listener ports were accepted")
	}
	t.Setenv("HTTP_PORT", "invalid")
	t.Setenv("STARMAP_SERVER_PORT", "invalid")
	if _, err := parseConfig(NewCommand(nil)); err == nil {
		t.Fatal("invalid listener port was ignored")
	}
}
