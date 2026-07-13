// Package catalogartifact defines the deterministic distribution format for
// immutable Starmap catalog generations.
package catalogartifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// FormatVersion is the current catalog distribution archive format.
	FormatVersion uint64 = 1
	// MediaType is the media type of the compressed catalog archive.
	MediaType = "application/vnd.agentstation.starmap.catalog-artifact.v1+tar+gzip"
	// DescriptorMediaType is the media type of artifact.json.
	DescriptorMediaType = "application/vnd.agentstation.starmap.catalog-artifact-descriptor.v1+json"
	// AttestationPredicateType identifies the detached in-toto predicate.
	AttestationPredicateType = "https://agentstation.ai/starmap/catalog-generation/v1"
	// AttestationStatementType is the in-toto statement schema identifier.
	AttestationStatementType = "https://in-toto.io/Statement/v1"
	// Filename is the stable archive filename; generation identity is carried by
	// the descriptor and distribution path rather than interpolated into a path.
	Filename = "starmap-catalog.tar.gz"
	// AttestationFilename is the detached in-toto statement filename.
	AttestationFilename = "starmap-catalog.intoto.json"
	// OCIMirrorArtifactType identifies an OCI manifest that mirrors the exact
	// immutable release assets. The catalog archive remains a layer with
	// MediaType, so its digest can be compared across distribution channels.
	OCIMirrorArtifactType = "application/vnd.agentstation.starmap.catalog-mirror.v1"
	// OCIGenerationAnnotation carries the logical catalog generation ID on an
	// OCI mirror manifest. Consumers must still pin and verify content digests.
	OCIGenerationAnnotation = "ai.agentstation.starmap.generation"

	descriptorFilename = "artifact.json"
	manifestFilename   = "manifest.json"
	payloadFilename    = "catalog.json"
	maxArtifactBytes   = 64 << 20
	fileMode           = 0o644
)

// FileDescriptor binds one named artifact member to exact bytes.
type FileDescriptor struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Checksum  string `json:"checksum"`
	SizeBytes int64  `json:"size_bytes"`
}

// Descriptor describes the complete logical generation carried by an archive.
type Descriptor struct {
	FormatVersion   uint64         `json:"format_version"`
	MediaType       string         `json:"media_type"`
	GenerationID    string         `json:"generation_id"`
	ManifestVersion uint64         `json:"manifest_version"`
	SchemaVersion   uint64         `json:"schema_version"`
	Manifest        FileDescriptor `json:"manifest"`
	Payload         FileDescriptor `json:"payload"`
}

// DigestSet is the SHA-256 digest map used by an in-toto subject.
type DigestSet struct {
	SHA256 string `json:"sha256"`
}

// Subject is one byte object bound by the detached statement.
type Subject struct {
	Name   string    `json:"name"`
	Digest DigestSet `json:"digest"`
}

// AttestationPredicate records the catalog compatibility identity asserted by
// the detached statement. Signature and builder provenance are added and
// verified by the publication boundary.
type AttestationPredicate struct {
	GenerationID    string `json:"generation_id"`
	ManifestVersion uint64 `json:"manifest_version"`
	SchemaVersion   uint64 `json:"schema_version"`
}

// AttestationStatement is the deterministic in-toto statement emitted beside
// an artifact. It is deliberately detached to avoid a self-referential archive
// digest and to permit signing without changing reproducible artifact bytes.
type AttestationStatement struct {
	Type          string               `json:"_type"`
	Subject       []Subject            `json:"subject"`
	PredicateType string               `json:"predicateType"`
	Predicate     AttestationPredicate `json:"predicate"`
}

// Artifact is one reproducible archive and its detached attestation statement.
type Artifact struct {
	GenerationID        string
	Filename            string
	MediaType           string
	Data                []byte
	Checksum            string
	AttestationFilename string
	Attestation         []byte
}

// Build validates a generation and deterministically packages it for
// distribution. Rebuilding identical generation bytes produces identical
// archive and attestation bytes.
func Build(generation catalogstore.Generation) (Artifact, error) {
	if err := generation.Validate(); err != nil {
		return Artifact{}, errors.WrapResource("validate", "catalog artifact generation", generation.Manifest.GenerationID, err)
	}
	if err := validateCanonicalPayload(generation.Payload); err != nil {
		return Artifact{}, errors.WrapResource("validate", "canonical catalog artifact payload", generation.Manifest.GenerationID, err)
	}
	manifest, err := json.Marshal(generation.Manifest)
	if err != nil {
		return Artifact{}, artifactValidation("manifest", generation.Manifest.GenerationID, err.Error())
	}
	descriptor := Descriptor{
		FormatVersion:   FormatVersion,
		MediaType:       MediaType,
		GenerationID:    generation.Manifest.GenerationID,
		ManifestVersion: generation.Manifest.ManifestVersion,
		SchemaVersion:   generation.Manifest.SchemaVersion,
		Manifest:        describeFile(manifestFilename, "application/json", manifest),
		Payload:         describeFile(payloadFilename, catalogs.CatalogPayloadMediaType, generation.Payload),
	}
	descriptorData, err := json.Marshal(descriptor)
	if err != nil {
		return Artifact{}, artifactValidation("descriptor", descriptor.GenerationID, err.Error())
	}
	archive, err := encodeArchive([]archiveMember{
		{name: descriptorFilename, data: descriptorData},
		{name: manifestFilename, data: manifest},
		{name: payloadFilename, data: generation.Payload},
	})
	if err != nil {
		return Artifact{}, err
	}
	archiveChecksum := checksum(archive)
	statement := AttestationStatement{
		Type: AttestationStatementType,
		Subject: []Subject{
			{Name: Filename, Digest: digestSet(archiveChecksum)},
			{Name: descriptorFilename, Digest: digestSet(checksum(descriptorData))},
			{Name: manifestFilename, Digest: digestSet(descriptor.Manifest.Checksum)},
			{Name: payloadFilename, Digest: digestSet(descriptor.Payload.Checksum)},
		},
		PredicateType: AttestationPredicateType,
		Predicate: AttestationPredicate{
			GenerationID: descriptor.GenerationID, ManifestVersion: descriptor.ManifestVersion,
			SchemaVersion: descriptor.SchemaVersion,
		},
	}
	attestation, err := json.Marshal(statement)
	if err != nil {
		return Artifact{}, artifactValidation("attestation", descriptor.GenerationID, err.Error())
	}
	return Artifact{
		GenerationID: descriptor.GenerationID, Filename: Filename, MediaType: MediaType, Data: archive, Checksum: archiveChecksum,
		AttestationFilename: AttestationFilename, Attestation: attestation,
	}, nil
}

// Open verifies an archive and detached statement before returning its exact
// immutable catalog generation.
func Open(archive, attestation []byte) (catalogstore.Generation, error) {
	if len(archive) > maxArtifactBytes {
		return catalogstore.Generation{}, artifactValidation("archive", len(archive), "exceeds maximum artifact size")
	}
	members, err := decodeArchive(archive)
	if err != nil {
		return catalogstore.Generation{}, err
	}
	descriptorData, ok := members[descriptorFilename]
	if !ok {
		return catalogstore.Generation{}, artifactValidation("archive", descriptorFilename, "required member is missing")
	}
	manifestData, ok := members[manifestFilename]
	if !ok {
		return catalogstore.Generation{}, artifactValidation("archive", manifestFilename, "required member is missing")
	}
	payload, ok := members[payloadFilename]
	if !ok {
		return catalogstore.Generation{}, artifactValidation("archive", payloadFilename, "required member is missing")
	}
	if len(members) != 3 {
		return catalogstore.Generation{}, artifactValidation("archive", len(members), "contains unsupported members")
	}

	var descriptor Descriptor
	if err := decodeStrictJSON(descriptorData, &descriptor); err != nil {
		return catalogstore.Generation{}, errors.WrapResource("parse", "catalog artifact descriptor", descriptorFilename, err)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		return catalogstore.Generation{}, err
	}
	generation := catalogstore.Generation{Manifest: manifest, Payload: append([]byte(nil), payload...)}
	if err := generation.Validate(); err != nil {
		return catalogstore.Generation{}, err
	}
	if err := validateCanonicalPayload(generation.Payload); err != nil {
		return catalogstore.Generation{}, errors.WrapResource("validate", "canonical catalog artifact payload", manifest.GenerationID, err)
	}
	if err := validateDescriptor(descriptor, generation, manifestData); err != nil {
		return catalogstore.Generation{}, err
	}
	if err := verifyAttestation(attestation, archive, descriptor, descriptorData); err != nil {
		return catalogstore.Generation{}, err
	}
	return generation, nil
}

type archiveMember struct {
	name string
	data []byte
}

func encodeArchive(members []archiveMember) ([]byte, error) {
	var output bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&output, gzip.BestCompression)
	if err != nil {
		return nil, artifactValidation("archive", nil, err.Error())
	}
	canonicalTime := time.Unix(0, 0).UTC()
	gzipWriter.ModTime = canonicalTime
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, member := range members {
		header := &tar.Header{
			Name: member.name, Mode: fileMode, Size: int64(len(member.data)),
			ModTime: canonicalTime, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, closeArchiveWriters(tarWriter, gzipWriter, err)
		}
		if _, err := tarWriter.Write(member.data); err != nil {
			return nil, closeArchiveWriters(tarWriter, gzipWriter, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, artifactValidation("archive", nil, err.Error())
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, artifactValidation("archive", nil, err.Error())
	}
	return output.Bytes(), nil
}

func closeArchiveWriters(tarWriter *tar.Writer, gzipWriter *gzip.Writer, cause error) error {
	_ = tarWriter.Close()
	_ = gzipWriter.Close()
	return artifactValidation("archive", nil, cause.Error())
}

func decodeArchive(data []byte) (map[string][]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, &errors.ParseError{Format: "tar+gzip", File: Filename, Message: err.Error(), Err: err}
	}
	defer func() { _ = gzipReader.Close() }()
	tarReader := tar.NewReader(io.LimitReader(gzipReader, maxArtifactBytes+1))
	members := make(map[string][]byte, 3)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &errors.ParseError{Format: "tar", File: Filename, Message: err.Error(), Err: err}
		}
		if header.Typeflag != tar.TypeReg {
			return nil, artifactValidation("archive.member", header.Name, "must be a regular file")
		}
		if _, exists := members[header.Name]; exists {
			return nil, artifactValidation("archive.member", header.Name, "is duplicated")
		}
		if header.Size < 0 || header.Size > maxArtifactBytes {
			return nil, artifactValidation("archive.member", header.Name, "exceeds maximum member size")
		}
		member, err := io.ReadAll(io.LimitReader(tarReader, maxArtifactBytes+1))
		if err != nil {
			return nil, &errors.ParseError{Format: "tar", File: header.Name, Message: err.Error(), Err: err}
		}
		if len(member) > maxArtifactBytes {
			return nil, artifactValidation("archive.member", header.Name, "exceeds maximum member size")
		}
		members[header.Name] = member
	}
	return members, nil
}

func validateDescriptor(descriptor Descriptor, generation catalogstore.Generation, manifestData []byte) error {
	manifest := generation.Manifest
	if descriptor.FormatVersion != FormatVersion || descriptor.MediaType != MediaType {
		return artifactValidation("descriptor.format", descriptor.FormatVersion, "is not supported")
	}
	if descriptor.GenerationID != manifest.GenerationID ||
		descriptor.ManifestVersion != manifest.ManifestVersion ||
		descriptor.SchemaVersion != manifest.SchemaVersion {
		return artifactValidation("descriptor.generation", descriptor.GenerationID, "does not match manifest identity")
	}
	wantManifest := describeFile(manifestFilename, "application/json", manifestData)
	wantPayload := describeFile(payloadFilename, catalogs.CatalogPayloadMediaType, generation.Payload)
	if descriptor.Manifest != wantManifest || descriptor.Payload != wantPayload {
		return artifactValidation("descriptor.files", descriptor.GenerationID, "does not match archive member bytes")
	}
	return nil
}

func verifyAttestation(data, archive []byte, descriptor Descriptor, descriptorData []byte) error {
	var statement AttestationStatement
	if err := decodeStrictJSON(data, &statement); err != nil {
		return errors.WrapResource("parse", "catalog artifact attestation", AttestationFilename, err)
	}
	if statement.Type != AttestationStatementType || statement.PredicateType != AttestationPredicateType {
		return artifactValidation("attestation.type", statement.PredicateType, "is not supported")
	}
	wantPredicate := AttestationPredicate{
		GenerationID: descriptor.GenerationID, ManifestVersion: descriptor.ManifestVersion,
		SchemaVersion: descriptor.SchemaVersion,
	}
	if statement.Predicate != wantPredicate {
		return artifactValidation("attestation.predicate", statement.Predicate.GenerationID, "does not match artifact descriptor")
	}
	want := []Subject{
		{Name: Filename, Digest: digestSet(checksum(archive))},
		{Name: descriptorFilename, Digest: digestSet(checksum(descriptorData))},
		{Name: manifestFilename, Digest: digestSet(descriptor.Manifest.Checksum)},
		{Name: payloadFilename, Digest: digestSet(descriptor.Payload.Checksum)},
	}
	sort.Slice(statement.Subject, func(i, j int) bool { return statement.Subject[i].Name < statement.Subject[j].Name })
	sort.Slice(want, func(i, j int) bool { return want[i].Name < want[j].Name })
	if len(statement.Subject) != len(want) {
		return artifactValidation("attestation.subject", len(statement.Subject), "has the wrong subject count")
	}
	for index := range want {
		if statement.Subject[index] != want[index] {
			return artifactValidation("attestation.subject", statement.Subject[index].Name, "digest does not match artifact bytes")
		}
	}
	return nil
}

func describeFile(name, mediaType string, data []byte) FileDescriptor {
	return FileDescriptor{Name: name, MediaType: mediaType, Checksum: checksum(data), SizeBytes: int64(len(data))}
}

func validateCanonicalPayload(data []byte) error {
	catalog, err := catalogstore.DecodeCatalogPayload(data)
	if err != nil {
		return err
	}
	canonical, err := catalogstore.EncodeCatalogPayload(catalog)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, canonical) {
		return artifactValidation("payload", checksum(data), "is valid but not canonical catalog JSON")
	}
	return nil
}

func checksum(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func digestSet(value string) DigestSet {
	return DigestSet{SHA256: value[len("sha256:"):]}
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return &errors.ParseError{Format: "json", File: "catalog artifact", Message: err.Error(), Err: err}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return &errors.ParseError{Format: "json", File: "catalog artifact", Message: "invalid trailing JSON", Err: err}
	}
	return nil
}

func artifactValidation(field string, value any, message string) error {
	return &errors.ValidationError{Field: "catalog_artifact." + field, Value: value, Message: message}
}

// String returns a concise descriptor useful in logs.
func (d Descriptor) String() string {
	return fmt.Sprintf("generation=%s schema=%d payload=%s", d.GenerationID, d.SchemaVersion, d.Payload.Checksum)
}
