package remote

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestHealthErrorClassificationDoesNotExposeSecrets(t *testing.T) {
	t.Parallel()

	const (
		secretEndpoint = "https://user:secret@example.com/catalog"
		secretBody     = "publisher response contains token-123"
	)
	err := &pkgerrors.APIError{
		Provider:   "starmap-server",
		Endpoint:   secretEndpoint,
		StatusCode: http.StatusForbidden,
		Message:    secretBody,
		Err:        errors.New(secretBody),
	}
	occurredAt := time.Date(2026, time.July, 29, 20, 0, 0, 0, time.UTC)
	healthError := classifyHealthError("stream_open", err, occurredAt)
	encoded, marshalErr := json.Marshal(healthError)
	if marshalErr != nil {
		t.Fatalf("Marshal health error: %v", marshalErr)
	}
	if strings.Contains(string(encoded), "secret") ||
		strings.Contains(string(encoded), "token-123") ||
		strings.Contains(string(encoded), "example.com") {
		t.Fatalf("health error exposed secret-bearing details: %s", encoded)
	}
	if healthError.Operation != "stream_open" ||
		healthError.Kind != "http" ||
		healthError.StatusCode != http.StatusForbidden ||
		!healthError.Terminal ||
		!healthError.OccurredAt.Equal(occurredAt) {
		t.Fatalf("health error = %#v", healthError)
	}
}

func TestHealthCatalogAgeIsIndependentOfTransportActivity(t *testing.T) {
	t.Parallel()

	generatedAt := time.Date(2026, time.July, 29, 18, 0, 0, 0, time.UTC)
	now := generatedAt.Add(2 * time.Hour)
	subscriber := &Subscriber{
		streamState: StreamStateStreaming,
		active: generationIdentity{
			id:          "generation-health",
			generatedAt: generatedAt,
		},
		now: func() time.Time { return now },
	}
	subscriber.recordHeartbeat()
	first := subscriber.Health()
	if first.CatalogAgeSeconds != int64((2*time.Hour)/time.Second) {
		t.Fatalf("catalog age = %d, want 7200", first.CatalogAgeSeconds)
	}

	now = now.Add(30 * time.Minute)
	subscriber.recordHeartbeat()
	second := subscriber.Health()
	if second.CatalogAgeSeconds != int64((150*time.Minute)/time.Second) {
		t.Fatalf("catalog age after heartbeat = %d, want 9000", second.CatalogAgeSeconds)
	}
	if !second.CatalogGeneratedAt.Equal(first.CatalogGeneratedAt) {
		t.Fatal("heartbeat changed the catalog generation timestamp")
	}
	if !second.LastHeartbeatAt.After(first.LastHeartbeatAt) {
		t.Fatal("heartbeat activity timestamp did not advance")
	}
}

func assertHeartbeatStreamHealth(
	t testing.TB,
	subscriber *Subscriber,
	generation catalogstore.Generation,
) time.Time {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		health := subscriber.Health()
		if health.StreamState == StreamStateStreaming &&
			health.ActiveGenerationID == generation.Manifest.GenerationID &&
			health.CatalogGeneratedAt.Equal(generation.Manifest.GeneratedAt) &&
			!health.LastHeartbeatAt.IsZero() &&
			health.LastEventAt.IsZero() &&
			!health.LastSuccessfulCatchUpAt.IsZero() {
			return health.CatalogGeneratedAt
		}
		if time.Now().After(deadline) {
			t.Fatalf("heartbeat stream health = %#v", health)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func assertStoppedCatalogHealth(
	t testing.TB,
	subscriber *Subscriber,
	generatedAt time.Time,
) {
	t.Helper()
	health := subscriber.Health()
	if health.StreamState != StreamStateStopped ||
		!health.CatalogGeneratedAt.Equal(generatedAt) {
		t.Fatalf("stopped subscriber health = %#v", health)
	}
}

func assertRecoveredStreamHealth(
	t testing.TB,
	subscriber *Subscriber,
	generation catalogstore.Generation,
	retries uint64,
) {
	t.Helper()
	health := subscriber.Health()
	if health.StreamState != StreamStateStreaming ||
		health.ActiveGenerationID != generation.Manifest.GenerationID ||
		!health.CatalogGeneratedAt.Equal(generation.Manifest.GeneratedAt) ||
		health.Retries != retries ||
		health.LastSuccessfulCatchUpAt.IsZero() {
		t.Fatalf("recovered subscriber health = %#v", health)
	}
	if health.LastError == nil || health.LastError.Operation != "stream_open" ||
		health.LastError.Kind != "http" ||
		health.LastError.StatusCode != http.StatusServiceUnavailable ||
		health.LastError.Terminal {
		t.Fatalf("last recoverable error = %#v", health.LastError)
	}
}
