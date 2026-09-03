package operations

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestRegistryReportsProgressAndCompletion(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	started := make(chan struct{})
	registry := newTestRegistry(t)

	accepted, err := registry.Start(KindCatalogUpdate, func(context.Context) (map[string]any, error) {
		close(started)
		<-release
		return map[string]any{"total_changes": 7}, nil
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if accepted.State != StateAccepted || accepted.ID == "" {
		t.Fatalf("accepted = %#v, want an identified accepted operation", accepted)
	}

	<-started
	running, found := registry.Status(accepted.ID)
	if !found || running.State != StateRunning {
		t.Fatalf("running status = %#v found=%t, want a running operation", running, found)
	}

	close(release)
	<-registry.Done(accepted.ID)
	final, found := registry.Status(accepted.ID)
	if !found || final.State != StateSucceeded {
		t.Fatalf("final status = %#v found=%t, want a succeeded operation", final, found)
	}
	if final.Detail["total_changes"] != 7 {
		t.Fatalf("detail = %#v, want the run summary", final.Detail)
	}
	if final.Reason != "" {
		t.Fatalf("reason = %q, want none for a success", final.Reason)
	}
}

func TestRegistryReportsBoundedFailureReason(t *testing.T) {
	t.Parallel()

	const secret = "token sk-live-should-never-appear"
	registry := newTestRegistry(t)

	accepted, err := registry.Start(KindCatalogUpdate, func(context.Context) (map[string]any, error) {
		return nil, &errors.AuthenticationError{Provider: "openai", Message: secret}
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-registry.Done(accepted.ID)

	final, _ := registry.Status(accepted.ID)
	if final.State != StateFailed {
		t.Fatalf("state = %q, want %q", final.State, StateFailed)
	}
	if final.Reason != sources.ProviderReasonCredentialUnavailable {
		t.Fatalf(
			"reason = %q, want %q",
			final.Reason, sources.ProviderReasonCredentialUnavailable,
		)
	}
	if !final.Reason.Valid() {
		t.Fatalf("reason %q is outside the closed set", final.Reason)
	}
}

func TestRegistryCancelsALiveOperation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	registry := newTestRegistry(t)

	accepted, err := registry.Start(KindCatalogUpdate, func(ctx context.Context) (map[string]any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-started

	if _, found := registry.Cancel(accepted.ID); !found {
		t.Fatal("Cancel did not find the live operation")
	}
	<-registry.Done(accepted.ID)
	final, _ := registry.Status(accepted.ID)
	if final.State != StateCanceled {
		t.Fatalf("state = %q, want %q", final.State, StateCanceled)
	}
	if _, found := registry.Cancel("unknown"); found {
		t.Fatal("Cancel found an operation that the registry does not hold")
	}
}

func TestRegistryEvictsTerminalHistoryOnly(t *testing.T) {
	t.Parallel()

	registry := NewRegistry(WithRetained(1))
	t.Cleanup(func() { closeRegistry(t, registry) })

	started := make(chan struct{})
	live, err := registry.Start(KindCatalogUpdate, func(ctx context.Context) (map[string]any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatalf("Start live: %v", err)
	}
	<-started

	for range 3 {
		finished, err := registry.Start(KindCatalogUpdate, func(context.Context) (map[string]any, error) {
			return nil, nil
		})
		if err != nil {
			t.Fatalf("Start finished: %v", err)
		}
		<-registry.Done(finished.ID)
	}

	if _, found := registry.Status(live.ID); !found {
		t.Fatal("eviction dropped a live operation")
	}
}

func TestRegistryMetricsUseBoundedLabels(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	accepted, err := registry.Start(KindCatalogUpdate, func(context.Context) (map[string]any, error) {
		return nil, &errors.ParseError{Message: "unexpected body"}
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-registry.Done(accepted.ID)

	samples := registry.Metrics()
	if len(samples) == 0 {
		t.Fatal("Metrics returned no rows")
	}
	for _, sample := range samples {
		if !sample.Kind.Valid() {
			t.Fatalf("kind %q is outside the closed set", sample.Kind)
		}
		if !sample.State.Valid() {
			t.Fatalf("state %q is outside the closed set", sample.State)
		}
		if sample.Reason != "" && !sample.Reason.Valid() {
			t.Fatalf("reason %q is outside the closed set", sample.Reason)
		}
		if sample.Total < 1 {
			t.Fatalf("total = %d, want a counted state entry", sample.Total)
		}
	}
}

func TestRegistryCloseCancelsLiveOperations(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	registry := NewRegistry()
	accepted, err := registry.Start(KindCatalogUpdate, func(ctx context.Context) (map[string]any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := registry.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	final, _ := registry.Status(accepted.ID)
	if final.State != StateCanceled {
		t.Fatalf("state after Close = %q, want %q", final.State, StateCanceled)
	}
	if _, err := registry.Start(KindCatalogUpdate, func(context.Context) (map[string]any, error) {
		return nil, nil
	}); err == nil {
		t.Fatal("a closed registry accepted a new operation")
	}
}

func TestRegistryStampsTheInjectedClock(t *testing.T) {
	t.Parallel()

	stamp := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	registry := NewRegistry(
		WithClock(func() time.Time { return stamp }),
		WithTimeout(time.Minute),
	)
	t.Cleanup(func() { closeRegistry(t, registry) })

	accepted, err := registry.Start(KindCatalogUpdate, func(context.Context) (map[string]any, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-registry.Done(accepted.ID)

	final, _ := registry.Status(accepted.ID)
	if !final.AcceptedAt.Equal(stamp) || !final.CompletedAt.Equal(stamp) {
		t.Fatalf("timestamps = %s/%s, want the injected clock %s",
			final.AcceptedAt, final.CompletedAt, stamp)
	}
}

func TestRegistryRejectsAnUnknownKind(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	_, err := registry.Start("catalog_delete", func(context.Context) (map[string]any, error) {
		return nil, nil
	})
	var validationErr *errors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error = %v, want a validation error", err)
	}
}

// newTestRegistry returns a registry that closes with the test.
func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	t.Cleanup(func() { closeRegistry(t, registry) })
	return registry
}

// closeRegistry joins one registry inside a bounded wait.
func closeRegistry(t *testing.T, registry *Registry) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := registry.Close(ctx); err != nil {
		t.Errorf("Close: %v", err)
	}
}
