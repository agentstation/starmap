package consumer

import (
	"bytes"
	"encoding/json"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestActivatePinned(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	if err := ActivatePinned(t.Context()); err != nil {
		t.Fatalf("ActivatePinned: %v", err)
	}
}

func TestPinnedReleaseRejectsWrongTrustRoot(t *testing.T) {
	release, err := pinnedRelease()
	if err != nil {
		t.Fatal(err)
	}
	_, err = artifact.VerifyRelease(t.Context(), release, pinnedVerifier{digest: strings.Repeat("0", 64)})
	if err == nil || !strings.Contains(err.Error(), "pinned archive digest mismatch") {
		t.Fatalf("wrong trust root error = %v", err)
	}
}

func TestPinnedReleaseRejectsAlteredBytes(t *testing.T) {
	release, err := pinnedRelease()
	if err != nil {
		t.Fatal(err)
	}
	release.Archive[len(release.Archive)/2] ^= 1
	if _, err := artifact.VerifyRelease(t.Context(), release, pinnedVerifier{digest: pinnedArchiveSHA256}); err == nil {
		t.Fatal("altered archive passed verification")
	}
}

func TestPinnedFixtureMatchesInspectableSources(t *testing.T) {
	release, err := pinnedRelease()
	if err != nil {
		t.Fatal(err)
	}
	generation, err := artifact.VerifyRelease(t.Context(), release, pinnedVerifier{digest: pinnedArchiveSHA256})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile("testdata/fixture-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, generation.Payload) {
		t.Fatal("archive payload differs from fixture source")
	}
	manifestBytes, err := os.ReadFile("testdata/fixture-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest catalogs.GenerationManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manifest, generation.Manifest) {
		t.Fatal("archive manifest differs from fixture source")
	}
}
