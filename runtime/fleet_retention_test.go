package runtime

import (
	"context"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// fleetCollectSession records runtime dispatch. Starport tests the native storage contract.
type fleetCollectSession struct {
	*fleetTestSession
	mu       sync.Mutex
	requests []storage.RetentionRequest
}

func (s *fleetCollectSession) Collect(_ context.Context, request storage.RetentionRequest) (storage.RetentionReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, request)
	return storage.RetentionReport{Publications: &storage.PublicationRetentionReport{After: storage.PublicationRetentionUsage{Receipts: 3, EncodedBytes: 456, RecoveryBytes: 123, ReaderClaims: 1}}}, nil
}

func TestFleetRetentionUsesCanonicalPolicyAndExplicitCollection(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			session := &fleetCollectSession{fleetTestSession: &fleetTestSession{backend: newFleetRuntimeBackend(t), session: "collector-process"}}
			r := openTestRuntime(t, WithFleetStore(session), WithSchedulerIdentity("collector"), WithStateDirectory(privateRuntimeDirectory(t)),
				WithSource(newStubSource("fleet-source")), withScheduleTimer(newStubScheduleTimer().after),
				WithRetentionEnabled(enabled), WithRetentionMaxGenerations(7), WithRetentionMaxBytes(123456))
			if got := r.RetentionSnapshot(); got.GenerationCollection != "supported" || got.Enabled != enabled {
				t.Fatalf("coordinated fleet collection not exposed: %+v", got)
			}
			if enabled {
				if err := r.collectRetainedState(t.Context()); err != nil {
					t.Fatal(err)
				}
				status := r.RetentionSnapshot()
				if !status.PublicationAccounting || status.Publications.After.Receipts != 3 || status.Publications.After.EncodedBytes != 456 || status.Publications.After.RecoveryBytes != 123 || status.Publications.After.ReaderClaims != 1 {
					t.Fatalf("fleet accounting not delivered: %+v", status)
				}
				status.Publications.After.Receipts = 99
				if r.RetentionSnapshot().Publications.After.Receipts != 3 {
					t.Fatal("status is not an independent value")
				}

			} else {
				if _, err := r.Client().CollectGenerations(t.Context(), storage.RetentionRequest{ExpectedGenerationID: r.State().GenerationID, MaxGenerations: 7, MaxBytes: 123456}); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			session.mu.Lock()
			defer session.mu.Unlock()
			if len(session.requests) != 1 {
				t.Fatalf("collection calls=%d, want one explicit call", len(session.requests))
			}
			request := session.requests[0]
			if request.MaxGenerations != 7 || request.MaxBytes != 123456 || request.ExpectedGenerationID == "" {
				t.Fatalf("policy not delivered: %+v", request)
			}
		})
	}
}
