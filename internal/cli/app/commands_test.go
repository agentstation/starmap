package app

import (
	"bytes"
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
