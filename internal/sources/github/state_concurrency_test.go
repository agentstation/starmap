package github

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/attestation"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestGitHubSourceInstancesPreserveTheReplayFloor(t *testing.T) {
	server := newFixtureServer(t)
	older := publishCatalog(t, server, "older-concurrent-generation", testChannelSequence)
	directory := t.TempDir()
	entered, release := make(chan struct{}), make(chan struct{})
	var closeOnce sync.Once
	unblock := func() { closeOnce.Do(func() { close(release) }) }
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	var workers sync.WaitGroup
	defer func() { unblock(); cancel(); workers.Wait() }()
	recorder := &recordingAttester{}
	verify := recorder.attest()
	slow := newTestSource(t, server, WithStateDirectory(directory), WithAttester(func(ctx context.Context, bundle []byte, digest string, policy attestation.Policy) (attestation.Result, error) {
		result, err := verify(ctx, bundle, digest, policy)
		if digest == hexDigest(older.Assets[0].Body) {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return attestation.Result{}, ctx.Err()
			}
		}
		return result, err
	}))
	result := make(chan error, 1)
	workers.Go(func() { _, err := slow.ReadChannel(ctx); result <- err })
	select {
	case <-entered:
	case err := <-result:
		t.Fatalf("older refresh returned before the pause: %v", err)
	case <-ctx.Done():
		t.Fatal("older verification did not pause")
	}
	newer := publishCatalog(t, server, "newer-concurrent-generation", testChannelSequence+1)
	fast := newTestSource(t, server, WithStateDirectory(directory), WithAttester((&recordingAttester{}).attest()))
	if _, err := fast.ReadChannel(t.Context()); err != nil {
		t.Fatal(err)
	}
	unblock()
	var conflict *errors.ConflictError
	if err := <-result; !stderrors.As(err, &conflict) {
		t.Errorf("stale source overwrote the accepted replay floor: %v", err)
	}
	state, err := fast.state.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Sequence != newer.Channel.Sequence || state.Verified.GenerationID != newer.Generation.Manifest.GenerationID {
		t.Fatalf("accepted replay floor changed: sequence=%d, generation=%s", state.Sequence, state.Verified.GenerationID)
	}
	retried, err := slow.ReadChannel(t.Context())
	if err != nil || retried.Sequence != newer.Channel.Sequence || retried.GenerationID != newer.Generation.Manifest.GenerationID {
		t.Fatalf("conflicting source cannot retry the current channel: %+v %v", retried, err)
	}
}
