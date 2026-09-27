package runtime

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/logging"
)

func repairWorkspaceForStartup(ctx context.Context, client *starmap.Client) error {
	current, baseline := client.CurrentCatalogState(), client.EmbeddedCatalogState()
	if current.GenerationID == baseline.GenerationID && current.PayloadChecksum == baseline.PayloadChecksum && current.AuthorityHead == baseline.AuthorityHead {
		// Retaining the offline baseline does not select YAML authoring. Existing
		// trees and recovery records still reach the normal repair validator.
		present, err := workspace.HasProjectionState(client.WorkspacePath())
		if err == nil && !present {
			return ctx.Err()
		}
	}
	repair, err := client.RepairWorkspace(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch {
	case err != nil:
		logging.Warn().Err(err).Str("generation_id", repair.GenerationID).Str("workspace", repair.Path).
			Msg("Durable catalog is active; YAML workspace repair remains pending")
	case repair.IssueCode != "":
		logging.Warn().Str("generation_id", repair.GenerationID).Str("workspace", repair.Path).Str("issue_code", repair.IssueCode).
			Msg("Durable catalog is active; YAML workspace has operator changes")
	case repair.Changed:
		logging.Info().Str("generation_id", repair.GenerationID).Str("workspace", repair.Path).
			Msg("Repaired YAML workspace from durable catalog generation")
	}
	return nil
}
