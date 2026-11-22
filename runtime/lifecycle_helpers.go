package runtime

import (
	"context"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// joinRuntimeShutdown bounds the caller's wait without canceling resource cleanup.
// The cleanup retains directory ownership until every owned worker stops.
func joinRuntimeShutdown(bound time.Duration, cleanup func() error) error {
	joined := make(chan error, 1)
	go func() { joined <- cleanup() }()
	timer := time.NewTimer(bound)
	defer timer.Stop()
	select {
	case err := <-joined:
		return err
	case <-timer.C:
		return &errors.TimeoutError{Operation: "close starmap runtime", Duration: bound.String()}
	}
}

func repairWorkspaceForStartup(ctx context.Context, client *starmap.Client) error {
	current, baseline := client.CurrentCatalogState(), client.EmbeddedCatalogState()
	if current.GenerationID == baseline.GenerationID && current.PayloadChecksum == baseline.PayloadChecksum && current.AuthorityHead == baseline.AuthorityHead {
		// Retaining the offline baseline does not select YAML authoring.
		// Existing trees and recovery records still reach the normal repair validator.
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

// originPublicationStore applies explicit bootstrap inside the client's guarded commit.
// The publisher still validates the exact predecessor and atomically commits the generation.
type originPublicationStore struct {
	*permission.Publisher
	bootstrap bool
}

func (s *originPublicationStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if s.bootstrap {
		current, err := s.Current(ctx)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err == nil && current.Manifest.AuthorityHead == (catalogs.CatalogAuthorityHead{}) {
			return s.Bootstrap(ctx, generation, expected)
		}
	}
	return s.Publisher.Commit(ctx, generation, expected)
}
