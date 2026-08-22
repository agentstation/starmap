package artifact

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

const (
	wantArtifactFixtureChecksum    = "sha256:40343616f3dcabc3389d8e67d4460aff75e7e8c71c463a342e8bdfde8f72e349"
	wantAttestationFixtureChecksum = "sha256:2c49c77b6ae329e6bb30e9ef5f2ecdbf8aa2dedadca0b220bf2b5e79c00420e6"
)

func TestBundleReproducibleFixtureHashes(t *testing.T) {
	generation := artifactFixtureGeneration(t)
	first, err := Build(generation)
	if err != nil {
		t.Fatalf("Build first: %v", err)
	}
	second, err := Build(generation.Copy())
	if err != nil {
		t.Fatalf("Build second: %v", err)
	}
	if !bytes.Equal(first.Data, second.Data) || !bytes.Equal(first.Attestation, second.Attestation) {
		t.Fatal("identical generation inputs produced different artifact bytes")
	}
	if first.Checksum != wantArtifactFixtureChecksum {
		t.Fatalf("artifact checksum = %q, want %q", first.Checksum, wantArtifactFixtureChecksum)
	}
	if got := checksum(first.Attestation); got != wantAttestationFixtureChecksum {
		t.Fatalf("attestation checksum = %q, want %q", got, wantAttestationFixtureChecksum)
	}
	if first.Filename != Filename || first.MediaType != MediaType || first.AttestationFilename != AttestationFilename {
		t.Fatalf("artifact identity = %#v", first)
	}
}

func TestArtifactReleasePublicationIsImmutableAndIdempotent(t *testing.T) {
	generation := artifactFixtureGeneration(t)
	artifact, err := Build(generation)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	root := t.TempDir()
	first, err := StageReleaseAssets(root, artifact)
	if err != nil {
		t.Fatalf("StageReleaseAssets first: %v", err)
	}
	second, err := StageReleaseAssets(root, artifact)
	if err != nil {
		t.Fatalf("StageReleaseAssets retry: %v", err)
	}
	if first.Directory != second.Directory || len(first.Files) != 3 {
		t.Fatalf("release assets = %#v / %#v", first, second)
	}
	for _, path := range first.Files {
		info, statErr := os.Stat(path)
		if statErr != nil || info.Mode().Perm() != 0o644 {
			t.Fatalf("release asset %q: info=%#v err=%v", path, info, statErr)
		}
	}

	conflictingGeneration := generation.Copy()
	conflictingGeneration.Manifest.GeneratedAt = conflictingGeneration.Manifest.GeneratedAt.AddDate(0, 0, 1)
	conflicting, err := Build(conflictingGeneration)
	if err != nil {
		t.Fatalf("Build conflict: %v", err)
	}
	_, err = StageReleaseAssets(root, conflicting)
	var conflictErr *pkgerrors.ConflictError
	if !stderrors.As(err, &conflictErr) {
		t.Fatalf("conflicting publication error = %T %v, want ConflictError", err, err)
	}

	if err := os.WriteFile(filepath.Join(first.Directory, Filename), []byte("tampered"), 0o644); err != nil {
		t.Fatalf("tamper staged archive: %v", err)
	}
	_, err = StageReleaseAssets(root, artifact)
	if !stderrors.As(err, &conflictErr) {
		t.Fatalf("tampered retry error = %T %v, want ConflictError", err, err)
	}
}

func TestArtifactReleaseRejectsSymlinkedLifecyclePaths(t *testing.T) {
	artifact, err := Build(artifactFixtureGeneration(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	t.Run("root", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "release-root")
		if err := os.Symlink(t.TempDir(), root); err != nil {
			t.Fatalf("Symlink root: %v", err)
		}
		if _, err := StageReleaseAssets(root, artifact); !stderrors.Is(
			err,
			pkgerrors.ErrInvalidInput,
		) {
			t.Fatalf("StageReleaseAssets error = %T %v", err, err)
		}
	})

	t.Run("generation", func(t *testing.T) {
		root := t.TempDir()
		base := filepath.Join(root, releaseDirectory)
		if err := os.Mkdir(base, resourcepolicy.DirMode); err != nil {
			t.Fatalf("Mkdir base: %v", err)
		}
		target := filepath.Join(
			base,
			releaseDirectoryName(artifact.GenerationID),
		)
		if err := os.Symlink(t.TempDir(), target); err != nil {
			t.Fatalf("Symlink generation: %v", err)
		}
		if _, err := StageReleaseAssets(root, artifact); !stderrors.Is(
			err,
			pkgerrors.ErrInvalidInput,
		) {
			t.Fatalf("StageReleaseAssets error = %T %v", err, err)
		}
	})
}

func TestArtifactManifestPayloadCompatibilityAndAttestationRoundTrip(t *testing.T) {
	want := artifactFixtureGeneration(t)
	artifact, err := Build(want)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got, err := Open(artifact.Data, artifact.Attestation)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	wantManifest, _ := json.Marshal(want.Manifest)
	gotManifest, _ := json.Marshal(got.Manifest)
	if !bytes.Equal(got.Payload, want.Payload) || !bytes.Equal(gotManifest, wantManifest) {
		t.Fatal("opened artifact differs from exact generation input")
	}
	if got.Manifest.GenerationID != want.Manifest.GenerationID ||
		got.Manifest.SchemaVersion != want.Manifest.SchemaVersion ||
		got.Manifest.ConsumerCompatibility != want.Manifest.ConsumerCompatibility {
		t.Fatalf("generation identity/compatibility changed: %#v", got.Manifest)
	}
}

func TestInspectReportsPriorSchemaWithoutEnablingPayloadCompatibility(t *testing.T) {
	archive, attestation := priorSchemaEnvelope(t)
	descriptor, err := Inspect(archive, attestation)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if descriptor.SchemaVersion != 4 ||
		descriptor.ConsumerCompatibility.MinSchemaVersion != 2 ||
		descriptor.ConsumerCompatibility.MaxSchemaVersion != 4 ||
		descriptor.ConsumerCompatibility.SupportsSchema(catalogs.CurrentCatalogSchemaVersion) {
		t.Fatalf("descriptor compatibility = %#v", descriptor)
	}
	if _, err := Open(archive, attestation); err == nil {
		t.Fatal("Open accepted a prior-schema manifest")
	}
}

func TestArtifactRejectsArchiveOrAttestationTampering(t *testing.T) {
	artifact, err := Build(artifactFixtureGeneration(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	tamperedArchive := append([]byte(nil), artifact.Data...)
	tamperedArchive[len(tamperedArchive)/2] ^= 0xff
	if _, err := Open(tamperedArchive, artifact.Attestation); err == nil {
		t.Fatal("Open accepted a tampered archive")
	}
	if _, err := Inspect(tamperedArchive, artifact.Attestation); err == nil {
		t.Fatal("Inspect accepted a tampered archive")
	}

	var statement AttestationStatement
	if err := json.Unmarshal(artifact.Attestation, &statement); err != nil {
		t.Fatalf("Unmarshal attestation: %v", err)
	}
	statement.Subject[0].Digest.SHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tamperedAttestation, err := json.Marshal(statement)
	if err != nil {
		t.Fatalf("Marshal attestation: %v", err)
	}
	if _, err := Open(artifact.Data, tamperedAttestation); err == nil {
		t.Fatal("Open accepted a tampered attestation")
	}
	if _, err := Inspect(artifact.Data, tamperedAttestation); err == nil {
		t.Fatal("Inspect accepted a tampered attestation")
	}
}

func TestArtifactRejectsValidButNonCanonicalCatalogPayload(t *testing.T) {
	generation := artifactFixtureGeneration(t)
	var indented bytes.Buffer
	if err := json.Indent(&indented, generation.Payload, "", "  "); err != nil {
		t.Fatalf("Indent payload: %v", err)
	}
	generation.Payload = indented.Bytes()
	generation.Manifest.Payload = catalogs.DescribeCatalogPayload(generation.Payload)
	if _, err := Build(generation); err == nil {
		t.Fatal("Build accepted valid but non-canonical catalog JSON")
	}
}

func artifactFixtureGeneration(t *testing.T) catalogs.Generation {
	t.Helper()
	manifestData, err := os.ReadFile("../testdata/generation/manifest.json")
	if err != nil {
		t.Fatalf("Read manifest fixture: %v", err)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		t.Fatalf("Parse manifest fixture: %v", err)
	}
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build empty catalog: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("Encode canonical payload: %v", err)
	}
	manifest.GenerationID = "artifact-fixture-generation-v1"
	manifest.Payload = catalogs.DescribeCatalogPayload(payload)
	generation := catalogs.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		t.Fatalf("Validate generation fixture: %v", err)
	}
	return generation
}

func priorSchemaEnvelope(t *testing.T) ([]byte, []byte) {
	t.Helper()
	generation := artifactFixtureGeneration(t)
	manifestData, err := json.Marshal(generation.Manifest)
	if err != nil {
		t.Fatalf("Marshal manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Unmarshal manifest: %v", err)
	}
	delete(manifest, "review_candidates")
	manifest["manifest_version"] = float64(1)
	manifest["schema_version"] = float64(4)
	manifest["consumer_compatibility"] = map[string]any{
		"min_schema_version": float64(2),
		"max_schema_version": float64(4),
	}
	manifestData, err = json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal prior manifest: %v", err)
	}
	descriptor := Descriptor{
		FormatVersion: FormatVersion, MediaType: MediaType,
		GenerationID: generation.Manifest.GenerationID, ManifestVersion: 1, SchemaVersion: 4,
		ConsumerCompatibility: catalogs.ConsumerCompatibility{MinSchemaVersion: 2, MaxSchemaVersion: 4},
		Manifest:              describeFile(manifestFilename, "application/json", manifestData),
		Payload:               describeFile(payloadFilename, catalogs.CatalogPayloadMediaType, generation.Payload),
	}
	descriptorData, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("Marshal descriptor: %v", err)
	}
	archive, err := encodeArchive([]archiveMember{
		{name: descriptorFilename, data: descriptorData},
		{name: manifestFilename, data: manifestData},
		{name: payloadFilename, data: generation.Payload},
	})
	if err != nil {
		t.Fatalf("encodeArchive: %v", err)
	}
	statement := AttestationStatement{
		Type: AttestationStatementType,
		Subject: []Subject{
			{Name: Filename, Digest: digestSet(checksum(archive))},
			{Name: descriptorFilename, Digest: digestSet(checksum(descriptorData))},
			{Name: manifestFilename, Digest: digestSet(descriptor.Manifest.Checksum)},
			{Name: payloadFilename, Digest: digestSet(descriptor.Payload.Checksum)},
		},
		PredicateType: AttestationPredicateType,
		Predicate: AttestationPredicate{
			GenerationID: descriptor.GenerationID, ManifestVersion: descriptor.ManifestVersion,
			SchemaVersion: descriptor.SchemaVersion, ConsumerCompatibility: descriptor.ConsumerCompatibility,
		},
	}
	attestation, err := json.Marshal(statement)
	if err != nil {
		t.Fatalf("Marshal attestation: %v", err)
	}
	return archive, attestation
}
