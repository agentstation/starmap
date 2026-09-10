package runtime

import (
	"context"
	"io/fs"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/server"
)

type permissionRelay interface {
	ReadPermission(context.Context) (catalogs.CatalogPermissionEnvelope, error)
}

func TestAuthorityRuntimeRelaysConfirmedRequirementWithoutActivation(t *testing.T) {
	source, options := authorityRuntimeFixture(t)
	timer := newStubScheduleTimer()
	r := openTestRuntime(t, append(options, withScheduleTimer(timer.after))...)
	if timer.waited(t, 5*time.Second) != permissionPollInterval {
		t.Fatal("permission scheduler did not complete its initial read")
	}
	reader, ok := any(r).(permissionRelay)
	if !ok {
		t.Fatal("runtime cannot relay its confirmed permission receipt")
	}
	if err := r.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	if r.AllowsNewAttempt() {
		t.Fatal("receipt alone authorized the baseline")
	}
	before := source.receipt
	var upstreamReads atomic.Int64
	source.permissionMu.Lock()
	source.permissionHook = func(context.Context) { upstreamReads.Add(1) }
	source.permissionMu.Unlock()
	got, err := reader.ReadPermission(t.Context())
	if err != nil || got != before {
		t.Fatalf("relay changed the unactivated receipt: got=%+v error=%v", got, err)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		if got, err := reader.ReadPermission(t.Context()); err != nil || got != before {
			t.Fatal("cached receipt changed")
		}
	}); allocs != 0 {
		t.Fatalf("relay allocated %v times", allocs)
	}
	if upstreamReads.Load() != 0 {
		t.Fatal("relay read upstream permission")
	}
	source.permissionMu.Lock()
	next := before
	next.Head.Sequence++
	next.Head.GenerationID = "withdrawal-two"
	next.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	next.Head.PermissionSchemaVersion++
	source.receipt = next
	source.permissionMu.Unlock()
	if err := r.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, err = reader.ReadPermission(t.Context())
	if err != nil || got != next {
		t.Fatalf("incompatible withdrawal was hidden: got=%+v error=%v", got, err)
	}
	if r.AllowsNewAttempt() {
		t.Fatal("incompatible receipt authorized inference")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err = reader.ReadPermission(t.Context()); err == nil || got.Version != 0 {
		t.Fatal("closed runtime relayed a receipt")
	}
}

func TestAuthorityRuntimeRelayServesWithdrawalOverVerifiedTLS(t *testing.T) {
	source, options := authorityRuntimeFixture(t)
	source.permissionErr = fs.ErrNotExist
	timer := newStubScheduleTimer()
	r := openTestRuntime(t, append(options, withScheduleTimer(timer.after))...)
	if timer.waited(t, 5*time.Second) != permissionPollInterval {
		t.Fatal("permission scheduler did not complete its initial read")
	}
	srv, err := server.New(r.Client(), server.DefaultConfig(), server.WithRuntime(r))
	if err != nil {
		t.Fatal(err)
	}
	endpoint := httptest.NewTLSServer(srv.Handler())
	t.Cleanup(endpoint.Close)
	client, err := remote.NewClient(endpoint.URL+"/api/v1", endpoint.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FetchPermissionEnvelope(t.Context()); err == nil {
		t.Fatal("cold relay served permission before a confirmed receipt")
	}
	source.permissionMu.Lock()
	source.permissionErr = nil
	source.permissionMu.Unlock()
	if err := r.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, err := client.FetchPermissionEnvelope(t.Context())
	if err != nil || got != source.receipt {
		t.Fatalf("TLS relay changed the upstream receipt: got=%+v error=%v", got, err)
	}
	source.permissionMu.Lock()
	next := source.receipt
	next.Head.Sequence++
	next.Head.GenerationID = "incompatible-withdrawal"
	next.Head.PermissionSchemaVersion++
	next.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	source.receipt = next
	source.permissionMu.Unlock()
	if err := r.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, err = client.FetchPermissionEnvelope(t.Context())
	if err != nil || got != next || r.AllowsNewAttempt() {
		t.Fatalf("TLS relay failed to propagate incompatible withdrawal: got=%+v error=%v", got, err)
	}
	if got.ValidUntil != source.receipt.ValidUntil {
		t.Fatal("relay renewed upstream permission")
	}
}

func TestAuthorityRuntimeRelayRejectsCanceledContext(t *testing.T) {
	p := retainedAuthorityPermissions(t)
	r := &Runtime{ctx: t.Context(), permissions: p}
	reader, ok := any(r).(permissionRelay)
	if !ok {
		t.Fatal("runtime has no permission relay")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if receipt, err := reader.ReadPermission(ctx); err != context.Canceled || receipt.Version != 0 {
		t.Fatalf("canceled request returned receipt=%+v error=%v", receipt, err)
	}
}

func TestAuthorityRuntimeRelayRefusesUnconfirmedOrExpiredState(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*authorityPermissions, *options)
	}{
		{name: "unconfirmed renewal", change: func(p *authorityPermissions, _ *options) { p.retained, p.pending = false, true }},
		{name: "rejected receipt", change: func(p *authorityPermissions, _ *options) { p.retained, p.pending = false, false }},
		{name: "newer manifest", change: func(p *authorityPermissions, _ *options) { p.highest.Sequence++ }},
		{name: "expired", change: func(p *authorityPermissions, o *options) {
			o.now = func() time.Time { return p.activeReceipt.ValidUntil }
		}},
		{name: "unknown clock", change: func(_ *authorityPermissions, o *options) { o.permissionClockUncertainty = nil }},
		{name: "excessive uncertainty", change: func(_ *authorityPermissions, o *options) {
			o.permissionClockUncertainty = func() (time.Duration, bool) { return time.Minute, true }
		}},
		{name: "ordinary runtime", change: func(_ *authorityPermissions, o *options) { o.source.StartupPolicy = StartupRequireSource }},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := retainedAuthorityPermissions(t)
			o := options{source: SourcePolicy{StartupPolicy: StartupRequireAuthority},
				now:                        func() time.Time { return p.activeReceipt.IssuedAt.Add(time.Second) },
				permissionClockUncertainty: func() (time.Duration, bool) { return 0, true },
			}
			test.change(&p, &o)
			r := &Runtime{ctx: t.Context(), config: o, permissions: p}
			reader, ok := any(r).(permissionRelay)
			if !ok {
				t.Fatal("runtime has no permission relay")
			}
			if receipt, err := reader.ReadPermission(t.Context()); err == nil || receipt.Version != 0 {
				t.Fatalf("unsafe relay: receipt=%+v error=%v", receipt, err)
			}
		})
	}
}
