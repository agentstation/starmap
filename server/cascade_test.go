package server_test

import (
	"context"
	stderrors "errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/catalog/settings"
	"github.com/agentstation/starmap/internal/test/channel"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

// cascadeDeadline bounds every wait for a streamed delta. A subscriber that
// needs longer than this failed, because the whole cascade runs on loopback.
const cascadeDeadline = 10 * time.Second

// TestServerCascadesVerifiedCatalogSource proves one cascade end to end. The
// origin runtime pulls a synthetic verified channel and serves it. A middle
// runtime consumes the origin through the starmap source kind, and a leaf
// runtime consumes the middle. The origin channel time propagates through both
// hops, and each hop reports one more chain entry. A streamed delta reaches the
// middle with no new fetch of its own. A self reference, an alias of this
// instance, and a two-node cycle each refuse the read before the runtime keeps
// a generation.
func TestServerCascadesVerifiedCatalogSource(t *testing.T) {
	t.Setenv(channel.ConfiguredEnvironment, "test-key")
	ctx := context.Background()

	upstream, err := channel.Start()
	if err != nil {
		t.Fatalf("channel.Start: %v", err)
	}
	t.Cleanup(upstream.Close)

	origin := openRuntime(t, upstream, t.TempDir(), t.TempDir())
	t.Cleanup(func() { _ = origin.Close() })
	if _, err := origin.RefreshSource(ctx); err != nil {
		t.Fatalf("origin RefreshSource: %v", err)
	}
	originAPI := serveRuntime(t, origin)
	originStatus := origin.Status()
	if originStatus.ChannelUpdatedAt.IsZero() {
		t.Fatal("the origin reported no channel time")
	}

	// The middle hop consumes the origin through the composed starmap source.
	middle := openCascadeRuntime(t, cascade{url: originAPI, identity: "middle"})
	t.Cleanup(func() { _ = middle.Close() })
	report, err := middle.RefreshSource(ctx)
	if err != nil {
		t.Fatalf("middle RefreshSource: %v", err)
	}
	if !report.Changed {
		t.Fatal("the first cascaded read reported no change")
	}
	middleStatus := middle.Status()
	if middleStatus.PayloadChecksum != originStatus.PayloadChecksum {
		t.Fatalf("middle payload = %q, want the origin payload %q",
			middleStatus.PayloadChecksum, originStatus.PayloadChecksum)
	}
	if !middleStatus.ChannelUpdatedAt.Equal(originStatus.ChannelUpdatedAt) {
		t.Fatalf("middle channel time = %s, want the origin time %s",
			middleStatus.ChannelUpdatedAt, originStatus.ChannelUpdatedAt)
	}
	if len(middleStatus.Chain) != 1 {
		t.Fatalf("middle chain = %#v, want the origin as the one hop", middleStatus.Chain)
	}
	if middleStatus.Chain[0].Identity != originStatus.InstanceIdentity {
		t.Fatalf("middle first hop = %q, want the origin identity %q",
			middleStatus.Chain[0].Identity, originStatus.InstanceIdentity)
	}

	// The leaf hop consumes the middle. The origin channel time survives the
	// second hop, so freshness rests on the origin and not on the last check.
	middleAPI := serveRuntime(t, middle)
	leaf := openCascadeRuntime(t, cascade{url: middleAPI, identity: "leaf"})
	t.Cleanup(func() { _ = leaf.Close() })
	if _, err := leaf.RefreshSource(ctx); err != nil {
		t.Fatalf("leaf RefreshSource: %v", err)
	}
	leafStatus := leaf.Status()
	if !leafStatus.ChannelUpdatedAt.Equal(originStatus.ChannelUpdatedAt) {
		t.Fatalf("leaf channel time = %s, want the origin time %s",
			leafStatus.ChannelUpdatedAt, originStatus.ChannelUpdatedAt)
	}
	if len(leafStatus.Chain) != 2 {
		t.Fatalf("leaf chain = %#v, want the middle and the origin", leafStatus.Chain)
	}
	if leafStatus.Chain[0].Identity != middleStatus.InstanceIdentity ||
		leafStatus.Chain[1].Identity != originStatus.InstanceIdentity {
		t.Fatalf("leaf chain identities = %#v, want the middle then the origin",
			leafStatus.Chain)
	}

	assertCascadeRejections(t, ctx, originAPI, middleAPI, originStatus.InstanceIdentity)

	// Local acquisition at the origin publishes new bytes. The delta reaches
	// the middle on its subscriber stream, so the middle needs no fetch of its
	// own to observe it.
	if _, err := origin.Sync(ctx); err != nil {
		t.Fatalf("origin Sync: %v", err)
	}
	acquired := origin.Status()
	if acquired.PayloadChecksum == originStatus.PayloadChecksum {
		t.Fatal("local acquisition changed no served bytes")
	}
	assertStreamedDeltaReachesMiddle(t, middle, acquired.PayloadChecksum)
}

// assertCascadeRejections proves the three refusal rules of the source chain.
// Each runtime refuses the read before it keeps the generation behind it.
func assertCascadeRejections(
	t *testing.T,
	ctx context.Context,
	originAPI, middleAPI, originIdentity string,
) {
	t.Helper()
	for _, test := range []struct {
		name    string
		config  cascade
		wantHop string
	}{
		{
			name:    "self reference",
			config:  cascade{url: originAPI, identity: originIdentity},
			wantHop: originIdentity,
		},
		{
			name: "alias of this instance",
			config: cascade{
				url:      originAPI,
				identity: "aliased",
				aliases:  []string{"other-name", originIdentity},
			},
			wantHop: originIdentity,
		},
		{
			name:    "two node cycle",
			config:  cascade{url: middleAPI, identity: originIdentity},
			wantHop: originIdentity,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			connected := openCascadeRuntime(t, test.config)
			t.Cleanup(func() { _ = connected.Close() })
			_, err := connected.RefreshSource(ctx)
			var conflict *errors.ConflictError
			if !stderrors.As(err, &conflict) {
				t.Fatalf("RefreshSource error = %v, want a ConflictError", err)
			}
			if conflict.Actual != test.wantHop {
				t.Fatalf("refused hop = %q, want %q", conflict.Actual, test.wantHop)
			}
			status := connected.Status()
			if status.SourceHealth != runtime.HealthUnavailable {
				t.Fatalf("source health = %s, want unavailable", status.SourceHealth)
			}
			if status.SourceReason != "chain_rejected" {
				t.Fatalf("source reason = %q, want chain_rejected", status.SourceReason)
			}
		})
	}
}

// assertStreamedDeltaReachesMiddle proves the reactive path of the cascade.
// The middle hop polls once an hour, and the test calls no refresh. Only the
// subscriber stream can move the middle onto the new bytes. The wake reaches
// the source worker, and that worker refreshes on its own.
func assertStreamedDeltaReachesMiddle(
	t *testing.T,
	middle *runtime.Runtime,
	wantChecksum string,
) {
	t.Helper()
	deadline := time.Now().Add(cascadeDeadline)
	for time.Now().Before(deadline) {
		if middle.Status().PayloadChecksum == wantChecksum {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("middle payload = %q, want the streamed delta %q",
		middle.Status().PayloadChecksum, wantChecksum)
}

// cascade names one downstream runtime of the test cascade.
type cascade struct {
	url      string
	identity string
	aliases  []string
}

// serveRuntime serves one runtime and returns its versioned API root.
func serveRuntime(t *testing.T, connected *runtime.Runtime) string {
	t.Helper()
	srv := newServer(t, connected)
	endpoint := httptest.NewServer(srv.Handler())
	t.Cleanup(endpoint.Close)
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownBound)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	})
	return endpoint.URL + "/api/v1"
}

// openCascadeRuntime composes one downstream runtime with the starmap source
// kind. The canonical settings own the composition, so the test exercises the
// same path a deployment uses.
func openCascadeRuntime(t *testing.T, node cascade) *runtime.Runtime {
	t.Helper()
	values := map[string]string{
		settings.Source:              string(runtime.SourceStarmap),
		settings.SourceURL:           node.url,
		settings.SourcePollInterval:  "1h",
		settings.SourceMaxHops:       "8",
		settings.AcquisitionEnabled:  "false",
		settings.StateDirectory:      t.TempDir(),
		settings.CoalesceWindow:      "10ms",
		settings.StartupSpread:       "10ms",
		settings.TransferMaxDuration: "5s",
		settings.SchedulerIdentity:   node.identity,
	}
	if len(node.aliases) > 0 {
		values[settings.SourceAliases] = strings.Join(node.aliases, ",")
	}
	config, err := settings.Load(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	})
	if err != nil {
		t.Fatalf("settings.Load: %v", err)
	}
	store, err := storage.NewFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("storage.NewFilesystem: %v", err)
	}
	acquirer, err := acquisition.NewAcquirer(
		acquisition.WithAcquirerCredentialResolver(auth.NewResolver()),
		acquisition.WithAcquirerCoalesceWindow(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("acquisition.NewAcquirer: %v", err)
	}
	connected, err := settings.Composition{
		Config:   config,
		Acquirer: acquirer,
		Base: []runtime.Option{
			runtime.WithClientOptions(
				starmap.WithCatalogStore(store),
				starmap.WithCatalogPath(filepath.Join(t.TempDir(), "workspace")),
			),
		},
	}.Open(context.Background())
	if err != nil {
		t.Fatalf("Composition.Open: %v", err)
	}
	return connected
}
