package handlers

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/runtime/status"
)

func TestRuntimeReadinessReportsRetentionCapacity(t *testing.T) {
	report := runtimeReadiness(status.Status{Usable: true, Retention: status.RetentionStatus{
		Enabled: true, Interval: time.Hour, Health: status.HealthDegraded, Reason: "required_content_exceeds_limit",
		GenerationCollection: "supported", MaxGenerations: 1, Generations: 2, ProtectedGenerations: 2,
		ProtectedBytes: 100, GenerationBytes: 100, RemovedGenerations: 3, RemovedInputs: 4, OverLimit: true,
	}})
	retention, ok := report["retention"].(map[string]any)
	if !ok {
		t.Fatal("readiness has no retention diagnostics")
	}
	if report["usable"] != true || retention["over_limit"] != true || retention["interval_seconds"] != int64(3600) ||
		retention["protected_generations"] != 2 || retention["removed_inputs"] != 4 || retention["reason"] != "required_content_exceeds_limit" {
		t.Fatalf("capacity changed readiness or lost diagnostics: %+v", report)
	}
}
