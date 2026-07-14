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

const (
	catalogGenerationResource = "catalog generation"
	catalogPayloadFile        = "catalog payload"
	catalogStoreComponent     = "catalog store"
	objectResource            = "object"
	validationRequiredMessage = "is required"
)

// Generation is an immutable manifest and its exact catalog payload bytes.
type Generation struct {
	Manifest catalogs.GenerationManifest
	Payload  []byte
}

// Copy returns a generation that does not share mutable slices with g.
func (g Generation) Copy() Generation {
	return Generation{
		Manifest: g.Manifest.Copy(),
		Payload:  append([]byte(nil), g.Payload...),
	}
}

// Validate verifies the manifest and its binding to the payload.
func (g Generation) Validate() error {
	if err := g.Manifest.Validate(); err != nil {
		return err
	}
	if err := g.Manifest.Payload.Verify(g.Payload); err != nil {
		return err
	}
	return nil
}

// Store commits and reads immutable catalog generations.
//
// Commit always performs compare-and-swap against expectedGenerationID. An
// empty expected ID means that no current generation may exist. Implementations
// must validate and persist the complete generation before changing Current.
// Repeating an already-successful identical commit is idempotent.
type Store interface {
	Current(context.Context) (Generation, error)
	Get(context.Context, string) (Generation, error)
	Commit(context.Context, Generation, string) error
}

func validateCandidate(ctx context.Context, generation Generation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return generation.Validate()
}

func sameGeneration(left, right Generation) bool {
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
	return &errors.NotFoundError{Resource: catalogGenerationResource, ID: currentFilename}
}

func generationNotFound(id string) error {
	return &errors.NotFoundError{Resource: catalogGenerationResource, ID: id}
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
		Resource: catalogGenerationResource,
		Expected: id,
		Actual:   id,
		Message:  "generation ID is already bound to different content",
	}
}
