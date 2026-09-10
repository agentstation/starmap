package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/remote"
)

type observedAuthoritySource struct {
	*authorityTestSource
	observer func(context.Context, catalogs.CatalogAuthorityHead) error
}

func (s *observedAuthoritySource) BindAuthorityObserver(observer func(context.Context, catalogs.CatalogAuthorityHead) error) error {
	s.observer = observer
	return nil
}

func TestAuthorityRuntimeRefusesManifestCallbacksAfterClose(t *testing.T) {
	initial, options := authorityRuntimeFixture(t)
	source := &observedAuthoritySource{authorityTestSource: initial}
	timer := newStubScheduleTimer()
	r := openTestRuntime(t, append(options, WithSource(source), withScheduleTimer(timer.after))...)
	if timer.waited(t, 5*time.Second) != permissionPollInterval {
		t.Fatal("permission schedule did not finish")
	}
	if source.observer == nil {
		t.Fatal("runtime did not bind the source observer")
	}
	before := r.Status().RequiredPermissionRevision
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	next := initial.receipt.Head
	next.Sequence++
	next.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	if err := source.observer(t.Context(), next); err != context.Canceled {
		t.Fatalf("closed observer returned %v", err)
	}
	if r.Status().RequiredPermissionRevision != before {
		t.Fatal("late callback changed sealed permission state")
	}
}

func TestAuthorityRuntimeLearnsWithdrawalBeforePayloadArrival(t *testing.T) {
	initial, options := authorityRuntimeFixture(t)
	next := initial.replies[0].Generation.Copy()
	next.Manifest.GenerationID = "withdrawal-awaiting-payload"
	next.Manifest.AuthorityHead.GenerationID = next.Manifest.GenerationID
	next.Manifest.AuthorityHead.Sequence++
	next.Manifest.AuthorityHead.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	manifest, err := protocol.MarshalManifest(next.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	permission, err := json.Marshal(initial.receipt)
	if err != nil {
		t.Fatal(err)
	}
	payloadEntered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1" + protocol.PermissionEnvelopePath:
			w.Header().Set("Content-Type", protocol.PermissionEnvelopeMediaType)
			_, _ = w.Write(permission)
		case "/api/v1" + protocol.ManifestPath:
			w.Header().Set("Content-Type", protocol.ManifestMediaType)
			_, _ = w.Write(manifest)
		case "/api/v1" + protocol.PayloadPath(next.Manifest.GenerationID):
			enteredOnce.Do(func() { close(payloadEntered) })
			select {
			case <-release:
			case <-request.Context().Done():
			}
			http.Error(w, "payload unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, request)
		}
	}))
	defer endpoint.Close()
	defer unblock()
	options = append(options, WithSourceURL(endpoint.URL+"/api/v1"), WithStateDirectory(privateRuntimeDirectory(t)))
	seed := openTestRuntime(t, options...)
	if _, err := seed.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !seed.AllowsNewAttempt() {
		t.Fatal("seed catalog has no valid permission")
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	source, err := remote.NewSource(t.Context(), remote.SourceConfig{Identity: initial.Identity(), Subscriber: remote.Config{
		BaseURL: endpoint.URL + "/api/v1", HTTPClient: endpoint.Client(), ShutdownTimeout: time.Second,
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	r := openTestRuntime(t, append(options, WithSource(source))...)
	defer r.Close()
	// Release the transfer before shutdown joins the source worker.
	defer unblock()
	finished := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { defer close(finished); _, _ = r.RefreshSource(ctx) }()
	select {
	case <-payloadEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("verified manifest did not reach payload transfer")
	}
	if r.AllowsNewAttempt() {
		t.Error("trusted manifest withdrawal left old permission active during payload transfer")
	}
	if got := r.Status().RequiredPermissionRevision; got != next.Manifest.AuthorityHead.RequiredPermissionRevision {
		t.Errorf("required revision=%q, want the trusted manifest revision", got)
	}
	unblock()
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("source refresh did not finish after payload refusal")
	}
}
