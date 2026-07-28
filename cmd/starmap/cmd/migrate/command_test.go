package migrate

import (
	"bytes"
	"context"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/catalog/workspace"
)

type migrationStub struct {
	result workspace.LegacyLayoutMigrationResult
	err    error
	called int
}

func (s *migrationStub) MigrateCatalogWorkspace(context.Context) (workspace.LegacyLayoutMigrationResult, error) {
	s.called++
	return s.result, s.err
}

func TestCatalogCommandRunsExplicitMigration(t *testing.T) {
	t.Parallel()

	stub := &migrationStub{result: workspace.LegacyLayoutMigrationResult{
		WorkspacePath: "/home/test/.starmap/catalog",
		StatePath:     "/home/test/.starmap/state/catalog",
		GenerationID:  "generation-1",
		RetainedCount: 3,
	}}
	command := NewCommand(stub)
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetArgs([]string{"catalog"})

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	if stub.called != 1 {
		t.Fatalf("migration calls = %d, want 1", stub.called)
	}
	for _, want := range []string{
		"generation-1",
		"3 retained",
		"/home/test/.starmap/state/catalog",
		"/home/test/.starmap/catalog",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output %q does not contain %q", output.String(), want)
		}
	}
}

func TestCatalogCommandReturnsMigrationError(t *testing.T) {
	t.Parallel()

	fault := stderrors.New("migration failed")
	stub := &migrationStub{err: fault}
	command := NewCommand(stub)
	command.SetArgs([]string{"catalog"})

	if err := command.ExecuteContext(context.Background()); !stderrors.Is(err, fault) {
		t.Fatalf("ExecuteContext error = %v, want fault", err)
	}
	if stub.called != 1 {
		t.Fatalf("migration calls = %d, want 1", stub.called)
	}
}
