package runtime

import (
	"context"
	"testing"
	"time"
)

type fleetStartupPublication struct {
	FleetStore
	afterRead func()
}

func (s *fleetStartupPublication) CurrentPublication(ctx context.Context) (FleetSnapshot, error) {
	snapshot, err := s.FleetStore.CurrentPublication(ctx)
	if err == nil && s.afterRead != nil {
		callback := s.afterRead
		s.afterRead = nil
		callback()
	}
	return snapshot, err
}

func TestFleetRuntimeStartupRefreshesAfterOwnership(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	leader := openFleetRuntime(t, backend, "startup-leader", privateRuntimeDirectory(t))
	var latest FleetStatus
	session := &fleetStartupPublication{
		FleetStore: &fleetTestSession{backend: backend, session: "startup-replacement-process"},
		afterRead: func() {
			if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "latest", time.Now().UTC(), false)); err != nil {
				t.Fatal(err)
			}
			latest, _ = leader.FleetStatus()
			if err := leader.Close(); err != nil {
				t.Fatal(err)
			}
		},
	}
	replacement := openTestRuntime(t, WithFleetStore(session), WithSchedulerIdentity("startup-replacement"),
		WithStateDirectory(privateRuntimeDirectory(t)), WithSource(newStubSource("fleet-source")),
		withScheduleTimer(newStubScheduleTimer().after))
	status, _ := replacement.FleetStatus()
	if status.Head.Revision <= latest.Head.Revision || status.Head.GenerationID != latest.Head.GenerationID {
		t.Fatal("startup did not rebind the latest publication under its new grant")
	}
	provider, err := replacement.Catalog().Provider("manual-provider")
	if err != nil || provider.Models["latest"] == nil {
		t.Fatal("startup lost the publication made between bootstrap and ownership", err)
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if !sameLeaseGrant(backend.snapshots[status.Head].Publication.Grant, backend.lease) {
		t.Fatal("startup retained the former owner's publication grant")
	}
}
