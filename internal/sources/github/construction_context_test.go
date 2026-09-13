package github

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

type cancelWhenDiscoveryWriterHeld struct {
	context.Context
	lock   string
	cancel context.CancelFunc
}

func (c cancelWhenDiscoveryWriterHeld) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	probe := flock.New(c.lock)
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

func TestGitHubConstructionHonorsCancellation(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state")
		if _, err := NewContext(nil, WithStateDirectory(path)); err == nil {
			t.Fatal("nil context reached source construction")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("nil context created state: %v", err)
		}
	})
	t.Run("before state creation", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := NewContext(ctx, WithStateDirectory(path)); !errors.Is(err, context.Canceled) {
			t.Fatalf("construction ignored cancellation: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("canceled construction created state: %v", err)
		}
	})
	t.Run("during recovery", func(t *testing.T) {
		path := t.TempDir()
		opts := []Option{WithStateDirectory(path), WithRepository(testRepository), WithChannel("catalog")}
		source, err := New(opts...)
		if err != nil {
			t.Fatal(err)
		}
		accepted := State{Repository: testRepository, Channel: "catalog", Sequence: 7, ChannelETag: "accepted-etag", Verified: ReleaseRef{Tag: "accepted-tag", GenerationID: "accepted-generation"}}
		if err := source.state.saveSnapshot(t.Context(), accepted, nil); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestGitHubDiscoveryRecoversRecordPublicationAfterProcessExit$")
		command.Env = append(os.Environ(), "STARMAP_TEST_DISCOVERY_RECORD_EXIT="+path)
		output, err := command.CombinedOutput()
		if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
			t.Fatalf("child exit: %v %s", err, output)
		}
		before := map[string][]byte{}
		if err := filepath.WalkDir(path, func(file string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			data, err := os.ReadFile(file)
			before[file] = data
			return err
		}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		controlled := cancelWhenDiscoveryWriterHeld{Context: ctx, cancel: cancel, lock: filepath.Join(path, stateDirectoryName, ".record-publications", ".owner.lock")}
		if _, err := NewContext(controlled, opts...); !errors.Is(err, context.Canceled) || !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("recovery ignored caller cancellation: %v, context=%v", err, ctx.Err())
		}
		for path, data := range before {
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatalf("canceled recovery changed %s: %v", path, err)
			}
		}
		reopened, err := NewContext(t.Context(), opts...)
		if err != nil {
			t.Fatal(err)
		}
		state, err := reopened.state.load()
		if err != nil || state.Sequence != accepted.Sequence || state.Verified != accepted.Verified || state.ChannelETag != accepted.ChannelETag {
			t.Fatalf("retry changed accepted discovery state: %+v %v", state, err)
		}
	})
}
