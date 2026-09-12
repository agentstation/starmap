package workspace

import (
	"context"

	"github.com/agentstation/starmap/pkg/errors"
)

func recoverWorkspace(ctx context.Context, target string, writer *workspaceWriter) (bool, error) {
	recovered, err := recoverReplacement(ctx, target, writer)
	if err != nil {
		return recovered, errors.WrapResource("recover", "workspace replacement", target, err)
	}
	if err := recoverPreparations(ctx, target, writer); err != nil {
		return recovered, errors.WrapResource("recover", "workspace preparation", target, err)
	}
	return recovered, nil
}
