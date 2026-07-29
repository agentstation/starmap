package starmap

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func catalogUpdate(
	mutate func(*catalogs.Builder) error,
) UpdateFunc {
	return func(_ context.Context, current *catalogs.Catalog) (*Candidate, error) {
		builder, err := catalogs.NewBuilderFrom(current)
		if err != nil {
			return nil, err
		}
		if mutate != nil {
			if err := mutate(builder); err != nil {
				return nil, err
			}
		}
		catalog, err := builder.Build()
		if err != nil {
			return nil, err
		}
		return NewCandidate(catalog)
	}
}

func updateCatalog(
	t testing.TB,
	client *Client,
	update UpdateFunc,
) Publication {
	t.Helper()
	publication, err := client.Update(context.Background(), update)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	return publication
}
