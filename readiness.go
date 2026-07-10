package starmap

import (
	"fmt"
	"time"
)

const (
	// ReadinessIssueCatalogUnavailable means no active immutable catalog exists.
	ReadinessIssueCatalogUnavailable = "catalog_unavailable"
	// ReadinessIssueEmbeddedBootstrapFuture means embedded metadata is dated in the future.
	ReadinessIssueEmbeddedBootstrapFuture = "embedded_bootstrap_future"
	// ReadinessIssueEmbeddedBootstrapStale means the configured age budget was exceeded.
	ReadinessIssueEmbeddedBootstrapStale = "embedded_bootstrap_stale"
	// ReadinessIssueEmbeddedBootstrapOversize means the configured size budget was exceeded.
	ReadinessIssueEmbeddedBootstrapOversize = "embedded_bootstrap_oversize"
)

// ReadinessIssue is one stable machine-readable reason a client is not ready.
type ReadinessIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// EmbeddedBootstrapInfo reports the exact offline generation embedded in the
// binary and the budgets applied while it remains active.
type EmbeddedBootstrapInfo struct {
	Active            bool      `json:"active"`
	ManifestVersion   uint64    `json:"manifest_version"`
	GenerationID      string    `json:"generation_id"`
	GeneratedAt       time.Time `json:"generated_at"`
	AgeSeconds        int64     `json:"age_seconds"`
	SchemaVersion     uint64    `json:"schema_version"`
	PayloadChecksum   string    `json:"payload_checksum"`
	PayloadSizeBytes  int64     `json:"payload_size_bytes"`
	MaximumAgeSeconds int64     `json:"maximum_age_seconds,omitempty"`
	MaximumSizeBytes  int64     `json:"maximum_size_bytes,omitempty"`
}

// CatalogReadiness reports whether the current immutable catalog is safe to
// serve and includes embedded-bootstrap generation evidence.
type CatalogReadiness struct {
	Ready    bool                  `json:"ready"`
	Embedded EmbeddedBootstrapInfo `json:"embedded_bootstrap"`
	Issues   []ReadinessIssue      `json:"issues,omitempty"`
}

// Readiness evaluates catalog availability and configured embedded-bootstrap
// age/size budgets without performing I/O.
func (c *Client) Readiness() CatalogReadiness {
	if c == nil {
		return CatalogReadiness{Issues: []ReadinessIssue{{Code: ReadinessIssueCatalogUnavailable, Message: "client is nil"}}}
	}
	c.mu.RLock()
	catalogAvailable := c.catalog != nil
	active := c.usingEmbeddedBootstrap
	manifest := c.embeddedBootstrap
	c.mu.RUnlock()

	now := c.currentTime()
	age := now.Sub(manifest.GeneratedAt)
	info := EmbeddedBootstrapInfo{
		Active: active, ManifestVersion: manifest.ManifestVersion,
		GenerationID: manifest.GenerationID, GeneratedAt: manifest.GeneratedAt,
		AgeSeconds: int64(age / time.Second), SchemaVersion: manifest.SchemaVersion,
		PayloadChecksum: manifest.Payload.Checksum, PayloadSizeBytes: manifest.Payload.SizeBytes,
		MaximumSizeBytes: c.options.embeddedBootstrapMaxSizeBytes,
	}
	if c.options.embeddedBootstrapMaxAge > 0 {
		info.MaximumAgeSeconds = int64(c.options.embeddedBootstrapMaxAge / time.Second)
	}
	issues := make([]ReadinessIssue, 0, 3)
	if !catalogAvailable {
		issues = append(issues, ReadinessIssue{Code: ReadinessIssueCatalogUnavailable, Message: "catalog is not available"})
	}
	if active && age < 0 {
		issues = append(issues, ReadinessIssue{Code: ReadinessIssueEmbeddedBootstrapFuture, Message: "embedded bootstrap generation time is in the future"})
	}
	if active && c.options.embeddedBootstrapMaxAge > 0 && age > c.options.embeddedBootstrapMaxAge {
		issues = append(issues, ReadinessIssue{
			Code:    ReadinessIssueEmbeddedBootstrapStale,
			Message: fmt.Sprintf("embedded bootstrap age %s exceeds maximum %s", age.Round(time.Second), c.options.embeddedBootstrapMaxAge),
		})
	}
	if active && c.options.embeddedBootstrapMaxSizeBytes > 0 && manifest.Payload.SizeBytes > c.options.embeddedBootstrapMaxSizeBytes {
		issues = append(issues, ReadinessIssue{
			Code:    ReadinessIssueEmbeddedBootstrapOversize,
			Message: fmt.Sprintf("embedded bootstrap size %d exceeds maximum %d", manifest.Payload.SizeBytes, c.options.embeddedBootstrapMaxSizeBytes),
		})
	}
	return CatalogReadiness{Ready: len(issues) == 0, Embedded: info, Issues: issues}
}
