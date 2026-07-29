package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"

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
