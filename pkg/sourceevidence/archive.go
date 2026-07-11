package sourceevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const maxEvidenceFileBytes = 64 << 20

// Archive durably retains minimized normalized evidence and encrypted raw evidence.
// Directory and file modes enforce owner-only access in addition to raw encryption.
type Archive struct {
	root   string
	key    []byte
	policy Policy
}

// NewArchive creates a passive evidence archive configuration.
func NewArchive(root string, key []byte, policy Policy) (*Archive, error) {
	if strings.TrimSpace(root) == "" {
		return nil, evidenceValidation("archive.root", root, "is required")
	}
	if len(key) != 32 {
		return nil, evidenceValidation("archive.key", len(key), "must be exactly 32 bytes")
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &Archive{root: root, key: append([]byte(nil), key...), policy: policy}, nil
}

// RetainNormalized atomically stores one validated long-term replay record.
func (a *Archive) RetainNormalized(record NormalizedRecord) error {
	if _, err := Replay(record); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return evidenceValidation("normalized_record", record.ObservationID, err.Error())
	}
	return a.write("normalized", record.ObservationID, data)
}

// LoadNormalized reads and validates one retained normalized record.
func (a *Archive) LoadNormalized(observationID string) (NormalizedRecord, error) {
	data, err := a.read("normalized", observationID)
	if err != nil {
		return NormalizedRecord{}, err
	}
	var record NormalizedRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return NormalizedRecord{}, evidenceValidation("normalized_record", observationID, err.Error())
	}
	if record.ObservationID != observationID {
		return NormalizedRecord{}, evidenceValidation("observation_id", record.ObservationID, "does not match archive key")
	}
	if _, err := Replay(record); err != nil {
		return NormalizedRecord{}, err
	}
	return record, nil
}

// ReplayObservation loads and deterministically replays a retained observation.
func (a *Archive) ReplayObservation(observationID string) (sources.Observation, error) {
	record, err := a.LoadNormalized(observationID)
	if err != nil {
		return sources.Observation{}, err
	}
	return Replay(record)
}

// RetainRaw encrypts and atomically stores one short-lived raw response body.
func (a *Archive) RetainRaw(observationID string, record RawRecord) error {
	if strings.TrimSpace(observationID) == "" {
		return evidenceValidation("observation_id", observationID, "is required")
	}
	sealed, err := sealRaw(a.key, observationID, record, record.ObservedAt.Add(a.policy.RawRetention))
	if err != nil {
		return err
	}
	data, err := json.Marshal(sealed)
	if err != nil {
		return evidenceValidation("sealed_raw", observationID, err.Error())
	}
	return a.write("raw", observationID, data)
}

// OpenRaw loads, authenticates, decrypts, and expiry-checks raw evidence.
func (a *Archive) OpenRaw(observationID string, now time.Time) (RawRecord, error) {
	data, err := a.read("raw", observationID)
	if err != nil {
		return RawRecord{}, err
	}
	var sealed SealedRawRecord
	if err := json.Unmarshal(data, &sealed); err != nil {
		return RawRecord{}, evidenceValidation("sealed_raw", observationID, err.Error())
	}
	if sealed.ObservationID != observationID {
		return RawRecord{}, evidenceValidation("observation_id", sealed.ObservationID, "does not match archive key")
	}
	return OpenRaw(a.key, sealed, now)
}

// PurgeExpiredRaw removes raw evidence whose bounded retention window has elapsed.
func (a *Archive) PurgeExpiredRaw(now time.Time) (int, error) {
	dir := filepath.Join(a.root, "raw")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, errors.WrapIO("read", dir, err)
	}
	purged := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, readErr := os.ReadFile(path) //nolint:gosec // entries are read from the configured archive directory.
		if readErr != nil {
			return purged, errors.WrapIO("read", path, readErr)
		}
		var sealed SealedRawRecord
		if unmarshalErr := json.Unmarshal(data, &sealed); unmarshalErr != nil {
			return purged, evidenceValidation("sealed_raw", entry.Name(), unmarshalErr.Error())
		}
		if now.Before(sealed.ExpiresAt) {
			continue
		}
		if removeErr := os.Remove(path); removeErr != nil {
			return purged, errors.WrapIO("remove", path, removeErr)
		}
		purged++
	}
	if purged > 0 {
		if err := syncDirectory(dir); err != nil {
			return purged, err
		}
	}
	return purged, nil
}

func (a *Archive) write(kind, observationID string, data []byte) error {
	if len(data) > maxEvidenceFileBytes {
		return evidenceValidation("archive.size", len(data), "exceeds 64 MiB limit")
	}
	dir := filepath.Join(a.root, kind)
	if err := os.MkdirAll(dir, constants.SecureDirPermissions); err != nil {
		return errors.WrapIO("create", dir, err)
	}
	if err := os.Chmod(a.root, constants.SecureDirPermissions); err != nil {
		return errors.WrapIO("chmod", a.root, err)
	}
	if err := os.Chmod(dir, constants.SecureDirPermissions); err != nil {
		return errors.WrapIO("chmod", dir, err)
	}
	temp, err := os.CreateTemp(dir, ".evidence-*")
	if err != nil {
		return errors.WrapIO("create", dir, err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(constants.SecureFilePermissions); err != nil {
		_ = temp.Close()
		return errors.WrapIO("chmod", tempPath, err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return errors.WrapIO("write", tempPath, err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return errors.WrapIO("sync", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return errors.WrapIO("close", tempPath, err)
	}
	destination := a.path(kind, observationID)
	if err := os.Rename(tempPath, destination); err != nil {
		return errors.WrapIO("rename", destination, err)
	}
	return syncDirectory(dir)
}

func (a *Archive) read(kind, observationID string) ([]byte, error) {
	path := a.path(kind, observationID)
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.WrapIO("stat", path, err)
	}
	if info.Size() > maxEvidenceFileBytes {
		return nil, evidenceValidation("archive.size", info.Size(), "exceeds 64 MiB limit")
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is content-addressed under configured archive root.
	if err != nil {
		return nil, errors.WrapIO("read", path, err)
	}
	return data, nil
}

func (a *Archive) path(kind, observationID string) string {
	digest := sha256.Sum256([]byte(observationID))
	return filepath.Join(a.root, kind, hex.EncodeToString(digest[:])+".json")
}

func syncDirectory(dir string) error {
	handle, err := os.Open(dir) //nolint:gosec // dir is derived from the configured archive root.
	if err != nil {
		return errors.WrapIO("open", dir, err)
	}
	if err := handle.Sync(); err != nil {
		_ = handle.Close()
		return errors.WrapIO("sync", dir, err)
	}
	if err := handle.Close(); err != nil {
		return errors.WrapIO("close", dir, err)
	}
	return nil
}
