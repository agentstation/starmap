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
		"actions/checkout":                  "3d3c42e5aac5ba805825da76410c181273ba90b1", // v7.0.1
		"actions/setup-go":                  "b7ad1dad31e06c5925ef5d2fc7ad053ef454303e", // v7.0.0
		"actions/attest-build-provenance":   "0f67c3f4856b2e3261c31976d6725780e5e4c373", // v4.1.1
		"actions/upload-artifact":           "043fb46d1a93c77aae656e7c1c64a875d1fc6a0a", // v7.0.1
		"anchore/sbom-action/download-syft": "e22c389904149dbc22b58101806040fa8d37a610", // v0.24.0
		"docker/login-action":               "dbcb813823bdd20940b903addbd779551569679f", // v4.6.0
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

// requireSHAPinnedActions fails when the workflow does not reach every named
// action through a 40-character commit pin. Only
// TestActiveWorkflowsUseReviewedCurrentActions holds the reviewed commit
// values, so a version bump stays a single-file edit.
func requireSHAPinnedActions(t *testing.T, workflow, name string, actions ...string) {
	t.Helper()
	for _, action := range actions {
		pinned := regexp.MustCompile(`uses:\s+` + regexp.QuoteMeta(action) + `@[0-9a-f]{40}(?:\s|$)`)
		if !pinned.MatchString(workflow) {
			t.Errorf("%s workflow does not use %s through a 40-character commit pin", name, action)
		}
	}
}
