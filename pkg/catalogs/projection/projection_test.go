package projection

import "testing"

func TestProjectionContractValuesRemainStable(t *testing.T) {
	t.Parallel()

	if ProjectionStatusApplied != "applied" {
		t.Fatalf("ProjectionStatusApplied = %q, want applied", ProjectionStatusApplied)
	}
	if ProjectionStatusPendingRepair != "pending_repair" {
		t.Fatalf("ProjectionStatusPendingRepair = %q, want pending_repair", ProjectionStatusPendingRepair)
	}
	if ProjectionIssueWorkspaceFailed != "workspace_projection_failed" {
		t.Fatalf(
			"ProjectionIssueWorkspaceFailed = %q, want workspace_projection_failed",
			ProjectionIssueWorkspaceFailed,
		)
	}
}

func TestProjectionResultKeepsIndependentOutcomeFields(t *testing.T) {
	t.Parallel()

	result := ProjectionResult{
		Path:              "/catalog",
		Status:            ProjectionStatusPendingRepair,
		IssueCode:         ProjectionIssueWorkspaceFailed,
		GenerationID:      "generation-1",
		WorkspaceChecksum: "sha256:workspace",
	}
	if result.Path != "/catalog" ||
		result.Status != ProjectionStatusPendingRepair ||
		result.IssueCode != ProjectionIssueWorkspaceFailed ||
		result.GenerationID != "generation-1" ||
		result.WorkspaceChecksum != "sha256:workspace" {
		t.Fatalf("ProjectionResult changed values: %#v", result)
	}
}
