// Package resourcepolicy owns resource limits and filesystem defaults for the
// public catalog tree.
package resourcepolicy

import (
	"io/fs"
	"time"

	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

const (
	// MaxModels bounds the canonical model collection and model-file loading.
	MaxModels = 10000
	// MaxProviders bounds the canonical provider collection.
	MaxProviders = 100
	// MaxPayloadBytes is the shared source and catalog JSON payload limit.
	MaxPayloadBytes = sourcepayload.MaxBytes
)

const (
	// DirMode is the mode for catalog-owned directories.
	DirMode fs.FileMode = 0o755
	// FileMode is the mode for catalog-owned data files.
	FileMode fs.FileMode = 0o644
	// SecureFileMode is the mode used by tests for protected fixture files.
	SecureFileMode fs.FileMode = 0o600
)

const (
	// DefaultHTTPTimeout bounds catalog distribution requests.
	DefaultHTTPTimeout = 30 * time.Second
	// StoreLockRetryDelay is the retry interval for a contended filesystem commit lock.
	StoreLockRetryDelay = 10 * time.Millisecond
)
