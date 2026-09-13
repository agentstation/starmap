package runtime

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type closeTestSource struct {
	closes  atomic.Int64
	failure error
}

func (*closeTestSource) Identity() string { return "owned-source-test" }
func (*closeTestSource) Read(context.Context) (SourceRead, error) {
	return SourceRead{Health: HealthOK}, nil
}
func (s *closeTestSource) Shutdown(context.Context) error {
	s.closes.Add(1)
	return s.failure
}

func TestRuntimeSourceOwnership(t *testing.T) {
	for _, test := range []struct {
		name    string
		owned   bool
		invalid bool
		want    int64
	}{
		{name: "owned", owned: true, want: 1},
		{name: "borrowed"},
		{name: "owned-invalid-options", owned: true, invalid: true, want: 1},
		{name: "borrowed-invalid-options", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := &closeTestSource{}
			selection := WithSource(source)
			if test.owned {
				selection = WithOwnedSource(source)
			}
			opts := []Option{WithCatalogSource("embedded"), WithSourcePollInterval(0), WithAcquisitionEnabled(false),
				WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())), selection}
			if test.invalid {
				opts = append(opts, WithCatalogNetworkMode("invalid"))
			}
			connected, err := Open(t.Context(), opts...)
			if test.invalid {
				if err == nil || connected != nil {
					t.Fatal("invalid options opened a runtime")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := connected.Close(); err != nil {
					t.Fatal(err)
				}
				if err := connected.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if got := source.closes.Load(); got != test.want {
				t.Fatalf("source closes=%d, want %d", got, test.want)
			}
		})
	}
}

func TestRuntimeClosesOnlyFinalOwnedSource(t *testing.T) {
	for _, borrowed := range []bool{false, true} {
		first, last := &closeTestSource{}, &closeTestSource{}
		selection := WithOwnedSource(last)
		if borrowed {
			selection = WithSource(last)
		}
		connected := openTestRuntime(t, WithOwnedSource(first), selection)
		if err := connected.Close(); err != nil {
			t.Fatal(err)
		}
		want := int64(1)
		if borrowed {
			want = 0
		}
		if first.closes.Load() != 0 || last.closes.Load() != want {
			t.Fatal("runtime closed an unselected or borrowed source")
		}
	}
}

func TestRuntimeReturnsOwnedSourceCloseFailure(t *testing.T) {
	failure := stderrors.New("source close failure")
	source := &closeTestSource{failure: failure}
	connected, err := Open(t.Context(), WithCatalogSource("embedded"), WithOwnedSource(source), WithSourcePollInterval(0))
	if err != nil {
		t.Fatal(err)
	}
	if err := connected.Close(); !stderrors.Is(err, failure) {
		t.Fatalf("close error=%v, want source failure", err)
	}
}

func TestFailedRuntimeOpenReturnsOwnedSourceCloseFailure(t *testing.T) {
	for _, partial := range []bool{false, true} {
		failure := stderrors.New("source close failure")
		source := &closeTestSource{failure: failure}
		directory := privateRuntimeDirectory(t)
		opts := []Option{WithStateDirectory(directory), WithCatalogSource("embedded"), WithOwnedSource(source),
			WithSourcePollInterval(0), WithAcquisitionEnabled(false)}
		if partial {
			if err := os.WriteFile(filepath.Join(directory, layerDirectoryName), []byte("preserve"), 0o600); err != nil {
				t.Fatal(err)
			}
		} else {
			opts = append(opts, WithCatalogNetworkMode("invalid"))
		}
		connected, err := Open(t.Context(), opts...)
		if connected != nil || !stderrors.Is(err, failure) || source.closes.Load() != 1 {
			t.Fatalf("failed Open lost the source close error: runtime=%p error=%v closes=%d", connected, err, source.closes.Load())
		}
		lock, err := acquireDirectory(t.Context(), directory)
		if err != nil {
			t.Fatal(err)
		}
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

type waitingShutdownSource struct {
	closeTestSource
	entered chan struct{}
	release chan struct{}
	start   sync.Once
}

func (s *waitingShutdownSource) Shutdown(ctx context.Context) error {
	s.start.Do(func() { close(s.entered) })
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestFailedOpenKeepsDirectoryUntilOwnedSourceStops(t *testing.T) {
	directory := privateRuntimeDirectory(t)
	if err := os.WriteFile(filepath.Join(directory, migrationPendingName), []byte("pending"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := &waitingShutdownSource{entered: make(chan struct{}), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(source.release) })
	defer release()
	finished := make(chan error, 1)
	go func() {
		connected, err := Open(t.Context(), WithStateDirectory(directory), WithOwnedSource(source), WithCatalogSource("embedded"),
			WithAcquisitionEnabled(false), WithSourcePollInterval(0))
		if connected != nil {
			_ = connected.Close()
		}
		finished <- err
	}()
	select {
	case <-source.entered:
	case <-time.After(time.Second):
		t.Fatal("failed Open did not start source shutdown")
	}
	select {
	case err := <-finished:
		var conflict *pkgerrors.ConflictError
		if !stderrors.Is(err, pkgerrors.ErrTimeout) || !stderrors.As(err, &conflict) {
			t.Fatalf("Open error=%v, want migration refusal and shutdown timeout", err)
		}
	case <-time.After(closeJoinTimeout + time.Second):
		t.Fatal("failed Open exceeded the shutdown bound")
	}
	lock, err := acquireDirectory(t.Context(), directory)
	if lock != nil {
		_ = lock.Close()
	}
	var conflict *pkgerrors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("failed Open released its directory before source shutdown: %v", err)
	}
	release()
	requireDirectoryRelease(t, directory)
}
