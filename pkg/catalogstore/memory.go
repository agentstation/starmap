package catalogstore

import (
	"context"
	"sync"
)

// Memory is an in-process reference implementation of Store.
type Memory struct {
	mu          sync.RWMutex
	currentID   string
	generations map[string]Generation
}

// NewMemory creates an empty in-memory catalog store.
func NewMemory() *Memory {
	return &Memory{generations: make(map[string]Generation)}
}

// Current returns the currently active generation.
func (s *Memory) Current(ctx context.Context) (Generation, error) {
	if err := ctx.Err(); err != nil {
		return Generation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentID == "" {
		return Generation{}, currentNotFound()
	}
	return s.generations[s.currentID].Copy(), nil
}

// Get returns an immutable generation by ID.
func (s *Memory) Get(ctx context.Context, id string) (Generation, error) {
	if err := ctx.Err(); err != nil {
		return Generation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	generation, found := s.generations[id]
	if !found {
		return Generation{}, generationNotFound(id)
	}
	return generation.Copy(), nil
}

// Commit validates and atomically activates generation when current matches
// expectedGenerationID.
func (s *Memory) Commit(ctx context.Context, generation Generation, expectedGenerationID string) error {
	if err := validateCandidate(ctx, generation); err != nil {
		return err
	}
	candidate := generation.Copy()
	id := candidate.Manifest.GenerationID

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if existing, found := s.generations[id]; found {
		if !sameGeneration(existing, candidate) {
			return identityConflict(id)
		}
		if s.currentID == id {
			return nil
		}
	}
	if s.currentID != expectedGenerationID {
		return casConflict(expectedGenerationID, s.currentID)
	}
	s.generations[id] = candidate
	s.currentID = id
	return nil
}

var _ Store = (*Memory)(nil)
