package ciworkflow

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestActiveWorkflowsUseReviewedCurrentActions(t *testing.T) {
	t.Helper()
	approved := map[string]string{
		"actions/checkout":                  "9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0", // v7.0.0
		"actions/setup-go":                  "924ae3a1cded613372ab5595356fb5720e22ba16", // v6.5.0
		"actions/attest-build-provenance":   "0f67c3f4856b2e3261c31976d6725780e5e4c373", // v4.1.1
		"actions/upload-artifact":           "043fb46d1a93c77aae656e7c1c64a875d1fc6a0a", // v7.0.1
		"anchore/sbom-action/download-syft": "e22c389904149dbc22b58101806040fa8d37a610", // v0.24.0
		"docker/login-action":               "af1e73f918a031802d376d3c8bbc3fe56130a9b0", // v4.4.0
		"goreleaser/goreleaser-action":      "f06c13b6b1a9625abc9e6e439d9c05a8f2190e94", // v7.2.3
		"oras-project/setup-oras":           "1d808f7d7f6995cc68b7bf507bfe5c5446e1dc9d", // v2.0.1
	}
	seen := make(map[string]bool, len(approved))
	use := regexp.MustCompile(`uses:\s+([^@\s]+)@([0-9a-f]{40})(?:\s|$)`)
	workflows, err := filepath.Glob("../../.github/workflows/*.yaml")
	if err != nil {
		t.Fatalf("Glob active workflows: %v", err)
	}
	for _, path := range workflows {
		workflow := readFixture(t, path)
		for line := range strings.SplitSeq(workflow, "\n") {
			if !strings.Contains(line, "uses:") {
				continue
			}
			match := use.FindStringSubmatch(line)
			if len(match) != 3 {
				t.Errorf("%s contains a non-SHA-pinned action: %s", filepath.Base(path), strings.TrimSpace(line))
				continue
			}
			want, ok := approved[match[1]]
			if !ok {
				t.Errorf("%s uses unreviewed action %s", filepath.Base(path), match[1])
				continue
			}
			seen[match[1]] = true
			if match[2] != want {
				t.Errorf("%s action %s = %s, want reviewed current commit %s", filepath.Base(path), match[1], match[2], want)
			}
		}
	}
	for action := range approved {
		if !seen[action] {
			t.Errorf("reviewed action %s is not exercised by an active workflow", action)
		}
	}
}
