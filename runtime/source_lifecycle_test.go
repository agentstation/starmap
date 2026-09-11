package runtime

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type closeTestSource struct {
	closes  atomic.Int64
	failure error
}

func (*closeTestSource) Identity() string { return "owned-source-test" }
func (*closeTestSource) Read(context.Context) (SourceRead, error) {
	return SourceRead{Health: HealthOK}, nil
}
func (s *closeTestSource) Close() error {
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
