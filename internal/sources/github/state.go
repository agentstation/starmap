package github

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// StateSchemaVersion is the current durable state schema version.
	StateSchemaVersion uint64 = 1

	// stateDirectoryName holds every GitHub catalog source state file.
	stateDirectoryName = "github-catalog-source"

	// stateFileSuffix ends every state file name.
	stateFileSuffix = ".json"

	// maxStateBytes bounds one state file. The document holds a validator, a
	// sequence, and two short identity records.
	maxStateBytes = 1 << 16
)

// ReleaseRef names one verified immutable catalog release.
type ReleaseRef struct {
	Tag           string    `json:"tag"`
	GenerationID  string    `json:"generation_id"`
	CatalogDigest string    `json:"catalog_digest"`
	VerifiedAt    time.Time `json:"verified_at"`
}

// State is the durable discovery state of one repository channel. It carries
// the conditional-request validator, the replay floor, and the release that
// verification last accepted.
type State struct {
	SchemaVersion uint64     `json:"schema_version"`
	Repository    string     `json:"repository"`
	Channel       string     `json:"channel"`
	ChannelETag   string     `json:"channel_etag"`
	Sequence      uint64     `json:"sequence"`
	Verified      ReleaseRef `json:"verified"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Empty reports whether the reference names no release.
func (r ReleaseRef) Empty() bool {
	return r.Tag == ""
}

// stateStore reads and writes the durable state of one repository channel.
type stateStore struct {
	path string
}

// newStateStore resolves the state file of one configuration. The file name
// is a digest of the repository and the channel. A custom deployment therefore
// writes neither its host nor its URL into a path.
func newStateStore(config Config) (*stateStore, error) {
	directory := filepath.Join(config.StateDirectory, stateDirectoryName)
	if err := os.MkdirAll(directory, constants.DirPermissions); err != nil {
		return nil, errors.WrapIO("create", directory, err)
	}
	key := sha256.Sum256([]byte(config.Repository + "\x00" + config.Channel))
	return &stateStore{
		path: filepath.Join(directory, hex.EncodeToString(key[:])+stateFileSuffix),
	}, nil
}

// load reads the durable state. An absent file returns the zero state, so a
// cold instance starts with no validator and no sequence floor.
func (s *stateStore) load() (State, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, errors.WrapIO("read", s.path, err)
	}
	if len(data) > maxStateBytes {
		return State{}, sourceValidation("state", len(data), "exceeds the state document size limit")
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, errors.NewParseError("json", "catalog source state", "cannot decode the state", err)
	}
	if state.SchemaVersion != StateSchemaVersion {
		// A state file from another schema carries no usable floor. Start
		// cold rather than trust a document this build cannot read.
		return State{}, nil
	}
	return state, nil
}

// save writes the durable state through a temporary file and a rename, so a
// crash never leaves a partial document behind.
func (s *stateStore) save(state State) error {
	state.SchemaVersion = StateSchemaVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.WrapResource("encode", "catalog source state", state.Channel, err)
	}
	data = append(data, '\n')
	directory := filepath.Dir(s.path)
	file, err := os.CreateTemp(directory, ".state-")
	if err != nil {
		return errors.WrapIO("create", directory, err)
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return errors.WrapIO("write", temporary, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return errors.WrapIO("sync", temporary, err)
	}
	if err := file.Close(); err != nil {
		return errors.WrapIO("close", temporary, err)
	}
	if err := os.Chmod(temporary, constants.FilePermissions); err != nil {
		return errors.WrapIO("chmod", temporary, err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		return errors.WrapIO("publish", s.path, err)
	}
	return nil
}
