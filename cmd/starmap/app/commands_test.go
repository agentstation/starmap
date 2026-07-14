package app

import (
	"bytes"
	"slices"
	"testing"
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

func TestPrelaunchCommandAndOutputAliasesAreAbsent(t *testing.T) {
	application, err := New("0.1.1", "abc123", "2026-07-12", "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	root := application.createRootCommand()
	for _, flag := range []string{"fmt", "format"} {
		if root.PersistentFlags().Lookup(flag) != nil {
			t.Fatalf("prelaunch output alias --%s remains registered", flag)
		}
	}
	for _, command := range root.Commands() {
		for _, alias := range []string{"inspect", "server"} {
			if slices.Contains(command.Aliases, alias) {
				t.Fatalf("prelaunch command alias %q remains registered by %q", alias, command.Name())
			}
		}
	}
}
