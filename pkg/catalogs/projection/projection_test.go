package projection

import "testing"

func TestProjectionContractValuesRemainStable(t *testing.T) {
	t.Parallel()

	if StatusApplied != "applied" {
		t.Fatalf("StatusApplied = %q, want applied", StatusApplied)
	}
	if StatusPendingRepair != "pending_repair" {
		t.Fatalf("StatusPendingRepair = %q, want pending_repair", StatusPendingRepair)
	}
	if IssueWorkspaceFailed != "workspace_projection_failed" {
		t.Fatalf(
			"IssueWorkspaceFailed = %q, want workspace_projection_failed",
			IssueWorkspaceFailed,
		)
	}
}

func TestProjectionResultKeepsIndependentOutcomeFields(t *testing.T) {
	t.Parallel()

	result := Result{
		Path:              "/catalog",
		Status:            StatusPendingRepair,
		IssueCode:         IssueWorkspaceFailed,
		GenerationID:      "generation-1",
		WorkspaceChecksum: "sha256:workspace",
	}
	if result.Path != "/catalog" ||
		result.Status != StatusPendingRepair ||
		result.IssueCode != IssueWorkspaceFailed ||
		result.GenerationID != "generation-1" ||
		result.WorkspaceChecksum != "sha256:workspace" {
		t.Fatalf("Result changed values: %#v", result)
	}
}
