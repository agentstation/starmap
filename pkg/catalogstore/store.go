// Package catalogstore provides durable generation-oriented catalog storage.
package catalogstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Store commits and reads immutable catalog generations.
//
// Commit always performs compare-and-swap against expectedGenerationID. An
// empty expected ID means that no current generation may exist. Implementations
// must validate and persist the complete generation before changing Current.
// Repeating an already-successful identical commit is idempotent.
//
// Starmap provides memory, filesystem, and conditional object-storage
// implementations. Embedding applications own and inject any database-backed
// implementation, including its driver, schema, migrations, credentials,
// connection pool, backups, and lifecycle.
type Store interface {
	Current(context.Context) (catalogs.Generation, error)
	Get(context.Context, string) (catalogs.Generation, error)
	Commit(context.Context, catalogs.Generation, string) error
}

func validateCandidate(ctx context.Context, generation catalogs.Generation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return generation.Validate()
}

func sameGeneration(left, right catalogs.Generation) bool {
	if !bytes.Equal(left.Payload, right.Payload) {
		return false
	}
	leftManifest, leftErr := json.Marshal(left.Manifest)
	rightManifest, rightErr := json.Marshal(right.Manifest)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftManifest, rightManifest)
}

func marshalManifest(manifest catalogs.GenerationManifest) ([]byte, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, &errors.ValidationError{
			Field:   "manifest",
			Value:   manifest.GenerationID,
			Message: fmt.Sprintf("cannot encode JSON: %v", err),
		}
	}
	return data, nil
}

func currentNotFound() error {
	return &errors.NotFoundError{Resource: "catalog generation", ID: "current"}
}

func generationNotFound(id string) error {
	return &errors.NotFoundError{Resource: "catalog generation", ID: id}
}

func casConflict(expected, actual string) error {
	return &errors.ConflictError{
		Resource: "catalog current generation",
		Expected: expected,
		Actual:   actual,
	}
}

func identityConflict(id string) error {
	return &errors.ConflictError{
		Resource: "catalog generation",
		Expected: id,
		Actual:   id,
		Message:  "generation ID is already bound to different content",
	}
}
