package ciworkflow

import (
	"strings"
	"testing"
)

// The hosted workflows check out the pinned technical-writing skill under
// .ci, and the release verification job then runs make release-check, which
// requires a clean tree. The ignore rule keeps that checkout out of the tree
// status.
func TestWorkspaceIgnoresHostedWorkflowCheckouts(t *testing.T) {
	ignore := readFixture(t, "../../.gitignore")
	if !strings.Contains(ignore, "\n/.ci/\n") {
		t.Error(".gitignore does not ignore the /.ci/ hosted workflow checkout directory")
	}
	for _, workflow := range []string{"pr.yaml", "release.yaml"} {
		content := readFixture(t, "../../.github/workflows/"+workflow)
		if !strings.Contains(content, "path: .ci/agentstation-skills") {
			t.Errorf("%s does not check out the skill under .ci", workflow)
		}
	}
}
