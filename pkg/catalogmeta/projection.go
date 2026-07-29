package catalogmeta

// ProjectionStatus is the post-commit state of an optional human YAML
// workspace projection.
type ProjectionStatus string

const (
	// ProjectionStatusApplied means the committed generation was materialized
	// successfully to the requested human workspace.
	ProjectionStatusApplied ProjectionStatus = "applied"
	// ProjectionStatusPendingRepair means the generation remains durably active,
	// but its optional human workspace projection must be repaired.
	ProjectionStatusPendingRepair ProjectionStatus = "pending_repair"

	// ProjectionIssueWorkspaceFailed identifies a committed generation whose
	// optional workspace projection did not complete.
	ProjectionIssueWorkspaceFailed = "workspace_projection_failed"
)

// ProjectionResult reports the post-commit state of an optional human YAML
// workspace.
type ProjectionResult struct {
	// Path is the requested human workspace root.
	Path string
	// Status reports whether projection completed or requires repair.
	Status ProjectionStatus
	// IssueCode is empty after a successful projection.
	IssueCode string
	// GenerationID is the durable generation the workspace represents.
	GenerationID string
	// WorkspaceChecksum is present when the projected YAML became visible,
	// including when only the repair marker remains pending.
	WorkspaceChecksum string
}
