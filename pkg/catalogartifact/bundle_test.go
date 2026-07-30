package catalogartifact

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

const (
	wantArtifactFixtureChecksum    = "sha256:3409290cff99bf4fd304d8b68145315d2a02f5f0553f2813fb4a3f4f851d0aca"
	wantAttestationFixtureChecksum = "sha256:b1ff6893e1270f75cacbf54e89564dd2aa7ba4991dc050e8b61b933441666a73"
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
		if err := os.Mkdir(base, constants.DirPermissions); err != nil {
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

func artifactFixtureGeneration(t *testing.T) catalogstore.Generation {
	t.Helper()
	manifestData, err := os.ReadFile("../catalogs/testdata/generation/manifest.json")
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
	payload, err := catalogstore.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("Encode canonical payload: %v", err)
	}
	manifest.GenerationID = "artifact-fixture-generation-v1"
	manifest.Payload = catalogs.DescribeCatalogPayload(payload)
	generation := catalogstore.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		t.Fatalf("Validate generation fixture: %v", err)
	}
	return generation
}
