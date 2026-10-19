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
		"oven-sh/setup-bun":               "0c5077e51419868618aeaa5fe8019c62421857d6", // v2
		"actions/checkout":                "3d3c42e5aac5ba805825da76410c181273ba90b1", // v7.0.1
		"actions/setup-go":                "b7ad1dad31e06c5925ef5d2fc7ad053ef454303e", // v7.0.0
		"actions/attest-build-provenance": "4d101475d8b20a2381f78447822ac1eab6504dd8", // v4.2.2
		"actions/upload-artifact":         "cf430e030ddbb5b0abf93d22962f4752f3646cd9", // v7.0.2
		"actions/download-artifact":       "9000827ccba6bdab643e8b6fd33ac0654aef8333", // v8.0.2
		"actions/create-github-app-token": "64ab729a6294000f5aa4a4bc0b73d1f32bc45510", // v3.3.0

		"anchore/sbom-action/download-syft": "66cbf4bc1f1c0d2edc94016e65bc221b6bb0ad6c", // v0.24.3
		"docker/login-action":               "dbcb813823bdd20940b903addbd779551569679f", // v4.6.0
		"goreleaser/goreleaser-action":      "a4f02ab5355a589de60d917a659ce617ed19b3de", // v7.3.0
		"oras-project/setup-oras":           "005458ad77f1c8facd38a094e4af2e69e5607ff4", // v2.0.2
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
