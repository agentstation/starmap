package settings

import (
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/errors"
)

// TestCascadeSubscriberCarriesTheCanonicalBounds proves that the composition
// step maps the transfer bounds, the startup spread, and the identity onto the
// cascade subscriber. A deployment that bounds its transfers must bound the
// cascade too. A fleet that spreads its cold work must also spread the first
// cascade connection.
func TestCascadeSubscriberCarriesTheCanonicalBounds(t *testing.T) {
	values := map[string]string{
		Source:              string(starmap.SourceStarmap),
		SourceURL:           "https://catalog.example.test",
		SourceAPIKey:        "test-key",
		StartupSpread:       "90s",
		TransferIdleTimeout: "7s",
		TransferMaxDuration: "3m",
		SchedulerIdentity:   "replica-a",
	}
	config, err := Load(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	subscriber, err := cascadeSubscriber(config)
	if err != nil {
		t.Fatalf("cascadeSubscriber: %v", err)
	}
	if subscriber.TransferPolicy == nil {
		t.Fatal("the cascade subscriber carries no transfer policy")
	}
	if subscriber.TransferPolicy.IdleTimeout != 7*time.Second {
		t.Fatalf("idle timeout = %s, want 7s", subscriber.TransferPolicy.IdleTimeout)
	}
	if subscriber.TransferPolicy.MaxDuration != 3*time.Minute {
		t.Fatalf("max duration = %s, want 3m", subscriber.TransferPolicy.MaxDuration)
	}
	if subscriber.TransferPolicy.ResponseHeaderTimeout <= 0 {
		t.Fatal("the transfer policy bounds no response header wait")
	}
	if subscriber.StartupSpread != 90*time.Second {
		t.Fatalf("startup spread = %s, want 90s", subscriber.StartupSpread)
	}
	if subscriber.Identity.Instance != "replica-a" {
		t.Fatalf("instance identity = %q, want replica-a", subscriber.Identity.Instance)
	}
	if subscriber.PollingFallback == nil ||
		subscriber.PollingFallback.AfterFailures != cascadeFallbackAfterFailures {
		t.Fatalf("polling fallback = %#v, want %d failures",
			subscriber.PollingFallback, cascadeFallbackAfterFailures)
	}
	if subscriber.APIKey == "" {
		t.Fatal("the cascade subscriber carries no credential")
	}
}

// TestCascadeSubscriberNeedsAnUpstreamURL proves that the mapping names the
// missing setting instead of building a subscriber with no upstream.
func TestCascadeSubscriberNeedsAnUpstreamURL(t *testing.T) {
	_, err := cascadeSubscriber(Config{SourceKind: starmap.SourceStarmap})
	var configErr *errors.ConfigError
	if !stderrors.As(err, &configErr) {
		t.Fatalf("cascadeSubscriber error = %v, want a ConfigError", err)
	}
}
