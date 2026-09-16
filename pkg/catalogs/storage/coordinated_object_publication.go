package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type coordinatedObjectHeader struct {
	Version  int             `json:"version"`
	StoreID  string          `json:"store_id"`
	UploadID string          `json:"upload_id"`
	Manifest json.RawMessage `json:"manifest"`
}

func decodeCoordinatedJSON(data []byte, target any, maximum int) error {
	if len(data) == 0 || len(data) > maximum {
		return coordinatedInvalid("record_size", "stored record exceeds its format bounds")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return coordinatedInvalid("record", "stored record has invalid JSON or unknown fields")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return coordinatedInvalid("record", "stored record contains trailing data")
	}
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(data, canonical) {
		return coordinatedInvalid("record", "stored record is not in its canonical representation")
	}
	return nil
}

func coordinatedDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func coordinatedCandidate(ctx context.Context, candidate catalogs.Generation) (coordinatedGeneration, []byte, error) {
	manifest, err := prepareFilesystemManifest(ctx, candidate)
	if err != nil {
		return coordinatedGeneration{}, nil, err
	}
	authority, err := authorityRecordData(candidate)
	if err != nil {
		return coordinatedGeneration{}, nil, err
	}
	entry := coordinatedGeneration{ManifestDigest: coordinatedDigest(manifest), PayloadDigest: coordinatedDigest(candidate.Payload), GeneratedAt: candidate.Manifest.GeneratedAt, Bytes: int64(len(manifest)) + int64(len(candidate.Payload)), Authority: authority}
	return entry, manifest, nil
}

func sameCoordinatedCandidate(left, right coordinatedGeneration) bool {
	return left.ManifestDigest == right.ManifestDigest && left.PayloadDigest == right.PayloadDigest && left.Bytes == right.Bytes && left.GeneratedAt.Equal(right.GeneratedAt) && bytes.Equal(left.Authority, right.Authority)
}

func encodeCoordinatedObject(state coordinatedRegistry, entry coordinatedGeneration, manifest, payload []byte) ([]byte, error) {
	if len(manifest) > MaxFilesystemManifestBytes || len(payload) > MaxFilesystemPayloadBytes {
		return nil, coordinatedInvalid("object_size", "generation parts exceed their bounded format")
	}
	header, err := json.Marshal(coordinatedObjectHeader{Version: coordinatedVersion, StoreID: state.StoreID, UploadID: entry.UploadID, Manifest: manifest})
	if err != nil {
		return nil, err
	}
	if len(header) >= MaxCoordinatedObjectBytes || len(payload) > MaxCoordinatedObjectBytes-len(header)-1 {
		return nil, coordinatedInvalid("object_size", "generation object exceeds its bounded format")
	}
	data := make([]byte, 0, len(header)+1+len(payload))
	data = append(data, header...)
	data = append(data, '\n')
	data = append(data, payload...)
	return data, nil
}

func decodeCoordinatedObject(data []byte, storeID, uploadID string) (catalogs.Generation, error) {
	if len(data) > MaxCoordinatedObjectBytes {
		return catalogs.Generation{}, coordinatedInvalid("object_size", "stored generation object exceeds its bounded format")
	}
	end := bytes.IndexByte(data, '\n')
	if end < 0 {
		return catalogs.Generation{}, coordinatedInvalid("object_header", "stored generation lacks its ownership header")
	}
	var header coordinatedObjectHeader
	if err := decodeCoordinatedJSON(data[:end], &header, MaxFilesystemManifestBytes+(1<<20)); err != nil {
		return catalogs.Generation{}, err
	}
	if header.Version != coordinatedVersion || header.StoreID != storeID || header.UploadID != uploadID {
		return catalogs.Generation{}, coordinatedInvalid("object_identity", "stored generation belongs to a different publication")
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(header.Manifest)
	if err != nil {
		return catalogs.Generation{}, err
	}
	generation := catalogs.Generation{Manifest: manifest, Payload: bytes.Clone(data[end+1:])}
	if err := generation.Validate(); err != nil {
		return catalogs.Generation{}, err
	}
	return generation, nil
}

// Commit reserves one unique upload before writing bytes and atomically selects it after validation.
// A reservation retired by collection can never publish, even when its upload finishes later.
func (s *CoordinatedObject) Commit(ctx context.Context, generation catalogs.Generation, expectedGenerationID string) error {
	candidate := generation.Copy()
	descriptor, manifest, err := coordinatedCandidate(ctx, candidate)
	if err != nil {
		return err
	}
	if err := s.initialize(ctx); err != nil {
		return err
	}
	state, entry, err := s.reserve(ctx, candidate.Manifest.GenerationID, descriptor, expectedGenerationID)
	if err != nil {
		return err
	}
	if entry.Committed {
		stored, err := s.Get(ctx, candidate.Manifest.GenerationID)
		if err != nil {
			return err
		}
		if !sameGeneration(stored, candidate) {
			return identityConflict(candidate.Manifest.GenerationID)
		}
		return s.promote(ctx, candidate.Manifest.GenerationID, entry.UploadID, expectedGenerationID, entry.ObjectVersion, entry.ObjectBytes)
	}
	data, err := encodeCoordinatedObject(state, entry, manifest, candidate.Payload)
	if err != nil {
		return err
	}
	key := s.uploadKey(entry.UploadID)
	uploaded, err := s.objects.Put(ctx, key, data, ObjectPutCondition{IfAbsent: true})
	if err != nil {
		if !errors.IsConflict(err) {
			return err
		}
		existing, getErr := s.objects.Get(ctx, key)
		if getErr != nil {
			return getErr
		}
		if !bytes.Equal(data, existing.Data) {
			return identityConflict(candidate.Manifest.GenerationID)
		}
		uploaded = existing
	}
	return s.promote(ctx, candidate.Manifest.GenerationID, entry.UploadID, expectedGenerationID, uploaded.Version, int64(len(data)))
}

func (s *CoordinatedObject) reserve(ctx context.Context, id string, descriptor coordinatedGeneration, expected string) (coordinatedRegistry, coordinatedGeneration, error) {
	for range maxCoordinationAttempts {
		state, version, err := s.load(ctx)
		if err != nil {
			return state, coordinatedGeneration{}, err
		}
		entry, found := state.Generations[id]
		if found && !sameCoordinatedCandidate(entry, descriptor) {
			return state, entry, identityConflict(id)
		}
		if state.Current != expected && state.Current != id {
			return state, entry, casConflict(expected, state.Current)
		}
		if found && entry.Committed {
			return state, entry, nil
		}
		if !found {
			entry = descriptor
			entry.UploadID = rand.Text()
		}
		entry.ExpiresAt = s.config.Now().UTC().Add(s.config.WriteLifetime)
		state.Generations[id] = entry
		err = s.saveRegistry(ctx, state, version)
		if errors.IsConflict(err) {
			continue
		}
		return state, entry, err
	}
	return coordinatedRegistry{}, coordinatedGeneration{}, coordinatedBusy()
}

func (s *CoordinatedObject) promote(ctx context.Context, id, uploadID, expected, objectVersion string, objectBytes int64) error {
	if !coordinatedIdentity(objectVersion) || objectBytes <= 0 || objectBytes > MaxCoordinatedObjectBytes {
		return coordinatedInvalid("object_acknowledgment", "a bounded versioned object acknowledgment is required")
	}
	for range maxCoordinationAttempts {
		state, version, err := s.load(ctx)
		if err != nil {
			return err
		}
		entry, found := state.Generations[id]
		if !found || entry.UploadID != uploadID {
			return &errors.ConflictError{Resource: "catalog publication reservation", Message: "the upload no longer has publication rights"}
		}
		if state.Current == id && entry.Committed {
			return nil
		}
		if state.Current != expected {
			return casConflict(expected, state.Current)
		}
		entry.Committed = true
		entry.ObjectVersion, entry.ObjectBytes = objectVersion, objectBytes
		state.Generations[id] = entry
		state.Current = id
		err = s.saveRegistry(ctx, state, version)
		if errors.IsConflict(err) {
			continue
		}
		return err
	}
	return coordinatedBusy()
}

// CurrentAuthorityHead reads current registry metadata without loading catalog object bytes.
// The method creates no protection claim and never repairs state or renews a receipt.
func (s *CoordinatedObject) CurrentAuthorityHead(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
	state, _, err := s.load(ctx)
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	if state.Current == "" {
		return catalogs.CatalogAuthorityHead{}, currentNotFound()
	}
	if len(state.Generations[state.Current].Authority) == 0 {
		return catalogs.CatalogAuthorityHead{}, authorityNotFound(state.Current)
	}
	return parseAuthorityRecord(state.Generations[state.Current].Authority, state.Current)
}

func validCoordinatedDigest(value string) bool {
	return len(value) == 64 && strings.IndexFunc(value, func(r rune) bool { return (r < '0' || r > '9') && (r < 'a' || r > 'f') }) < 0
}
