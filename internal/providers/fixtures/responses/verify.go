package responses

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/errors"
)

// Verify validates captured response identity, source revision, and freshness.
func Verify(fixturePath string, now time.Time) error {
	payload, err := os.ReadFile(fixturePath) //nolint:gosec // Operator-selected fixture path.
	if err != nil {
		return errors.WrapIO("read", fixturePath, err)
	}
	metadataData, err := os.ReadFile(MetadataPath(fixturePath)) //nolint:gosec // Adjacent controlled metadata.
	if err != nil {
		return errors.WrapIO("read", MetadataPath(fixturePath), err)
	}
	var metadata Metadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		return &errors.ValidationError{Field: "fixture.metadata", Value: MetadataPath(fixturePath), Message: errors.SafeSummary(err)}
	}
	if metadata.Version != 1 || strings.TrimSpace(metadata.Provider) == "" || strings.TrimSpace(metadata.Source) == "" {
		return &errors.ValidationError{Field: "fixture.metadata", Value: metadata.Version, Message: "version 1, provider, and source are required"}
	}
	expectedProvider := ProviderFromPath(fixturePath)
	expectedSource := SourceFromPath(fixturePath)
	if metadata.Provider != expectedProvider || metadata.Source != expectedSource || metadata.Payload.Path != filepath.Base(fixturePath) {
		return &errors.ValidationError{Field: "fixture.identity", Value: metadata.Provider + "/" + metadata.Source, Message: "must match provider/source directories and adjacent payload"}
	}
	checksum := Checksum(payload)
	if metadata.Payload.Checksum != checksum {
		return &errors.ValidationError{Field: "fixture.payload.checksum", Value: metadata.Payload.Checksum, Message: "does not match fixture bytes"}
	}
	if metadata.SourceRevision.Kind == "" || strings.TrimSpace(metadata.SourceRevision.Value) == "" {
		return &errors.ValidationError{Field: "fixture.source_revision", Value: metadata.SourceRevision, Message: "kind and value are required"}
	}
	switch metadata.SourceRevision.Kind {
	case catalogmeta.ObservationRevisionKindETag, catalogmeta.ObservationRevisionKindLastModified,
		catalogmeta.ObservationRevisionKindGitCommit, catalogmeta.ObservationRevisionKindSourceVersion,
		catalogmeta.ObservationRevisionKindContentDigest:
	default:
		return &errors.ValidationError{Field: "fixture.source_revision.kind", Value: metadata.SourceRevision.Kind, Message: "is not supported"}
	}
	if metadata.SourceRevision.Kind == catalogmeta.ObservationRevisionKindContentDigest && metadata.SourceRevision.Value != checksum {
		return &errors.ValidationError{Field: "fixture.source_revision", Value: metadata.SourceRevision, Message: "content revision must match fixture bytes"}
	}
	if metadata.FetchedAt.IsZero() || metadata.FetchedAt.After(now.Add(5*time.Minute)) {
		return &errors.ValidationError{Field: "fixture.fetched_at", Value: metadata.FetchedAt, Message: "must be a non-future capture time"}
	}
	maxAge, err := time.ParseDuration(metadata.MaxAge)
	if err != nil || maxAge <= 0 || now.Sub(metadata.FetchedAt) > maxAge {
		return &errors.ValidationError{Field: "fixture.fetched_at", Value: metadata.FetchedAt, Message: "fixture freshness policy is invalid or stale"}
	}
	return nil
}

// ProviderFromPath returns the provider identity encoded by the exact-current
// responses/<provider>/<source>/models_list.json layout.
func ProviderFromPath(fixturePath string) string {
	return filepath.Base(filepath.Dir(filepath.Dir(fixturePath)))
}

// SourceFromPath returns the logical source identity encoded by the governed path.
func SourceFromPath(fixturePath string) string {
	return filepath.Base(filepath.Dir(fixturePath))
}
