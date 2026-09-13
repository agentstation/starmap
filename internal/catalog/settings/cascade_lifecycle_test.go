package settings_test

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/settings"
	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/runtime"
)

func TestCompositionClosesBuiltCascade(t *testing.T) {
	testCompositionCascadeShutdown(t, false)
}

func TestFailedCompositionOpenClosesBuiltCascade(t *testing.T) {
	testCompositionCascadeShutdown(t, true)
}

func testCompositionCascadeShutdown(t *testing.T, cancelStartup bool) {
	t.Helper()
	generation, err := starmap.EmbeddedGeneration()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := protocol.MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	streamStarted := make(chan struct{})
	streamStopped := make(chan struct{})
	chainStarted := make(chan struct{})
	release := make(chan struct{})
	var started, stopped, chain sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1" + protocol.ManifestPath:
			w.Header().Set("Content-Type", protocol.ManifestMediaType)
			_, _ = w.Write(manifest)
		case "/api/v1" + protocol.PayloadPath(generation.Manifest.GenerationID):
			w.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
			_, _ = w.Write(generation.Payload)
		case "/api/v1" + protocol.EventStreamPath:
			w.Header().Set("Content-Type", protocol.EventStreamMediaType)
			_, _ = fmt.Fprint(w, ": connected\n\n")
			w.(http.Flusher).Flush()
			started.Do(func() { close(streamStarted) })
			select {
			case <-r.Context().Done():
				stopped.Do(func() { close(streamStopped) })
			case <-release:
			}
		case "/api/v1" + protocol.SourceChainPath:
			if cancelStartup {
				chain.Do(func() { close(chainStarted) })
				select {
				case <-r.Context().Done():
				case <-release:
				}
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	values := map[string]string{
		settings.Source: "starmap", settings.SourceURL: server.URL + "/api/v1",
		settings.SourcePollInterval: "0s", settings.AcquisitionEnabled: "false",
		settings.StateDirectory: filepath.Join(t.TempDir(), "runtime"),
	}
	if cancelStartup {
		values[settings.SourceStartupPolicy] = "require_source"
	}
	config, err := settings.Load(func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	})
	if err != nil {
		t.Fatal(err)
	}
	composition := settings.Composition{Config: config,
		Base: []runtime.Option{runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()))},
	}
	if cancelStartup {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		finished := make(chan error, 1)
		go func() {
			connected, err := composition.Open(ctx)
			if connected != nil {
				_ = connected.Close()
			}
			finished <- err
		}()
		select {
		case <-chainStarted:
		case err := <-finished:
			t.Fatalf("Open returned before the source-chain request: %v", err)
		case <-time.After(time.Minute):
			t.Fatal("startup did not request its source chain")
		}
		cancel()
		select {
		case err := <-finished:
			if !stderrors.Is(err, context.Canceled) {
				t.Fatalf("Open error=%v, want cancellation", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Open did not stop after cancellation")
		}
	} else {
		connected, err := composition.Open(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = connected.Close() })
		if _, err := connected.RefreshSource(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := connected.Close(); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-streamStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("cascade did not open its event stream")
	}
	select {
	case <-streamStopped:
	case <-time.After(time.Second):
		t.Fatal("runtime shutdown left its constructed cascade stream active")
	}
	composition.Extra = []runtime.Option{runtime.WithSourceRefreshMode("manual"),
		runtime.WithCatalogNetworkMode("offline"), runtime.WithSourceStartupPolicy("prefer_source")}
	replacement, err := composition.Open(t.Context())
	if err != nil {
		t.Fatalf("replacement could not reopen the runtime directory: %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
}
