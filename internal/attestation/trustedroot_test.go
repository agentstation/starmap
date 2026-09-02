package attestation

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/root"
)

func TestDefaultTrustedRootMatchesTheGitHubCLICapture(t *testing.T) {
	t.Parallel()

	captured, err := os.ReadFile(filepath.Join("testdata", "sigstore-public-good-trusted-root.json"))
	if err != nil {
		t.Fatalf("ReadFile capture: %v", err)
	}
	shipped := DefaultTrustedRootJSON()
	if !bytes.Equal(shipped, captured) {
		t.Fatalf("shipped trusted root is %d bytes, capture is %d bytes",
			len(shipped), len(captured))
	}
	if _, err := root.NewTrustedRootFromJSON(shipped); err != nil {
		t.Fatalf("NewTrustedRootFromJSON: %v", err)
	}
}

func TestDefaultTrustedRootReturnsACallerOwnedCopy(t *testing.T) {
	t.Parallel()

	first := DefaultTrustedRootJSON()
	if len(first) == 0 {
		t.Fatal("DefaultTrustedRootJSON returned no bytes")
	}
	first[0] = 'x'
	second := DefaultTrustedRootJSON()
	if second[0] == 'x' {
		t.Fatal("DefaultTrustedRootJSON shares its buffer with the caller")
	}
}
