package catalogdistribution

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestHostedPromotionDevCanaryStableProbeRollbackAndTelemetry(t *testing.T) {
	policy := PromotionPolicy{
		MaxGenerationAge: 24 * time.Hour,
		MaxProbeAge:      5 * time.Minute,
		// The positive path uses a real httptest round trip, whose wall time is
		// scheduler-dependent under the race detector. The exact SLO boundary is
		// exercised below with synthetic probe evidence.
		MaxProbeLatency: 30 * time.Second,
	}
	repository, err := NewMemoryRepositoryWithPolicy(policy)
	if err != nil {
		t.Fatalf("NewMemoryRepositoryWithPolicy: %v", err)
	}
	first := hostedFixture(t)
	secondGeneration := first.Generation.Copy()
	secondGeneration.Manifest.GenerationID = "embedded-bootstrap-promoted-second"
	secondGeneration.Manifest.GeneratedAt = first.Generation.Manifest.GeneratedAt.Add(time.Hour)
	secondArtifact, err := catalogartifact.Build(secondGeneration)
	if err != nil {
		t.Fatalf("Build second: %v", err)
	}
	second := PublishedGeneration{Generation: secondGeneration, Artifact: secondArtifact}
	for _, published := range []PublishedGeneration{first, second} {
		if err := repository.Publish(published); err != nil {
			t.Fatalf("Publish %s: %v", published.Generation.Manifest.GenerationID, err)
		}
	}
	now := second.Generation.Manifest.GeneratedAt.Add(time.Hour)
	repository.now = func() time.Time { return now }

	firstID := first.Generation.Manifest.GenerationID
	if err := repository.Promote(ChannelStable, firstID, nil); err == nil {
		t.Fatal("stable promotion bypassed dev/canary ordering")
	}
	if err := repository.Promote(ChannelCanary, firstID, nil); err == nil {
		t.Fatal("canary promotion bypassed dev")
	}
	if err := repository.Promote(ChannelDev, firstID, nil); err != nil {
		t.Fatalf("Promote first dev: %v", err)
	}
	if err := repository.Promote(ChannelCanary, firstID, nil); err != nil {
		t.Fatalf("Promote first canary: %v", err)
	}

	handler, err := NewHandler(repository)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	firstProbe := client.ProbeChannel(context.Background(), ChannelCanary, policy, now)
	assertPassingHostedProbe(t, firstProbe, first)
	if err := repository.Promote(ChannelStable, firstID, &firstProbe); err != nil {
		t.Fatalf("Promote first stable: %v", err)
	}

	secondID := second.Generation.Manifest.GenerationID
	if err := repository.Promote(ChannelDev, secondID, nil); err != nil {
		t.Fatalf("Promote second dev: %v", err)
	}
	if err := repository.Promote(ChannelStable, secondID, &firstProbe); err == nil {
		t.Fatal("stable promotion bypassed matching canary generation")
	}
	if err := repository.Promote(ChannelCanary, secondID, nil); err != nil {
		t.Fatalf("Promote second canary: %v", err)
	}
	secondProbe := client.ProbeChannel(context.Background(), ChannelCanary, policy, now)
	assertPassingHostedProbe(t, secondProbe, second)

	badLatency := secondProbe
	badLatency.Latency = policy.MaxProbeLatency + time.Nanosecond
	if err := repository.Promote(ChannelStable, secondID, &badLatency); err == nil {
		t.Fatal("stable promotion accepted latency SLO failure")
	}
	staleProbe := secondProbe
	staleProbe.ObservedAt = now.Add(-policy.MaxProbeAge - time.Nanosecond)
	if err := repository.Promote(ChannelStable, secondID, &staleProbe); err == nil {
		t.Fatal("stable promotion accepted stale probe evidence")
	}
	if err := repository.Promote(ChannelStable, secondID, &secondProbe); err != nil {
		t.Fatalf("Promote second stable: %v", err)
	}

	stable, err := client.FetchLatest(context.Background())
	if err != nil {
		t.Fatalf("FetchLatest second stable: %v", err)
	}
	if stable.Manifest.GenerationID != secondID {
		t.Fatalf("stable generation = %q, want %q", stable.Manifest.GenerationID, secondID)
	}
	if err := repository.Rollback(ChannelStable, firstID, "canary error-rate regression"); err != nil {
		t.Fatalf("Rollback stable: %v", err)
	}
	rolledBack, err := client.FetchLatest(context.Background())
	if err != nil {
		t.Fatalf("FetchLatest rollback: %v", err)
	}
	if rolledBack.Manifest.GenerationID != firstID {
		t.Fatalf("rolled-back generation = %q, want %q", rolledBack.Manifest.GenerationID, firstID)
	}
	canary, err := client.FetchChannel(context.Background(), ChannelCanary)
	if err != nil {
		t.Fatalf("FetchChannel canary: %v", err)
	}
	if canary.Manifest.GenerationID != secondID {
		t.Fatalf("rollback changed canary = %q, want %q", canary.Manifest.GenerationID, secondID)
	}

	events := repository.PromotionEvents()
	if len(events) < 10 {
		t.Fatalf("promotion telemetry count = %d, want rejected and successful attempts", len(events))
	}
	var sawRejectedOrder, sawRejectedSLO, sawRollback bool
	for index, event := range events {
		if event.Sequence != uint64(index+1) || event.ObservedAt.IsZero() {
			t.Fatalf("event[%d] = %#v", index, event)
		}
		if !event.Success && strings.Contains(event.Reason, "promotion requires") {
			sawRejectedOrder = true
		}
		if !event.Success && (strings.Contains(event.Reason, "latency") || strings.Contains(event.Reason, "stale")) {
			sawRejectedSLO = true
		}
		if event.Success && event.Action == PromotionActionRollback && event.From == secondID && event.To == firstID &&
			event.Reason == "canary error-rate regression" {
			sawRollback = true
		}
	}
	if !sawRejectedOrder || !sawRejectedSLO || !sawRollback {
		t.Fatalf("telemetry missing order/SLO/rollback evidence: %#v", events)
	}
	events[0].Reason = "caller mutation"
	if repository.PromotionEvents()[0].Reason == "caller mutation" {
		t.Fatal("PromotionEvents exposed mutable repository state")
	}
}

func TestHostedPromotionUnavailableOrStaleProbeCannotReachStable(t *testing.T) {
	policy := DefaultPromotionPolicy()
	repository, err := NewMemoryRepositoryWithPolicy(policy)
	if err != nil {
		t.Fatalf("NewMemoryRepositoryWithPolicy: %v", err)
	}
	published := hostedFixture(t)
	if err := repository.Publish(published); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	id := published.Generation.Manifest.GenerationID
	if err := repository.Promote(ChannelDev, id, nil); err != nil {
		t.Fatalf("Promote dev: %v", err)
	}
	if err := repository.Promote(ChannelCanary, id, nil); err != nil {
		t.Fatalf("Promote canary: %v", err)
	}
	now := published.Generation.Manifest.GeneratedAt.Add(policy.MaxGenerationAge + time.Nanosecond)
	repository.now = func() time.Time { return now }
	handler, err := NewHandler(repository)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	server := httptest.NewServer(handler)
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	stale := client.ProbeChannel(context.Background(), ChannelCanary, policy, now)
	if !stale.Available || stale.Fresh || stale.Failure == "" {
		t.Fatalf("stale probe = %#v", stale)
	}
	if err := repository.Promote(ChannelStable, id, &stale); err == nil {
		t.Fatal("stable promotion accepted stale generation")
	}
	server.Close()
	unavailable := client.ProbeChannel(context.Background(), ChannelCanary, policy, now)
	if unavailable.Available || unavailable.Failure == "" {
		t.Fatalf("unavailable probe = %#v", unavailable)
	}
}

func assertPassingHostedProbe(t *testing.T, probe PromotionProbe, published PublishedGeneration) {
	t.Helper()
	if !probe.Available || !probe.Fresh || probe.Failure != "" ||
		probe.Channel != ChannelCanary || probe.GenerationID != published.Generation.Manifest.GenerationID ||
		probe.ArtifactChecksum != published.Artifact.Checksum {
		t.Fatalf("probe = %#v, want passing identity-bound canary evidence", probe)
	}
}
