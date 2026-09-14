package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/privatefiles"
)

type discoveryRecoveryContext struct {
	context.Context
	directory string
	cancel    context.CancelFunc
}

func (c discoveryRecoveryContext) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	if c.cancel == nil {
		entries, _ := os.ReadDir(c.directory)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".state-") {
				if info, err := entry.Info(); err == nil && info.Size() > 0 {
					os.Exit(88)
				}
			}
		}
		return nil
	}
	probe := flock.New(filepath.Join(c.directory, ".record-publications", ".owner.lock"))
	held, err := probe.TryLock()
	_ = probe.Close()
	if err != nil {
		return err
	}
	if !held {
		c.cancel()
	}
	return c.Context.Err()
}

func TestRuntimeCancelsGitHubConstructionRecovery(t *testing.T) {
	if path := os.Getenv("STARMAP_TEST_DISCOVERY_CONSTRUCTION_EXIT"); path != "" {
		openTestRuntime(t, WithStateDirectory(path), WithCatalogSource("embedded"), WithSourceRefreshMode("manual"))
		directory, err := privatefiles.NewDirectory(filepath.Join(path, "github-catalog-source"))
		if err != nil {
			t.Fatal(err)
		}
		ctx := discoveryRecoveryContext{Context: t.Context(), directory: filepath.Join(path, "github-catalog-source")}
		err = directory.PublishFileContext(ctx, "candidate.json", []byte("candidate"), ".state-")
		t.Fatalf("writer did not exit: %v", err)
	}
	path := privateRuntimeDirectory(t)
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRuntimeCancelsGitHubConstructionRecovery$")
	command.Env = append(os.Environ(), "STARMAP_TEST_DISCOVERY_CONSTRUCTION_EXIT="+path)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
		t.Fatalf("child exit: %v %s", err, output)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	controlled := discoveryRecoveryContext{Context: ctx, directory: filepath.Join(path, "github-catalog-source"), cancel: cancel}
	connected, err := Open(controlled, WithStateDirectory(path), WithCatalogSource("public"), WithSourceRefreshMode("manual"))
	if connected != nil {
		_ = connected.Close()
	}
	if !errors.Is(err, context.Canceled) || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("runtime dropped construction context: %v, context=%v", err, ctx.Err())
	}
	reopened := openTestRuntime(t, WithStateDirectory(path), WithCatalogSource("public"), WithSourceRefreshMode("manual"))
	if !reopened.Status().Usable {
		t.Fatal("retry lost the embedded baseline")
	}
}
