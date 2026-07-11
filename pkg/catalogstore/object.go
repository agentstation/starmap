package catalogstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ObjectValue is one versioned object returned by an ObjectBackend.
type ObjectValue struct {
	Data    []byte
	Version string
}

// ObjectPutCondition configures an atomic conditional object write.
type ObjectPutCondition struct {
	IfAbsent  bool
	IfVersion string
}

// ObjectBackend is the minimum immutable-object and conditional-pointer API
// required by Object. Cloud adapters should translate ETags or generations to
// Version and conditional writes to ObjectPutCondition.
type ObjectBackend interface {
	Get(context.Context, string) (ObjectValue, error)
	Put(context.Context, string, []byte, ObjectPutCondition) (ObjectValue, error)
}

// MemoryObjectBackend is a deterministic reference object backend.
type MemoryObjectBackend struct {
	mu      sync.RWMutex
	next    uint64
	objects map[string]ObjectValue
}

// NewMemoryObjectBackend creates an empty reference object backend.
func NewMemoryObjectBackend() *MemoryObjectBackend {
	return &MemoryObjectBackend{objects: make(map[string]ObjectValue)}
}

// Get returns a defensive copy of key.
func (b *MemoryObjectBackend) Get(ctx context.Context, key string) (ObjectValue, error) {
	if err := ctx.Err(); err != nil {
		return ObjectValue{}, err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	value, found := b.objects[key]
	if !found {
		return ObjectValue{}, &errors.NotFoundError{Resource: "object", ID: key}
	}
	value.Data = append([]byte(nil), value.Data...)
	return value, nil
}

// Put atomically writes key when condition matches.
func (b *MemoryObjectBackend) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	if err := ctx.Err(); err != nil {
		return ObjectValue{}, err
	}
	if strings.TrimSpace(key) == "" {
		return ObjectValue{}, &errors.ValidationError{Field: "object.key", Message: "is required"}
	}
	if condition.IfAbsent && condition.IfVersion != "" {
		return ObjectValue{}, &errors.ValidationError{Field: "object.condition", Message: "cannot combine IfAbsent and IfVersion"}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	existing, found := b.objects[key]
	if condition.IfAbsent && found {
		return ObjectValue{}, &errors.ConflictError{Resource: "object", Expected: "", Actual: existing.Version}
	}
	if condition.IfVersion != "" && (!found || existing.Version != condition.IfVersion) {
		actual := ""
		if found {
			actual = existing.Version
		}
		return ObjectValue{}, &errors.ConflictError{Resource: "object", Expected: condition.IfVersion, Actual: actual}
	}
	b.next++
	value := ObjectValue{Data: append([]byte(nil), data...), Version: fmt.Sprintf("%d", b.next)}
	b.objects[key] = value
	value.Data = append([]byte(nil), value.Data...)
	return value, nil
}

// Object stores immutable generations in an object namespace and promotes a
// versioned current pointer with a conditional write.
type Object struct {
	backend ObjectBackend
	prefix  string
}

// NewObject creates an object-backed catalog store.
func NewObject(backend ObjectBackend, prefix string) (*Object, error) {
	if backend == nil {
		return nil, &errors.ConfigError{Component: "catalog store", Message: "object backend is required"}
	}
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return nil, &errors.ConfigError{Component: "catalog store", Message: "object prefix is required"}
	}
	return &Object{backend: backend, prefix: prefix}, nil
}

// Current returns the currently active generation.
func (s *Object) Current(ctx context.Context) (Generation, error) {
	state, err := s.current(ctx)
	if err != nil {
		return Generation{}, err
	}
	if !state.exists {
		return Generation{}, currentNotFound()
	}
	return s.Get(ctx, state.id)
}

// Get returns an immutable generation by ID.
func (s *Object) Get(ctx context.Context, id string) (Generation, error) {
	manifestValue, err := s.backend.Get(ctx, s.generationKey(id, manifestFilename))
	if errors.IsNotFound(err) {
		return Generation{}, generationNotFound(id)
	}
	if err != nil {
		return Generation{}, err
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestValue.Data)
	if err != nil {
		return Generation{}, err
	}
	if manifest.GenerationID != id {
		return Generation{}, &errors.ValidationError{Field: "generation_id", Value: manifest.GenerationID, Message: "does not match requested generation"}
	}
	payloadValue, err := s.backend.Get(ctx, s.generationKey(id, payloadFilename))
	if err != nil {
		return Generation{}, err
	}
	generation := Generation{Manifest: manifest, Payload: append([]byte(nil), payloadValue.Data...)}
	if err := generation.Validate(); err != nil {
		return Generation{}, err
	}
	return generation, nil
}

// Commit uploads immutable generation objects before conditionally promoting
// the current pointer.
func (s *Object) Commit(ctx context.Context, generation Generation, expectedGenerationID string) error {
	if err := validateCandidate(ctx, generation); err != nil {
		return err
	}
	candidate := generation.Copy()
	id := candidate.Manifest.GenerationID

	existing, existingErr := s.Get(ctx, id)
	if existingErr == nil && !sameGeneration(existing, candidate) {
		return identityConflict(id)
	}
	if existingErr != nil && !errors.IsNotFound(existingErr) {
		return existingErr
	}
	state, err := s.current(ctx)
	if err != nil {
		return err
	}
	if existingErr == nil && state.exists && state.id == id {
		return nil
	}
	actual := ""
	if state.exists {
		actual = state.id
	}
	if actual != expectedGenerationID {
		return casConflict(expectedGenerationID, actual)
	}

	manifestData, err := marshalManifest(candidate.Manifest)
	if err != nil {
		return err
	}
	if err := s.putImmutable(ctx, s.generationKey(id, manifestFilename), manifestData); err != nil {
		return err
	}
	if err := s.putImmutable(ctx, s.generationKey(id, payloadFilename), candidate.Payload); err != nil {
		return err
	}
	pointerData, err := json.Marshal(struct {
		GenerationID string `json:"generation_id"`
	}{GenerationID: id})
	if err != nil {
		return &errors.ValidationError{Field: "current", Value: id, Message: fmt.Sprintf("cannot encode pointer: %v", err)}
	}
	condition := ObjectPutCondition{IfAbsent: !state.exists}
	if state.exists {
		condition.IfVersion = state.version
	}
	if _, err := s.backend.Put(ctx, s.currentKey(), pointerData, condition); err != nil {
		if errors.IsNotFound(err) || !isConflict(err) {
			return err
		}
		latest, latestErr := s.current(ctx)
		if latestErr == nil && latest.exists && latest.id == id {
			stored, getErr := s.Get(ctx, id)
			if getErr == nil && sameGeneration(stored, candidate) {
				return nil
			}
		}
		latestID := ""
		if latestErr == nil && latest.exists {
			latestID = latest.id
		}
		return casConflict(expectedGenerationID, latestID)
	}
	return nil
}

type objectCurrent struct {
	id      string
	version string
	exists  bool
}

func (s *Object) current(ctx context.Context) (objectCurrent, error) {
	value, err := s.backend.Get(ctx, s.currentKey())
	if errors.IsNotFound(err) {
		return objectCurrent{}, nil
	}
	if err != nil {
		return objectCurrent{}, err
	}
	var pointer struct {
		GenerationID string `json:"generation_id"`
	}
	if err := json.Unmarshal(value.Data, &pointer); err != nil {
		return objectCurrent{}, &errors.ValidationError{Field: "current", Value: string(value.Data), Message: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if strings.TrimSpace(pointer.GenerationID) == "" {
		return objectCurrent{}, &errors.ValidationError{Field: "current.generation_id", Message: "is required"}
	}
	return objectCurrent{id: pointer.GenerationID, version: value.Version, exists: true}, nil
}

func (s *Object) putImmutable(ctx context.Context, key string, data []byte) error {
	if _, err := s.backend.Put(ctx, key, data, ObjectPutCondition{IfAbsent: true}); err != nil {
		if !isConflict(err) {
			return err
		}
		existing, getErr := s.backend.Get(ctx, key)
		if getErr != nil {
			return getErr
		}
		if !bytes.Equal(existing.Data, data) {
			return &errors.ConflictError{Resource: "object", Expected: key, Actual: key, Message: "immutable object has different content"}
		}
	}
	return nil
}

func (s *Object) currentKey() string {
	return s.prefix + "/" + currentFilename + ".json"
}

func (s *Object) generationKey(id, filename string) string {
	digest := sha256.Sum256([]byte(id))
	return s.prefix + "/generations/" + hex.EncodeToString(digest[:]) + "/" + filename
}

func isConflict(err error) bool {
	return errors.IsConflict(err)
}

var (
	_ Store         = (*Object)(nil)
	_ ObjectBackend = (*MemoryObjectBackend)(nil)
)
