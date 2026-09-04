package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

// channelAssetMediaTypes names the media type of each published release asset.
var channelAssetMediaTypes = map[string]string{
	artifact.Filename:            artifact.MediaType,
	artifact.AttestationFilename: "application/vnd.in-toto+json",
	artifact.ChecksumFilename:    "text/plain",
}

// channelOptions describes one channel advance requested by the publisher.
type channelOptions struct {
	releaseDirectory    string
	tag                 string
	publishedAt         string
	updatedAt           string
	currentDocument     string
	outputPath          string
	attestationVerified bool
}

// channelReport records the staged channel document and its outcome.
type channelReport struct {
	Channel           string    `json:"channel"`
	Document          string    `json:"document"`
	Outcome           string    `json:"outcome"`
	CreatesGeneration bool      `json:"creates_generation"`
	SchemaVersion     uint64    `json:"schema_version"`
	Sequence          uint64    `json:"sequence"`
	ChannelUpdatedAt  time.Time `json:"channel_updated_at"`
	GenerationID      string    `json:"generation_id"`
	Tag               string    `json:"tag"`
	CatalogDigest     string    `json:"catalog_digest"`
	PublishedAt       time.Time `json:"published_at"`
}

// stageChannelDocument advances the stable channel over one verified immutable
// release and writes the canonical channel document. The release directory must
// hold every asset with matching checksums, and the caller must report that the
// release attestation verified. An unchanged catalog digest advances the
// sequence and channel_updated_at alone.
func stageChannelDocument(options channelOptions) (channelReport, error) {
	if strings.TrimSpace(options.outputPath) == "" {
		return channelReport{}, channelFlagError("channel_out", "is required")
	}
	verified, err := verifyReleaseDirectory(strings.TrimSpace(options.releaseDirectory))
	if err != nil {
		return channelReport{}, err
	}
	tag := strings.TrimSpace(options.tag)
	if tag == "" {
		expected, tagErr := artifact.ReleaseTag(verified.SemanticChecksum)
		if tagErr != nil {
			return channelReport{}, tagErr
		}
		tag = expected
	}
	publishedAt, err := channelTime("channel_published_at", options.publishedAt, time.Time{})
	if err != nil {
		return channelReport{}, err
	}
	updatedAt, err := channelTime("channel_updated_at", options.updatedAt, time.Now().UTC())
	if err != nil {
		return channelReport{}, err
	}
	assets, err := channelAssets(verified.Directory, verified.Files)
	if err != nil {
		return channelReport{}, err
	}
	current, err := readCurrentChannel(options.currentDocument)
	if err != nil {
		return channelReport{}, err
	}

	next, kind, err := current.Advance(artifact.Candidate{
		GenerationID:  verified.GenerationID,
		Tag:           tag,
		CatalogDigest: verified.SemanticChecksum,
		PublishedAt:   publishedAt,
		Assets:        assets,
		Verification: artifact.ReleaseVerification{
			AssetsPresent:       true,
			ChecksumsMatch:      true,
			AttestationVerified: options.attestationVerified,
		},
	}, updatedAt)
	if err != nil {
		return channelReport{}, err
	}
	document, err := artifact.EncodeChannel(next)
	if err != nil {
		return channelReport{}, err
	}
	path, err := writeChannelDocument(strings.TrimSpace(options.outputPath), document)
	if err != nil {
		return channelReport{}, err
	}
	return channelReport{
		Channel:           artifact.ChannelName,
		Document:          path,
		Outcome:           string(kind),
		CreatesGeneration: kind.CreatesGeneration(),
		SchemaVersion:     next.SchemaVersion,
		Sequence:          next.Sequence,
		ChannelUpdatedAt:  next.ChannelUpdatedAt,
		GenerationID:      next.GenerationID,
		Tag:               next.Tag,
		CatalogDigest:     next.CatalogDigest,
		PublishedAt:       next.PublishedAt,
	}, nil
}

// readCurrentChannel reads the channel document that the publisher read from
// the channel branch. A missing or empty path is the first publication.
func readCurrentChannel(path string) (artifact.Channel, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return artifact.Channel{}, nil
	}
	data, err := os.ReadFile(trimmed) //nolint:gosec // the publisher selects the channel document it read.
	if os.IsNotExist(err) {
		return artifact.Channel{}, nil
	}
	if err != nil {
		return artifact.Channel{}, pkgerrors.WrapIO("read", trimmed, err)
	}
	if len(data) == 0 {
		return artifact.Channel{}, nil
	}
	return artifact.DecodeChannel(data)
}

// channelAssets describes every published release asset with its checksum.
func channelAssets(directory string, files []string) ([]artifact.ChannelAsset, error) {
	assets := make([]artifact.ChannelAsset, 0, len(files))
	for _, file := range files {
		name := filepath.Base(file)
		mediaType, known := channelAssetMediaTypes[name]
		if !known {
			return nil, channelFlagError("channel_assets", "release asset "+name+" has no media type")
		}
		data, err := readReleaseAsset(file)
		if err != nil {
			return nil, err
		}
		assets = append(assets, artifact.ChannelAsset{
			Name:      name,
			MediaType: mediaType,
			Checksum:  fmt.Sprintf("%s%x", artifact.ChecksumPrefix, sha256.Sum256(data)),
			SizeBytes: int64(len(data)),
		})
	}
	if len(assets) == 0 {
		return nil, channelFlagError("channel_assets", "release directory "+directory+" has no assets")
	}
	return assets, nil
}

// channelTime parses one optional RFC 3339 publisher time.
func channelTime(field, value string, fallback time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if fallback.IsZero() {
			return time.Time{}, channelFlagError(field, "is required")
		}
		return fallback.UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, channelFlagError(field, "is not an RFC 3339 time")
	}
	return parsed.UTC(), nil
}

// writeChannelDocument writes the canonical document to its absolute path.
func writeChannelDocument(path string, document []byte) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", pkgerrors.WrapIO("resolve", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), constants.DirPermissions); err != nil {
		return "", pkgerrors.WrapIO("create", filepath.Dir(absolute), err)
	}
	if err := os.WriteFile(absolute, document, constants.FilePermissions); err != nil {
		return "", pkgerrors.WrapIO("write", absolute, err)
	}
	return absolute, nil
}

func channelFlagError(field, message string) error {
	return &pkgerrors.ValidationError{Field: "catalog_release." + field, Message: message}
}
