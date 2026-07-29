package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

// starportStore is a small external reference adapter. A production Starport
// adapter would replace the locked maps with its own dialect-specific
// transaction and compare-and-swap implementation.
type starportStore struct {
	mu          sync.RWMutex
	current     string
	generations map[string]catalogstore.Generation
}

func newStarportStore() *starportStore {
	return &starportStore{generations: make(map[string]catalogstore.Generation)}
}

func (s *starportStore) Current(ctx context.Context) (catalogstore.Generation, error) {
	if err := ctx.Err(); err != nil {
		return catalogstore.Generation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == "" {
		return catalogstore.Generation{}, &errors.NotFoundError{
			Resource: "catalog generation",
			ID:       "current",
		}
	}
	return s.generations[s.current].Copy(), nil
}

func (s *starportStore) Get(
	ctx context.Context,
	id string,
) (catalogstore.Generation, error) {
	if err := ctx.Err(); err != nil {
		return catalogstore.Generation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	generation, found := s.generations[id]
	if !found {
		return catalogstore.Generation{}, &errors.NotFoundError{
			Resource: "catalog generation",
			ID:       id,
		}
	}
	return generation.Copy(), nil
}

func (s *starportStore) Commit(
	ctx context.Context,
	generation catalogstore.Generation,
	expectedGenerationID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := generation.Validate(); err != nil {
		return err
	}
	candidate := generation.Copy()
	id := candidate.Manifest.GenerationID

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, found := s.generations[id]; found {
		if !sameExternalGeneration(existing, candidate) {
			return &errors.ConflictError{
				Resource: "catalog generation",
				Expected: id,
				Actual:   id,
				Message:  "generation ID is already bound to different content",
			}
		}
		if s.current == id {
			return nil
		}
	}
	if s.current != expectedGenerationID {
		return &errors.ConflictError{
			Resource: "catalog current generation",
			Expected: expectedGenerationID,
			Actual:   s.current,
		}
	}
	s.generations[id] = candidate
	s.current = id
	return nil
}

func sameExternalGeneration(left, right catalogstore.Generation) bool {
	if !bytes.Equal(left.Payload, right.Payload) {
		return false
	}
	leftManifest, leftErr := json.Marshal(left.Manifest)
	rightManifest, rightErr := json.Marshal(right.Manifest)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftManifest, rightManifest)
}

var _ catalogstore.Store = (*starportStore)(nil)
