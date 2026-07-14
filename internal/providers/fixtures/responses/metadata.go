package responses

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
)

// DefaultMaxAge is the reviewed freshness budget for captured provider responses.
const DefaultMaxAge = 365 * 24 * time.Hour

// Metadata binds a captured provider response to source revision and freshness evidence.
type Metadata struct {
	Version        uint64                          `json:"version"`
	Provider       string                          `json:"provider"`
	Source         string                          `json:"source"`
	FetchedAt      time.Time                       `json:"fetched_at"`
	SourceRevision catalogmeta.ObservationRevision `json:"source_revision"`
	Payload        Payload                         `json:"payload"`
	MaxAge         string                          `json:"max_age"`
}

// Payload identifies the exact governed response bytes.
type Payload struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
}

// MetadataPath returns the adjacent metadata path for a captured response.
func MetadataPath(fixturePath string) string {
	return strings.TrimSuffix(fixturePath, filepath.Ext(fixturePath)) + ".metadata.json"
}

// Checksum returns the captured response content digest.
func Checksum(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}
