package storage

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultObjectWriteLifetime bounds how long an unfinished upload remains protected without another commit attempt.
	DefaultObjectWriteLifetime = 5 * time.Minute
	// MaxCoordinatedObjectBytes includes the payload, manifest, and ownership header.
	MaxCoordinatedObjectBytes = MaxFilesystemPayloadBytes + MaxFilesystemManifestBytes + (1 << 20)
	maxCoordinationBytes      = 8 << 20
	maxCoordinationAttempts   = 64
	coordinatedVersion        = 1
)

// CoordinationBackend supplies atomic conditional records and current primary reads.
// Its records require durable storage without eviction or expiration.
type CoordinationBackend interface {
	ObjectBackend
	CurrentObjectReader
}

// CoordinatedObjectConfig selects a separate object namespace and its coordination record.
// OwnerID identifies this process incarnation. Now defaults to the system clock.
// Clock errors can cause early writer refusal but cannot bypass publication fencing.
type CoordinatedObjectConfig struct {
	Prefix          string
	CoordinationKey string
	OwnerID         string
	WriteLifetime   time.Duration
	Now             func() time.Time
}

// CoordinatedObject publishes immutable objects through a separate atomic registry.
// Readers retain stored bytes until explicit release. Expired unfinished uploads
// lose publication rights when collection removes their registry reservation.
type CoordinatedObject struct {
	objects      ObjectCollectionBackend
	coordination CoordinationBackend
	config       CoordinatedObjectConfig
}

// NewCoordinatedObject validates local configuration without accessing storage.
// The first explicit commit initializes an empty namespace. Existing plain object
// stores require a separate namespace and explicit migration.
func NewCoordinatedObject(objects ObjectCollectionBackend, coordination CoordinationBackend, config CoordinatedObjectConfig) (*CoordinatedObject, error) {
	if objects == nil || coordination == nil {
		return nil, coordinatedInvalid("backends", "object storage and current coordination are required")
	}
	config.Prefix = strings.Trim(config.Prefix, "/")
	for _, value := range []string{config.Prefix, config.CoordinationKey, config.OwnerID} {
		if !coordinatedIdentity(value) {
			return nil, coordinatedInvalid("identity", "bounded nonempty namespace and owner identities are required")
		}
	}
	if config.WriteLifetime == 0 {
		config.WriteLifetime = DefaultObjectWriteLifetime
	}
	if config.WriteLifetime < 0 || config.WriteLifetime > 24*time.Hour {
		return nil, coordinatedInvalid("write_lifetime", "must be positive and at most one day")
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &CoordinatedObject{objects: objects, coordination: coordination, config: config}, nil
}

type coordinatedRegistry struct {
	Version     int                              `json:"version"`
	StoreID     string                           `json:"store_id"`
	Prefix      string                           `json:"prefix"`
	Ready       bool                             `json:"ready"`
	Current     string                           `json:"current"`
	Generations map[string]coordinatedGeneration `json:"generations"`
	Readers     map[string]coordinatedReader     `json:"readers"`
}

type coordinatedGeneration struct {
	UploadID       string          `json:"upload_id"`
	ManifestDigest string          `json:"manifest_digest"`
	PayloadDigest  string          `json:"payload_digest"`
	GeneratedAt    time.Time       `json:"generated_at"`
	Bytes          int64           `json:"bytes"`
	Committed      bool            `json:"committed"`
	ObjectVersion  string          `json:"object_version"`
	ObjectBytes    int64           `json:"object_bytes"`
	ExpiresAt      time.Time       `json:"expires_at"`
	Authority      json.RawMessage `json:"authority,omitempty"`
}

type coordinatedReader struct {
	UploadID string `json:"upload_id"`
	OwnerID  string `json:"owner_id"`
}

type coordinatedGuard struct {
	Version         int    `json:"coordinated_store"`
	StoreID         string `json:"store_id"`
	CoordinationKey string `json:"coordination_key"`
}

func (s *CoordinatedObject) guardKey() string           { return s.config.Prefix + "/current.json" }
func (s *CoordinatedObject) uploadKey(id string) string { return s.config.Prefix + "/uploads/" + id }

func (s *CoordinatedObject) readRegistry(ctx context.Context) (coordinatedRegistry, string, error) {
	value, err := s.coordination.GetCurrent(ctx, s.config.CoordinationKey)
	if err != nil {
		return coordinatedRegistry{}, "", err
	}
	var state coordinatedRegistry
	if err := decodeCoordinatedJSON(value.Data, &state, maxCoordinationBytes); err != nil {
		return state, "", err
	}
	if value.Version == "" || state.Version != coordinatedVersion || !coordinatedToken(state.StoreID) || state.Prefix != s.config.Prefix || state.Generations == nil || state.Readers == nil {
		return state, "", coordinatedInvalid("registry", "stored coordination identity or schema is invalid")
	}
	if err := state.validate(); err != nil {
		return state, "", err
	}
	return state, value.Version, nil
}

func (s *CoordinatedObject) load(ctx context.Context) (coordinatedRegistry, string, error) {
	state, version, err := s.readRegistry(ctx)
	if errors.IsNotFound(err) {
		if _, guardErr := s.objects.Get(ctx, s.guardKey()); guardErr == nil {
			return state, "", coordinatedInvalid("registry", "initialized object storage lost its coordination record")
		} else if !errors.IsNotFound(guardErr) {
			return state, "", guardErr
		}
		return state, "", currentNotFound()
	}
	if err != nil {
		return state, "", err
	}
	if !state.Ready {
		return state, "", &errors.ConflictError{Resource: "catalog coordination initialization", Message: "an explicit commit must finish initialization"}
	}
	if err := s.checkGuard(ctx, state.StoreID); err != nil {
		return state, "", err
	}
	return state, version, nil
}

func (s *CoordinatedObject) checkGuard(ctx context.Context, storeID string) error {
	value, err := s.objects.Get(ctx, s.guardKey())
	if err != nil {
		return err
	}
	var guard coordinatedGuard
	if err := decodeCoordinatedJSON(value.Data, &guard, catalogs.MaxCatalogAuthorityRecordBytes); err != nil {
		return err
	}
	if guard.Version != coordinatedVersion || guard.StoreID != storeID || guard.CoordinationKey != s.config.CoordinationKey {
		return coordinatedInvalid("guard", "object storage and coordination identity do not match")
	}
	return nil
}

func (s *CoordinatedObject) initialize(ctx context.Context) error {
	for range maxCoordinationAttempts {
		state, version, err := s.readRegistry(ctx)
		if errors.IsNotFound(err) {
			if _, guardErr := s.objects.Get(ctx, s.guardKey()); guardErr == nil {
				return coordinatedInvalid("registry", "existing object storage cannot initialize new coordination")
			} else if !errors.IsNotFound(guardErr) {
				return guardErr
			}
			state = coordinatedRegistry{Version: coordinatedVersion, StoreID: rand.Text(), Prefix: s.config.Prefix, Generations: map[string]coordinatedGeneration{}, Readers: map[string]coordinatedReader{}}
			if err := s.saveRegistry(ctx, state, ""); errors.IsConflict(err) {
				continue
			} else if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if state.Ready {
			return s.checkGuard(ctx, state.StoreID)
		}
		guard, err := json.Marshal(coordinatedGuard{Version: coordinatedVersion, StoreID: state.StoreID, CoordinationKey: s.config.CoordinationKey})
		if err != nil {
			return err
		}
		if _, err := s.objects.Put(ctx, s.guardKey(), guard, ObjectPutCondition{IfAbsent: true}); err != nil && !errors.IsConflict(err) {
			return err
		}
		if err := s.checkGuard(ctx, state.StoreID); err != nil {
			return err
		}
		state.Ready = true
		err = s.saveRegistry(ctx, state, version)
		if errors.IsConflict(err) {
			continue
		}
		return err
	}
	return coordinatedBusy()
}

func (s *CoordinatedObject) saveRegistry(ctx context.Context, state coordinatedRegistry, version string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := state.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if len(data) > maxCoordinationBytes {
		return coordinatedInvalid("registry_size", "coordination exceeds its bounded record capacity")
	}
	_, err = s.coordination.Put(ctx, s.config.CoordinationKey, data, ObjectPutCondition{IfAbsent: version == "", IfVersion: version})
	return err
}

func (r coordinatedRegistry) validate() error {
	if len(r.Generations)+len(r.Readers) > MaxRetentionScanEntries {
		return coordinatedInvalid("registry_count", "coordination exceeds its bounded entry capacity")
	}
	uploads := make(map[string]bool, len(r.Generations))
	for id, entry := range r.Generations {
		if !coordinatedIdentity(id) || !coordinatedToken(entry.UploadID) || uploads[entry.UploadID] || !validCoordinatedDigest(entry.ManifestDigest) || !validCoordinatedDigest(entry.PayloadDigest) || entry.GeneratedAt.IsZero() || entry.Bytes <= 0 || entry.Bytes > MaxCoordinatedObjectBytes || (!entry.Committed && entry.ExpiresAt.IsZero()) {
			return coordinatedInvalid("generation", "stored generation descriptor is invalid")
		}
		if entry.Committed && (!coordinatedIdentity(entry.ObjectVersion) || entry.ObjectBytes <= 0 || entry.ObjectBytes > MaxCoordinatedObjectBytes) {
			return coordinatedInvalid("object_acknowledgment", "committed generation lacks a bounded versioned object acknowledgment")
		}
		uploads[entry.UploadID] = true
		if len(entry.Authority) > 0 {
			if _, err := parseAuthorityRecord(entry.Authority, id); err != nil {
				return err
			}
		}
	}
	if r.Current != "" && !r.Generations[r.Current].Committed {
		return coordinatedInvalid("current", "current must select a committed generation")
	}
	for token, reader := range r.Readers {
		if !coordinatedToken(token) || !coordinatedIdentity(reader.OwnerID) || !uploads[reader.UploadID] {
			return coordinatedInvalid("reader", "stored reader protection is invalid")
		}
	}
	if !r.Ready && (r.Current != "" || len(r.Generations)+len(r.Readers) != 0) {
		return coordinatedInvalid("initialization", "uninitialized storage contains publication state")
	}
	return nil
}

func coordinatedIdentity(value string) bool {
	return value != "" && len(value) <= 4096 && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}
func coordinatedToken(value string) bool {
	return len(value) >= 20 && len(value) <= 128 && strings.IndexFunc(value, func(r rune) bool { return (r < 'A' || r > 'Z') && (r < '2' || r > '7') }) < 0
}
func coordinatedInvalid(field, message string) error {
	return &errors.ValidationError{Field: "catalog_coordination." + field, Message: message}
}
func coordinatedBusy() error {
	return &errors.ConflictError{Resource: "catalog coordination", Message: "concurrent changes exhausted the retry bound"}
}
