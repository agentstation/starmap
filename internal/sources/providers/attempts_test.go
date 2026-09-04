package providers

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// TestMissingCredentialSkipsProviderWithoutRequest proves that an unconfigured
// catalog-acquisition credential skips the provider before any client exists,
// so the source opens no connection and sends no request. It also proves that
// a configured sibling still answers in the same observation.
func TestMissingCredentialSkipsProviderWithoutRequest(t *testing.T) {
	unconfigured := providerForTest("provider-unconfigured")
	unconfigured.Credentials = requiredAPIKeyCredentials("STARMAP_TEST_ABSENT_API_KEY")
	configured := providerForTest("provider-public")

	var clientsBuilt atomic.Int64
	var requests atomic.Int64
	var attempts []sources.ProviderAttempt

	src := newTestSource(
		newProviderSet(unconfigured, configured),
		WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
			clientsBuilt.Add(1)
			return fakeProviderClient{
				models: []catalogs.Model{{ID: "model-a", Name: "Model A"}},
				onList: func() { requests.Add(1) },
			}, nil
		}),
		WithMaxConcurrency(1),
		WithAttemptSink(func(attempt sources.ProviderAttempt) {
			attempts = append(attempts, attempt)
		}),
	)

	observation, reported, err := src.ObserveAttempts(context.Background())
	if err != nil {
		t.Fatalf("ObserveAttempts failed: %v", err)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("validate observation: %v", err)
	}

	if len(reported) != 2 {
		t.Fatalf("attempt count = %d, want 2", len(reported))
	}
	byProvider := make(map[catalogs.ProviderID]sources.ProviderAttempt, len(reported))
	for _, attempt := range reported {
		if err := attempt.Validate(); err != nil {
			t.Fatalf("validate attempt %s: %v", attempt.ProviderID, err)
		}
		byProvider[attempt.ProviderID] = attempt
	}

	skipped := byProvider["provider-unconfigured"]
	if skipped.Outcome != sources.ProviderOutcomeSkippedNotConfigured {
		t.Fatalf("outcome = %q, want %q",
			skipped.Outcome, sources.ProviderOutcomeSkippedNotConfigured)
	}
	if skipped.Reason != sources.ProviderReasonCredentialUnavailable {
		t.Fatalf("reason = %q, want %q",
			skipped.Reason, sources.ProviderReasonCredentialUnavailable)
	}
	if skipped.Requested {
		t.Fatal("a skipped provider must not report a request")
	}

	answered := byProvider["provider-public"]
	if answered.Outcome != sources.ProviderOutcomeSucceeded {
		t.Fatalf("outcome = %q, want %q",
			answered.Outcome, sources.ProviderOutcomeSucceeded)
	}
	if !answered.Requested {
		t.Fatal("an answered provider must report a request")
	}

	// The skipped provider builds no client and sends no request. Exactly one
	// provider remains, so exactly one client and one request may exist.
	if built := clientsBuilt.Load(); built != 1 {
		t.Fatalf("clients built = %d, want 1", built)
	}
	if sent := requests.Load(); sent != 1 {
		t.Fatalf("provider requests = %d, want 1", sent)
	}
	if len(attempts) != 2 {
		t.Fatalf("sink attempts = %d, want 2", len(attempts))
	}
}

// TestSkippedProviderDegradesObservationWithSafeReason proves the observation
// records the skip with a safe reason code and no credential value.
func TestSkippedProviderDegradesObservationWithSafeReason(t *testing.T) {
	unconfigured := providerForTest("provider-unconfigured")
	unconfigured.Credentials = requiredAPIKeyCredentials("STARMAP_TEST_ABSENT_API_KEY")

	src := newTestSource(
		newProviderSet(unconfigured),
		WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
			t.Fatal("client factory must not run for a skipped provider")
			return nil, nil
		}),
	)

	observation, attempts, err := src.ObserveAttempts(context.Background())
	if err != nil {
		t.Fatalf("ObserveAttempts failed: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(attempts))
	}
	if observation.Status != sources.ObservationStatusDegraded {
		t.Fatalf("status = %q, want degraded", observation.Status)
	}
	found := false
	for _, issue := range observation.Issues {
		if issue.Code != sources.ObservationIssueCodeMissingCredentials {
			continue
		}
		found = true
		if issue.Subject != "provider-unconfigured" {
			t.Fatalf("issue subject = %q", issue.Subject)
		}
	}
	if !found {
		t.Fatal("observation must record a missing-credential issue")
	}
}
