package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/runtime"
)

// cascadeHops is the number of serving hops of the propagation test. Three
// hops separate the origin publication time from the last local check.
const cascadeHops = 3

// TestCascadedFreshnessPropagatesChannelUpdatedAtThroughHops proves that the
// origin publication time survives every hop of a cascade. Each hop serves the
// source-chain manifest, and the hop below it reads that manifest instead of
// its own check time. The last hop therefore grades a stale origin as stale,
// even though it checked its own upstream a moment ago.
func TestCascadedFreshnessPropagatesChannelUpdatedAtThroughHops(t *testing.T) {
	now := time.Now().UTC()
	origin := now.Add(-12 * time.Hour)

	status := runtime.Status{
		Usable:           true,
		InstanceIdentity: "hop-0",
		SourceIdentity:   "synthetic-channel",
		GenerationID:     "generation-0",
		SourceHealth:     runtime.HealthOK,
		UpstreamHealth:   runtime.HealthUnknown,
		ChannelUpdatedAt: origin,
		ObservedAt:       now,
	}

	var served []protocol.SourceChain
	for hop := range cascadeHops {
		document := fetchSourceChain(t, status)
		served = append(served, document)
		if hop == cascadeHops-1 {
			break
		}
		status = downstreamStatus(document, hop+1, now)
	}

	for hop, document := range served {
		if !document.ChannelUpdatedAt.Equal(origin) {
			t.Fatalf("hop %d channel time = %s, want the origin time %s",
				hop, document.ChannelUpdatedAt, origin)
		}
		if len(document.Hops) != hop {
			t.Fatalf("hop %d carried %d chain entries, want %d",
				hop, len(document.Hops), hop)
		}
		if document.ObservedAt.Before(now) {
			t.Fatalf("hop %d observed at %s, want its own recent check",
				hop, document.ObservedAt)
		}
	}

	// The last hop graded the propagated origin time, so readiness reports a
	// critical channel while the local check of that hop stays current.
	assertReadinessReportsChannel(t, status, origin, now)
}

// assertReadinessReportsChannel proves that readiness publishes the propagated
// origin time, its age, and its grade. An operator then sees a stalled origin
// at every hop below it.
func assertReadinessReportsChannel(
	t *testing.T,
	status runtime.Status,
	origin, now time.Time,
) {
	t.Helper()
	app := newMockApplication()
	app.runtimeStatus = &status
	endpoint := serveTestRouter(t, app)

	response, err := http.Get(endpoint + "/api/v1/ready") //nolint:noctx // test client
	if err != nil {
		t.Fatalf("GET readiness: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	var body struct {
		Data struct {
			Runtime struct {
				ChannelFreshness     string `json:"channel_freshness"`
				ChannelAgeSeconds    int64  `json:"channel_age_seconds"`
				SourceCheckFreshness string `json:"source_check_freshness"`
				InstanceIdentity     string `json:"instance_identity"`
				ChainHops            int    `json:"chain_hops"`
			} `json:"runtime"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness body: %v", err)
	}
	if body.Data.Runtime.ChannelFreshness != string(runtime.FreshnessCritical) {
		t.Fatalf("readiness channel freshness = %q, want critical",
			body.Data.Runtime.ChannelFreshness)
	}
	// The local check of this hop is current, so the critical grade above rests
	// on the propagated origin time alone.
	if body.Data.Runtime.SourceCheckFreshness != string(runtime.FreshnessCurrent) {
		t.Fatalf("readiness source check freshness = %q, want current",
			body.Data.Runtime.SourceCheckFreshness)
	}
	wantAge := int64(now.Sub(origin) / time.Second)
	if body.Data.Runtime.ChannelAgeSeconds != wantAge {
		t.Fatalf("readiness channel age = %d seconds, want %d",
			body.Data.Runtime.ChannelAgeSeconds, wantAge)
	}
	if body.Data.Runtime.InstanceIdentity != status.InstanceIdentity {
		t.Fatalf("readiness instance = %q, want %q",
			body.Data.Runtime.InstanceIdentity, status.InstanceIdentity)
	}
	if body.Data.Runtime.ChainHops != len(status.Chain) {
		t.Fatalf("readiness chain hops = %d, want %d",
			body.Data.Runtime.ChainHops, len(status.Chain))
	}
}

// fetchSourceChain serves one runtime status and reads the manifest back with
// the protocol client, so the test crosses the real route and media type.
func fetchSourceChain(t *testing.T, status runtime.Status) protocol.SourceChain {
	t.Helper()
	app := newMockApplication()
	app.runtimeStatus = &status
	endpoint := serveTestRouter(t, app)
	client, err := protocol.NewClient(
		endpoint+"/api/v1", http.DefaultClient, catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	document, err := client.FetchSourceChain(context.Background())
	if err != nil {
		t.Fatalf("FetchSourceChain: %v", err)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("served manifest: %v", err)
	}
	return document
}

// serveTestRouter starts one server for the test application.
func serveTestRouter(t *testing.T, app *mockApplication) string {
	t.Helper()
	server, err := New(app, Config{PathPrefix: "/api/v1", CacheTTL: time.Minute})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	endpoint := httptest.NewServer(server.setupRouter())
	t.Cleanup(endpoint.Close)
	return endpoint.URL
}

// downstreamStatus builds the status of the hop below one served manifest. It
// mirrors the runtime rule: the downstream keeps the propagated origin time and
// the sanitized chain, and it records its own observation time.
func downstreamStatus(
	document protocol.SourceChain,
	hop int,
	now time.Time,
) runtime.Status {
	// The hop grades the propagated origin time. Its own check happened now, so
	// a hop that graded the check time would report a current channel.
	channelAge := now.Sub(document.ChannelUpdatedAt)
	channelFreshness := runtime.FreshnessCurrent
	switch {
	case channelAge >= runtime.FreshnessChannelCriticalAge:
		channelFreshness = runtime.FreshnessCritical
	case channelAge >= runtime.FreshnessChannelWarnAge:
		channelFreshness = runtime.FreshnessWarn
	}
	return runtime.Status{
		Usable:               true,
		InstanceIdentity:     identityOfHop(hop),
		SourceIdentity:       "starmap_cascade",
		GenerationID:         document.GenerationID,
		SourceHealth:         runtime.HealthOK,
		UpstreamHealth:       runtime.HealthOK,
		ChannelUpdatedAt:     document.ChannelUpdatedAt,
		ChannelAge:           channelAge,
		ChannelFreshness:     channelFreshness,
		SourceCheckFreshness: runtime.FreshnessCurrent,
		ObservedAt:           now,
		Chain:                chainOfDocument(document),
	}
}

// chainOfDocument returns the chain the downstream records, nearest hop first.
func chainOfDocument(document protocol.SourceChain) []runtime.SourceHop {
	hops := []runtime.SourceHop{{
		Identity:    document.Identity,
		Health:      runtime.HealthOK,
		PublishedAt: document.ChannelUpdatedAt,
		ObservedAt:  document.ObservedAt,
	}}
	for _, hop := range document.Hops {
		hops = append(hops, runtime.SourceHop{
			Identity:    hop.Identity,
			Health:      runtime.HealthOK,
			PublishedAt: hop.PublishedAt,
			ObservedAt:  hop.ObservedAt,
		})
	}
	return hops
}

// identityOfHop names one hop of the test cascade.
func identityOfHop(hop int) string {
	return "hop-" + string(rune('0'+hop))
}
