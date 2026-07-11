package catalogartifact

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestPublicationWorkflowsPinEveryExternalActionByCommit(t *testing.T) {
	pinned := regexp.MustCompile(`^\s*(?:-\s*)?uses:\s+[^@\s]+@[0-9a-f]{40}(?:\s+#.*)?$`)
	for _, path := range []string{
		"../../.github/workflows/release.yaml",
		"../../.github/workflows/catalog-generation.yaml",
	} {
		file, err := os.Open(path) //nolint:gosec // fixed repository fixtures
		if err != nil {
			t.Fatalf("Open %s: %v", path, err)
		}
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			value := scanner.Text()
			if strings.Contains(value, "uses:") && !pinned.MatchString(value) {
				t.Errorf("%s:%d external action is not SHA-pinned: %s", path, line, strings.TrimSpace(value))
			}
		}
		if err := scanner.Err(); err != nil {
			t.Errorf("Scan %s: %v", path, err)
		}
		if err := file.Close(); err != nil {
			t.Errorf("Close %s: %v", path, err)
		}
	}
}

func TestArtifactAttestationWorkflowPinsRepositoryAndSignerWorkflow(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/release.yaml")
	if err != nil {
		t.Fatalf("Read release workflow: %v", err)
	}
	workflow := string(data)
	for _, required := range []string{
		"attestations: write", "id-token: write", "actions/attest-build-provenance@e8998f949152b193b063cb0ec769d69d929409be # v2",
		"subject-path:", "gh attestation verify", `--repo "$GITHUB_REPOSITORY"`,
		`--signer-workflow "$GITHUB_REPOSITORY/.github/workflows/release.yaml"`,
		"--deny-self-hosted-runners",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release attestation workflow is missing %q", required)
		}
	}
	if strings.Contains(workflow, "gh attestation verify \\\n            --owner") {
		t.Fatal("release verification uses owner-only identity instead of exact repository/workflow")
	}
}

func TestArtifactOCIMirrorWorkflowRequiresIdenticalArchiveDigest(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/release.yaml")
	if err != nil {
		t.Fatalf("Read release workflow: %v", err)
	}
	workflow := string(data)
	for _, required := range []string{
		"if: vars.STARMAP_CATALOG_OCI_MIRROR == 'true'", "oras-project/setup-oras@22ce207df3b08e061f537244349aac6ae1d214f6 # v1", "version: 1.3.2",
		`OCI_TAG=sha256-${ARCHIVE_DIGEST}`, `oras push "${OCI_REPOSITORY}:${OCI_TAG}"`,
		`--artifact-type "` + OCIMirrorArtifactType + `"`,
		`--annotation "` + OCIGenerationAnnotation + `=${GENERATION_ID}"`,
		`starmap-catalog.tar.gz:` + MediaType, `REFERENCE=$(jq -er .reference`,
		`oras pull "$REFERENCE"`, `test "$PULLED_ARCHIVE_CHECKSUM" = "$ARCHIVE_CHECKSUM"`,
		`sha256sum --check`, `cmp "$DIRECTORY/starmap-catalog.tar.gz"`,
		`cmp "$DIRECTORY/starmap-catalog.intoto.json"`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("optional OCI mirror workflow is missing %q", required)
		}
	}
	if strings.Contains(workflow, "starmap-catalog:latest") {
		t.Fatal("OCI mirror gives a mutable latest tag authority over the catalog digest")
	}
}
