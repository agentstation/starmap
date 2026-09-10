package consumer

import (
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
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
