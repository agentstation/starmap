package projection

// Status is the post-commit state of an optional human YAML workspace
// projection.
type Status string

const (
	// StatusApplied means the workspace matches the committed generation.
	StatusApplied Status = "applied"
	// StatusPendingRepair means the generation remains durably active while the
	// optional human catalog workspace needs repair.
	StatusPendingRepair Status = "pending_repair"

	// IssueWorkspaceFailed identifies a committed generation whose
	// optional workspace projection did not complete.
	IssueWorkspaceFailed = "workspace_projection_failed"
)

// Result reports the post-commit state of an optional human YAML workspace.
type Result struct {
	// Path is the requested human catalog workspace root.
	Path string
	// Status reports whether projection completed or requires repair.
	Status Status
	// IssueCode is empty after a successful projection.
	IssueCode string
	// GenerationID is the durable generation the workspace represents.
	GenerationID string
	// WorkspaceChecksum is present when the projected YAML became visible,
	// including when only the repair marker remains pending.
	WorkspaceChecksum string
}
