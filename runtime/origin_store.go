package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
)

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
