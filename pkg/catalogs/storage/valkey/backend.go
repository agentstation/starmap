// Package valkey stores catalog coordination records in Valkey or Redis.
// The caller owns the client, its credentials, transport, and lifecycle.
// Records require persistent primary storage without eviction or expiration.
package valkey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	client "github.com/valkey-io/valkey-go"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultMaxObjectBytes bounds a coordination record when Config leaves the limit unset.
	DefaultMaxObjectBytes = 8 << 20
	// MaxObjectBytes is the largest supported coordination record.
	MaxObjectBytes   = 32 << 20
	maxIdentityBytes = 4096
	maxVersionBytes  = 128
)

// Config selects an isolated namespace and a bounded record size.
type Config struct {
	Prefix         string
	MaxObjectBytes int64
}

// Backend provides atomic conditional records and primary reads.
// It does not expose expiration, deletion, or advisory cache operations.
type Backend struct {
	client  client.Client
	prefix  string
	maximum int64
}

// New validates configuration without accessing the client or network.
func New(connection client.Client, config Config) (*Backend, error) {
	if connection == nil {
		return nil, invalid("client", "a caller-owned client is required")
	}
	if !identity(config.Prefix, maxIdentityBytes) {
		return nil, invalid("prefix", "a bounded nonempty namespace is required")
	}
	maximum := config.MaxObjectBytes
	if maximum == 0 {
		maximum = DefaultMaxObjectBytes
	}
	if maximum < 1 || maximum > MaxObjectBytes {
		return nil, invalid("max_object_bytes", "must be positive and at most 32 MiB")
	}
	return &Backend{client: connection, prefix: config.Prefix, maximum: maximum}, nil
}

// Get reads one complete record from the primary without client-side caching.
func (b *Backend) Get(ctx context.Context, key string) (storage.ObjectValue, error) {
	if !identity(key, maxIdentityBytes) {
		return storage.ObjectValue{}, invalid("key", "a bounded nonempty record key is required")
	}
	if err := ctx.Err(); err != nil {
		return storage.ObjectValue{}, err
	}
	command := b.client.B().Eval().Script(readRecordScript).Numkeys(1).Key(b.key(key)).Arg(strconv.FormatInt(b.maximum, 10)).Build()
	result, err := b.client.Do(ctx, command).AsStrSlice()
	return decodeResult(result, err, key)
}

// GetCurrent observes a complete primary record during the call.
// The script checks the server role without writing or renewing the record.
func (b *Backend) GetCurrent(ctx context.Context, key string) (storage.ObjectValue, error) {
	return b.Get(ctx, key)
}

// Put atomically replaces a record only when its exact condition holds.
// Every successful write receives a new opaque version, including identical bytes.
func (b *Backend) Put(ctx context.Context, key string, data []byte, condition storage.ObjectPutCondition) (storage.ObjectValue, error) {
	if !identity(key, maxIdentityBytes) {
		return storage.ObjectValue{}, invalid("key", "a bounded nonempty record key is required")
	}
	if condition.IfAbsent == (condition.IfVersion != "") || (condition.IfVersion != "" && !identity(condition.IfVersion, maxVersionBytes)) {
		return storage.ObjectValue{}, invalid("condition", "requires exactly one absence or version condition")
	}
	if int64(len(data)) > b.maximum {
		return storage.ObjectValue{}, invalid("object_bytes", "exceeds the configured record limit")
	}
	if err := ctx.Err(); err != nil {
		return storage.ObjectValue{}, err
	}
	version := rand.Text()
	command := b.client.B().Eval().Script(writeRecordScript).Numkeys(1).Key(b.key(key)).Arg(
		strconv.FormatInt(b.maximum, 10), condition.IfVersion, version, string(data),
	).Build()
	result, err := b.client.Do(ctx, command).AsStrSlice()
	value, err := decodeResult(result, err, key)
	if err != nil {
		if conflict, ok := err.(*errors.ConflictError); ok {
			conflict.Expected = condition.IfVersion
		}
		return storage.ObjectValue{}, err
	}
	if value.Version != version {
		return storage.ObjectValue{}, invalid("response", "write acknowledgment does not match its requested version")
	}
	value.Data = append([]byte(nil), data...)
	return value, nil
}

func (b *Backend) key(key string) string {
	digest := sha256.Sum256([]byte(key))
	return b.prefix + ":" + hex.EncodeToString(digest[:])
}

func decodeResult(result []string, err error, key string) (storage.ObjectValue, error) {
	if err != nil {
		return storage.ObjectValue{}, errors.WrapResource("access", "catalog coordination", key, err)
	}
	if len(result) == 0 {
		return storage.ObjectValue{}, invalid("response", "a record outcome is required")
	}
	switch result[0] {
	case "ok":
		if len(result) != 3 || !identity(result[1], maxVersionBytes) {
			return storage.ObjectValue{}, invalid("response", "a complete versioned record is required")
		}
		return storage.ObjectValue{Version: result[1], Data: []byte(result[2])}, nil
	case "missing":
		return storage.ObjectValue{}, &errors.NotFoundError{Resource: "catalog coordination", ID: key}
	case "conflict":
		if len(result) != 2 {
			return storage.ObjectValue{}, invalid("response", "a conflicting record version is required")
		}
		return storage.ObjectValue{}, &errors.ConflictError{Resource: "catalog coordination", Actual: result[1]}
	case "replica":
		return storage.ObjectValue{}, invalid("server_role", "coordination requires primary reads and writes")
	case "expiring":
		return storage.ObjectValue{}, invalid("record_expiration", "coordination records must not expire")
	case "oversized":
		return storage.ObjectValue{}, invalid("object_bytes", "stored record exceeds the configured limit")
	default:
		return storage.ObjectValue{}, invalid("record", "stored coordination metadata is invalid")
	}
}

func identity(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}

func invalid(field, message string) error {
	return &errors.ValidationError{Field: "catalog_coordination." + field, Message: message}
}

var (
	_ storage.ObjectBackend       = (*Backend)(nil)
	_ storage.CurrentObjectReader = (*Backend)(nil)
)
