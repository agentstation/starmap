package github

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type exitAfterDiscoveryStage struct {
	context.Context
	directory string
}

func (c exitAfterDiscoveryStage) Err() error {
	entries, _ := os.ReadDir(c.directory)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".state-") {
			if info, err := entry.Info(); err == nil && info.Size() > 0 {
				os.Exit(88)
			}
		}
	}
	return c.Context.Err()
}

func TestGitHubDiscoveryRecoversRecordPublicationAfterProcessExit(t *testing.T) {
	if directory := os.Getenv("STARMAP_TEST_DISCOVERY_RECORD_EXIT"); directory != "" {
		store, err := newStateStore(t.Context(), Config{StateDirectory: directory, Repository: testRepository, Channel: "catalog"})
		if err != nil {
			t.Fatal(err)
		}
		state, previous, err := store.loadSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		state.Sequence++
		ctx := exitAfterDiscoveryStage{Context: t.Context(), directory: filepath.Dir(store.path)}
		err = store.saveSnapshot(ctx, state, previous)
		t.Fatalf("writer did not exit: %v", err)
	}
	directory := t.TempDir()
	config := Config{StateDirectory: directory, Repository: testRepository, Channel: "catalog"}
	store, err := newStateStore(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	accepted := State{Repository: testRepository, Channel: "catalog", Sequence: 7, ChannelETag: "accepted-etag", Verified: ReleaseRef{Tag: "accepted-tag", GenerationID: "accepted-generation"}}
	if err := store.saveSnapshot(t.Context(), accepted, nil); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestGitHubDiscoveryRecoversRecordPublicationAfterProcessExit$")
	command.Env = append(os.Environ(), "STARMAP_TEST_DISCOVERY_RECORD_EXIT="+directory)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
		t.Fatalf("child exit: %v %s", err, output)
	}
	reopened, err := newStateStore(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(store.path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("recovery changed accepted state: %q %v", after, err)
	}
	entries, err := os.ReadDir(filepath.Dir(store.path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".state-") {
			t.Fatalf("abandoned discovery record: %s", entry.Name())
		}
	}
	state, previous, err := reopened.loadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	state.Sequence++
	if err := reopened.saveSnapshot(t.Context(), state, previous); err != nil {
		t.Fatal(err)
	}
	saved, err := reopened.load()
	if err != nil || saved.Sequence != 8 || saved.ChannelETag != accepted.ChannelETag || saved.Verified != accepted.Verified {
		t.Fatalf("retry lost accepted discovery metadata: %+v %v", saved, err)
	}
}
