package storage

import (
	"context"
	"crypto/rand"
	stderrors "errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const coordinatedReleaseTimeout = 30 * time.Second

// Current returns a generation that was current during this call.
func (s *CoordinatedObject) Current(ctx context.Context) (catalogs.Generation, error) {
	for range maxCoordinationAttempts {
		state, _, err := s.load(ctx)
		if err != nil {
			return catalogs.Generation{}, err
		}
		if state.Current == "" {
			return catalogs.Generation{}, currentNotFound()
		}
		generation, err := s.Get(ctx, state.Current)
		if errors.IsNotFound(err) {
			continue
		}
		return generation, err
	}
	return catalogs.Generation{}, coordinatedBusy()
}

// Get protects stored bytes during the read and returns an independent generation.
func (s *CoordinatedObject) Get(ctx context.Context, id string) (catalogs.Generation, error) {
	generation, release, err := s.AcquireGeneration(ctx, id)
	if err != nil {
		return catalogs.Generation{}, err
	}
	if err := release(); err != nil {
		return catalogs.Generation{}, err
	}
	return generation, nil
}

// AcquireGeneration protects stored bytes until an explicit successful release.
// The release function is idempotent, retryable after failure, and independent of ctx.
func (s *CoordinatedObject) AcquireGeneration(ctx context.Context, id string) (catalogs.Generation, func() error, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, nil, err
	}
	if !coordinatedIdentity(id) {
		return catalogs.Generation{}, nil, coordinatedInvalid("generation_id", "a bounded generation identity is required")
	}
	token := rand.Text()
	state, entry, err := s.claimReader(ctx, id, token)
	if err != nil {
		if !errors.IsNotFound(err) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), coordinatedReleaseTimeout)
			defer cancel()
			err = stderrors.Join(err, s.releaseReader(cleanupCtx, token))
		}
		return catalogs.Generation{}, nil, err
	}
	var mu sync.Mutex
	released := false
	release := func() error {
		mu.Lock()
		defer mu.Unlock()
		if released {
			return nil
		}
		releaseCtx, cancel := context.WithTimeout(context.Background(), coordinatedReleaseTimeout)
		defer cancel()
		err := s.releaseReader(releaseCtx, token)
		if err == nil {
			released = true
		}
		return err
	}
	value, err := s.objects.Get(ctx, s.uploadKey(entry.UploadID))
	if err != nil {
		return catalogs.Generation{}, nil, stderrors.Join(err, release())
	}
	if value.Version != entry.ObjectVersion || int64(len(value.Data)) != entry.ObjectBytes {
		return catalogs.Generation{}, nil, stderrors.Join(coordinatedInvalid("object_version", "stored object no longer matches its committed acknowledgment"), release())
	}
	generation, err := decodeCoordinatedObject(value.Data, state.StoreID, entry.UploadID)
	if err != nil {
		return catalogs.Generation{}, nil, stderrors.Join(err, release())
	}
	descriptor, _, err := coordinatedCandidate(ctx, generation)
	if err == nil && (generation.Manifest.GenerationID != id || !sameCoordinatedCandidate(descriptor, entry)) {
		err = coordinatedInvalid("generation", "stored bytes do not match the registered generation")
	}
	if err != nil {
		return catalogs.Generation{}, nil, stderrors.Join(err, release())
	}
	return generation, release, nil
}

func (s *CoordinatedObject) claimReader(ctx context.Context, id, token string) (coordinatedRegistry, coordinatedGeneration, error) {
	for range maxCoordinationAttempts {
		state, version, err := s.load(ctx)
		if err != nil {
			return state, coordinatedGeneration{}, err
		}
		entry, exists := state.Generations[id]
		if !exists || !entry.Committed {
			return state, entry, generationNotFound(id)
		}
		state.Readers[token] = coordinatedReader{UploadID: entry.UploadID, OwnerID: s.config.OwnerID}
		if err := s.saveRegistry(ctx, state, version); errors.IsConflict(err) {
			continue
		} else if err != nil {
			return state, entry, err
		}
		return state, entry, nil
	}
	return coordinatedRegistry{}, coordinatedGeneration{}, coordinatedBusy()
}

func (s *CoordinatedObject) releaseReader(ctx context.Context, token string) error {
	for range maxCoordinationAttempts {
		state, version, err := s.load(ctx)
		if err != nil {
			return err
		}
		if _, exists := state.Readers[token]; !exists {
			return nil
		}
		delete(state.Readers, token)
		err = s.saveRegistry(ctx, state, version)
		if errors.IsConflict(err) {
			continue
		}
		return err
	}
	return coordinatedBusy()
}

// ObjectReaderClaim identifies one durable reader protection claim.
type ObjectReaderClaim struct {
	Token        string
	GenerationID string
	OwnerID      string
}

// ObjectReaderClaims binds an inspection to the exact coordination revision.
type ObjectReaderClaims struct {
	Revision string
	Claims   []ObjectReaderClaim
}

// ReaderClaims reports durable readers for operator diagnosis and fenced recovery.
func (s *CoordinatedObject) ReaderClaims(ctx context.Context) (ObjectReaderClaims, error) {
	state, version, err := s.load(ctx)
	if err != nil {
		return ObjectReaderClaims{}, err
	}
	result := ObjectReaderClaims{Revision: version}
	ids := make(map[string]string, len(state.Generations))
	for id, entry := range state.Generations {
		ids[entry.UploadID] = id
	}
	for token, reader := range state.Readers {
		result.Claims = append(result.Claims, ObjectReaderClaim{Token: token, GenerationID: ids[reader.UploadID], OwnerID: reader.OwnerID})
	}
	slices.SortFunc(result.Claims, func(a, b ObjectReaderClaim) int { return strings.Compare(a.Token, b.Token) })
	return result, nil
}

// ReleaseFencedOwner removes the inspected owner's claims after external process fencing.
// The caller must stop every process using ownerID before this operation.
// A changed coordination revision refuses recovery without removing any claim.
func (s *CoordinatedObject) ReleaseFencedOwner(ctx context.Context, ownerID, revision string) error {
	if !coordinatedIdentity(ownerID) || revision == "" {
		return coordinatedInvalid("reader_recovery", "owner identity and inspected revision are required")
	}
	state, current, err := s.load(ctx)
	if err != nil {
		return err
	}
	if current != revision {
		return &errors.ConflictError{Resource: "catalog reader recovery", Expected: revision, Actual: current}
	}
	for token, reader := range state.Readers {
		if reader.OwnerID == ownerID {
			delete(state.Readers, token)
		}
	}
	return s.saveRegistry(ctx, state, current)
}
