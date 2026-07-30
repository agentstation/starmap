package catalogartifact

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type recordingPublisherVerifier struct {
	err   error
	name  string
	bytes []byte
}

func (v *recordingPublisherVerifier) VerifyPublisher(
	_ context.Context,
	name string,
	data []byte,
) error {
	v.name = name
	v.bytes = append([]byte(nil), data...)
	if len(data) > 0 {
		data[0] ^= 0xff
	}
	return v.err
}

func TestVerifyReleaseRequiresChecksumStatementAndPublisher(t *testing.T) {
	t.Parallel()

	generation := artifactFixtureGeneration(t)
	bundle, err := Build(generation)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	release := Release{
		Archive:     append([]byte(nil), bundle.Data...),
		Checksum:    []byte(strings.TrimPrefix(bundle.Checksum, "sha256:") + "  " + Filename + "\n"),
		Attestation: append([]byte(nil), bundle.Attestation...),
	}
	originalArchive := append([]byte(nil), release.Archive...)
	verifier := &recordingPublisherVerifier{}
	got, err := VerifyRelease(context.Background(), release, verifier)
	if err != nil {
		t.Fatalf("VerifyRelease: %v", err)
	}
	if got.Manifest.GenerationID != generation.Manifest.GenerationID ||
		!bytes.Equal(got.Payload, generation.Payload) {
		t.Fatalf("verified generation = %#v, want exact input", got.Manifest)
	}
	if verifier.name != Filename || !bytes.Equal(verifier.bytes, originalArchive) {
		t.Fatalf("publisher subject = %q / %d bytes", verifier.name, len(verifier.bytes))
	}
	if !bytes.Equal(release.Archive, originalArchive) {
		t.Fatal("publisher verifier could mutate caller-owned archive bytes")
	}
}

func TestVerifyReleaseAcceptsExactlyStagedAssets(t *testing.T) {
	t.Parallel()

	generation := artifactFixtureGeneration(t)
	bundle, err := Build(generation)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assets, err := StageReleaseAssets(t.TempDir(), bundle)
	if err != nil {
		t.Fatalf("StageReleaseAssets: %v", err)
	}
	readAsset := func(name string) []byte {
		t.Helper()
		data, readErr := os.ReadFile(filepath.Join(assets.Directory, name))
		if readErr != nil {
			t.Fatalf("Read staged %s: %v", name, readErr)
		}
		return data
	}
	release := Release{
		Archive:     readAsset(Filename),
		Checksum:    readAsset(ChecksumFilename),
		Attestation: readAsset(AttestationFilename),
	}

	got, err := VerifyRelease(
		context.Background(),
		release,
		&recordingPublisherVerifier{},
	)
	if err != nil {
		t.Fatalf("VerifyRelease staged assets: %v", err)
	}
	if got.Manifest.GenerationID != generation.Manifest.GenerationID ||
		!bytes.Equal(got.Payload, generation.Payload) {
		t.Fatalf("verified staged generation = %#v, want exact input", got.Manifest)
	}
}

func TestVerifyReleaseFailsClosedBeforeReturningGeneration(t *testing.T) {
	t.Parallel()

	bundle, err := Build(artifactFixtureGeneration(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	valid := Release{
		Archive:     bundle.Data,
		Checksum:    []byte(strings.TrimPrefix(bundle.Checksum, "sha256:") + "  " + Filename + "\n"),
		Attestation: bundle.Attestation,
	}
	incompatibleGeneration := artifactFixtureGeneration(t)
	incompatibleGeneration.Manifest.SchemaVersion =
		catalogs.CurrentCatalogSchemaVersion + 1
	incompatibleGeneration.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{
		MinSchemaVersion: incompatibleGeneration.Manifest.SchemaVersion,
		MaxSchemaVersion: incompatibleGeneration.Manifest.SchemaVersion,
	}
	incompatibleBundle, err := Build(incompatibleGeneration)
	if err != nil {
		t.Fatalf("Build incompatible: %v", err)
	}
	incompatible := Release{
		Archive: incompatibleBundle.Data,
		Checksum: []byte(
			strings.TrimPrefix(incompatibleBundle.Checksum, "sha256:") +
				"  " + Filename + "\n",
		),
		Attestation: incompatibleBundle.Attestation,
	}
	publisherErr := stderrors.New("unexpected publisher")

	tests := []struct {
		name     string
		release  Release
		verifier PublisherVerifier
		want     error
		anyError bool
	}{
		{name: "missing verifier", release: valid, want: pkgerrors.ErrInvalidInput},
		{name: "typed nil verifier", release: valid, verifier: (*recordingPublisherVerifier)(nil), want: pkgerrors.ErrInvalidInput},
		{
			name: "wrong checksum",
			release: Release{
				Archive: valid.Archive, Checksum: []byte("0  " + Filename + "\n"),
				Attestation: valid.Attestation,
			},
			verifier: &recordingPublisherVerifier{},
			want:     pkgerrors.ErrInvalidInput,
		},
		{
			name: "wrong statement",
			release: Release{
				Archive: valid.Archive, Checksum: valid.Checksum,
				Attestation: []byte(`{"_type":"wrong"}`),
			},
			verifier: &recordingPublisherVerifier{},
			anyError: true,
		},
		{
			name:     "incompatible schema",
			release:  incompatible,
			verifier: &recordingPublisherVerifier{},
			want:     pkgerrors.ErrInvalidInput,
		},
		{
			name:     "publisher rejected",
			release:  valid,
			verifier: &recordingPublisherVerifier{err: publisherErr},
			want:     publisherErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generation, verifyErr := VerifyRelease(
				context.Background(),
				test.release,
				test.verifier,
			)
			if verifyErr == nil ||
				(!test.anyError && !stderrors.Is(verifyErr, test.want)) {
				t.Fatalf("VerifyRelease error = %T %v, want %v", verifyErr, verifyErr, test.want)
			}
			if generation.Manifest.GenerationID != "" || len(generation.Payload) != 0 {
				t.Fatalf("failed verification returned generation %#v", generation)
			}
		})
	}
}
