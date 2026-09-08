package starmap

import (
	"context"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/errors"
)

// WorkspaceRepairResult describes an explicit repair of the optional YAML workspace.
type WorkspaceRepairResult struct {
	// Path identifies the configured workspace. It is empty without a workspace.
	Path string
	// GenerationID identifies the durable catalog used for repair.
	// It is empty when the client has no durable generation to project.
	GenerationID string
	// Changed reports a completed repair that replaced workspace files.
	Changed bool
	// IssueCode identifies preserved operator changes that prevented repair.
	IssueCode string
}

// RepairWorkspace explicitly restores the workspace from this client's durable catalog.
// It preserves operator changes and serializes repair with catalog publications.
// It never commits a generation or changes the in-memory catalog sequence.
// A client without a workspace or durable generation writes no files.
func (c *Client) RepairWorkspace(ctx context.Context) (WorkspaceRepairResult, error) {
	var result WorkspaceRepairResult
	if c == nil || ctx == nil {
		return result, &errors.ValidationError{Field: "workspace_repair", Message: "requires a client and context"}
	}
	ctx, cancel := context.WithTimeout(ctx, catalogProjectionTimeout)
	defer cancel()
	release, err := c.updates.acquire(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	if c.options == nil {
		return result, nil
	}
	result.Path = c.options.catalogPath
	if result.Path == "" || isNilCatalogStore(c.options.catalogStore) {
		return result, nil
	}
	c.mu.RLock()
	result.GenerationID = c.generationID
	c.mu.RUnlock()
	if result.GenerationID == "" {
		return result, nil
	}
	state := c.CurrentCatalogState()
	repair, err := workspace.Repair(ctx, result.Path, state.Catalog, workspace.Identity{
		GenerationID: result.GenerationID, PayloadChecksum: state.PayloadChecksum,
	})
	result.Changed = repair.Status == workspace.RepairStatusRepaired
	result.IssueCode = repair.IssueCode
	return result, err
}
