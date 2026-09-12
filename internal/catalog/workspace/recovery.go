package workspace

import (
	"context"

	"github.com/agentstation/starmap/pkg/errors"
)

func recoverWorkspace(ctx context.Context, target string, writer *workspaceWriter) (bool, error) {
	return recoverWorkspaceExcept(ctx, target, writer, nil)
}

func recoverWorkspaceExcept(ctx context.Context, target string, writer *workspaceWriter, relocation *workspaceStage) (bool, error) {
	pending, err := pendingRelocation(ctx, target, writer)
	if err != nil {
		return false, err
	}
	if pending != nil {
		name := pending.name
		pending.releaseHandles()
		if relocation == nil || relocation.name != name {
			return false, replacementConflict(target, "legacy relocation requires migration recovery")
		}
	}
	recovered, err := recoverReplacement(ctx, target, writer)
	if err != nil {
		return recovered, errors.WrapResource("recover", "workspace replacement", target, err)
	}
	if err := recoverPreparationsExcept(ctx, target, writer, relocation); err != nil {
		return recovered, errors.WrapResource("recover", "workspace preparation", target, err)
	}
	return recovered, nil
}
