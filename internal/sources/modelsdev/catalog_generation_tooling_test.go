package modelsdev

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestCatalogGenerationToolingPromotesPayloadWithQuarantinedRecord(t *testing.T) {
	source := filepath.Join("..", "..", "embedded", "sources", "models.dev", "api.json")
	valid, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile source: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(valid, &payload); err != nil {
		t.Fatalf("Unmarshal source: %v", err)
	}
	baselineAPI, err := parseAPIData(valid)
	if err != nil {
		t.Fatalf("parseAPIData source: %v", err)
	}
	baselineStats, err := validateAPISemantics(baselineAPI)
	if err != nil {
		t.Fatalf("validateAPISemantics source: %v", err)
	}
	mutated := false
	for _, providerValue := range payload {
		provider, ok := providerValue.(map[string]any)
		if !ok {
			continue
		}
		models, ok := provider["models"].(map[string]any)
		if !ok {
			continue
		}
		for modelKey, modelValue := range models {
			model, ok := modelValue.(map[string]any)
			if !ok {
				continue
			}
			name, ok := model["name"].(string)
			id, idOK := model["id"].(string)
			if !ok || name == "" || strings.TrimSpace(name) != name || !idOK || id != modelKey || strings.TrimSpace(id) == "" {
				continue
			}
			model["name"] = name + "\t"
			mutated = true
			break
		}
		if mutated {
			break
		}
	}
	if !mutated {
		t.Fatal("fixture has no model name to mutate")
	}
	candidateData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal candidate: %v", err)
	}

	directory := t.TempDir()
	candidate := filepath.Join(directory, "candidate.json")
	destination := filepath.Join(directory, "api.json")
	if err := os.WriteFile(candidate, candidateData, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile candidate: %v", err)
	}
	if err := os.WriteFile(destination, valid, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile destination: %v", err)
	}

	promotion, err := PromoteAPIFile(candidate, destination)
	if err != nil {
		t.Fatalf("PromoteAPIFile: %v", err)
	}
	if promotion.RejectedModelCount != baselineStats.rejectedModels+1 || promotion.ModelCount != baselineStats.models-1 {
		t.Fatalf("promotion counts = %#v", promotion)
	}
	promoted, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile promoted: %v", err)
	}
	if !bytes.Equal(promoted, candidateData) {
		t.Fatal("promotion did not preserve exact source evidence bytes")
	}
}

func TestCatalogGenerationToolingTypedAtomicPromotion(t *testing.T) {
	source := filepath.Join("..", "..", "embedded", "sources", "models.dev", "api.json")
	valid, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile source: %v", err)
	}
	directory := t.TempDir()
	candidate := filepath.Join(directory, "candidate.json")
	destination := filepath.Join(directory, "api.json")
	if err := os.WriteFile(candidate, valid, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile candidate: %v", err)
	}
	if err := os.WriteFile(destination, valid, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile destination: %v", err)
	}

	promotion, err := PromoteAPIFile(candidate, destination)
	if err != nil {
		t.Fatalf("PromoteAPIFile: %v", err)
	}
	if promotion.Checksum != checksumBytes(valid) || promotion.SizeBytes != int64(len(valid)) ||
		promotion.ProviderCount < minimumModelsDevProviders || promotion.ModelCount < minimumModelsDevPromotionModels {
		t.Fatalf("promotion = %#v", promotion)
	}
	promoted, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile promoted: %v", err)
	}
	if !bytes.Equal(promoted, valid) {
		t.Fatal("promoted payload differs from validated candidate")
	}

	if err := os.WriteFile(candidate, []byte(`{"provider":null}`), constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile invalid candidate: %v", err)
	}
	_, err = PromoteAPIFile(candidate, destination)
	if err == nil {
		t.Fatal("PromoteAPIFile accepted invalid candidate")
	}
	var validationError *starmaperrors.ValidationError
	if !stderrors.As(err, &validationError) {
		t.Fatalf("error = %T %v, want typed ValidationError", err, err)
	}
	retained, readErr := os.ReadFile(destination)
	if readErr != nil {
		t.Fatalf("ReadFile retained: %v", readErr)
	}
	if !bytes.Equal(retained, valid) {
		t.Fatal("failed promotion changed last-known-good destination")
	}
}

func TestCatalogGenerationToolingRejectsHTTPErrorBeforePromotion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash catalog tooling is not used on Windows")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Fatal("curl is required by catalog generation tooling")
	}
	root := repositoryRoot(t)
	target := filepath.Join(t.TempDir(), "api.json")
	original := []byte("last-known-good")
	if err := os.WriteFile(target, original, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "upstream unavailable", http.StatusInternalServerError)
	}))
	defer server.Close()

	command := exec.Command("bash", filepath.Join(root, "scripts", "refresh-embedded-modelsdev.sh"))
	command.Dir = root
	command.Env = append(os.Environ(),
		"STARMAP_MODELS_DEV_URL="+server.URL,
		"STARMAP_EMBEDDED_MODELSDEV_PATH="+target,
	)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("HTTP 500 refresh succeeded: %s", output)
	}
	retained, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile retained: %v", err)
	}
	if !bytes.Equal(retained, original) {
		t.Fatalf("HTTP error changed destination: %q", retained)
	}
}

func TestCatalogGenerationToolingUsesCurrentCLIAndRealValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash catalog tooling is not used on Windows")
	}
	root := repositoryRoot(t)
	directory := t.TempDir()
	logPath := filepath.Join(directory, "commands.log")
	starmapSpy := filepath.Join(directory, "starmap-spy")
	spy := `#!/usr/bin/env bash
set -euo pipefail
	printf '[CATALOG_PATH=%s][CATALOG_EXPORT_PATH=%s]' "${CATALOG_PATH:-}" "${CATALOG_EXPORT_PATH:-}" >> "$STARMAP_COMMAND_LOG"
for argument in "$@"; do printf '[%s]' "$argument" >> "$STARMAP_COMMAND_LOG"; done
printf '\n' >> "$STARMAP_COMMAND_LOG"
`
	if err := os.WriteFile(starmapSpy, []byte(spy), constants.ExecutablePermissions); err != nil {
		t.Fatalf("WriteFile starmap spy: %v", err)
	}
	refreshSpy := filepath.Join(directory, "refresh-spy")
	if err := os.WriteFile(refreshSpy, []byte("#!/usr/bin/env bash\nset -euo pipefail\n"), constants.ExecutablePermissions); err != nil {
		t.Fatalf("WriteFile refresh spy: %v", err)
	}
	manifestLog := filepath.Join(directory, "manifest.log")
	manifestSpy := filepath.Join(directory, "manifest-spy")
	manifestScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" > \"$STARMAP_MANIFEST_LOG\"\nprintf '{\"changed\":false}\\n'\n"
	if err := os.WriteFile(manifestSpy, []byte(manifestScript), constants.ExecutablePermissions); err != nil {
		t.Fatalf("WriteFile manifest spy: %v", err)
	}
	catalogPath := filepath.Join(directory, "catalog")
	manifestPath := filepath.Join(catalogPath, "generation.json")
	reportPath := filepath.Join(directory, "generation-report.json")
	generationStatePath := filepath.Join(directory, "generation-state")
	command := exec.Command("bash", filepath.Join(root, "scripts", "generate-embedded-catalog.sh"), "openai")
	command.Dir = root
	command.Env = append(os.Environ(),
		"STARMAP_BIN="+starmapSpy,
		"STARMAP_COMMAND_LOG="+logPath,
		"STARMAP_MODELSDEV_REFRESH_BIN="+refreshSpy,
		"STARMAP_EMBEDDED_CATALOG_PATH="+catalogPath,
		"STARMAP_EMBEDDED_MANIFEST_PATH="+manifestPath,
		"STARMAP_GENERATION_REPORT_PATH="+reportPath,
		"STARMAP_GENERATION_STATE_PATH="+generationStatePath,
		"STARMAP_BOOTSTRAP_MANIFEST_BIN="+manifestSpy,
		"STARMAP_MANIFEST_LOG="+manifestLog,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generation command: %v\n%s", err, output)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile command log: %v", err)
	}
	want := "[CATALOG_PATH=" + filepath.Join(generationStatePath, "update-store") + "][CATALOG_EXPORT_PATH=]" +
		"[update][openai][--input-dir][" + catalogPath + "][--output-dir][" + catalogPath + "][-y]\n" +
		"[CATALOG_PATH=" + filepath.Join(generationStatePath, "validation-store") + "][CATALOG_EXPORT_PATH=" + catalogPath + "]" +
		"[validate][catalog]\n"
	if string(log) != want {
		t.Fatalf("commands = %q, want %q", log, want)
	}
	manifestInvocation, err := os.ReadFile(manifestLog)
	if err != nil {
		t.Fatalf("ReadFile manifest log: %v", err)
	}
	wantManifest := "--catalog-dir " + catalogPath + " --output " + manifestPath + "\n"
	if string(manifestInvocation) != wantManifest {
		t.Fatalf("manifest command = %q, want %q", manifestInvocation, wantManifest)
	}
	if report, err := os.ReadFile(reportPath); err != nil || string(report) != "{\"changed\":false}\n" {
		t.Fatalf("generation report = %q, %v", report, err)
	}
	for _, obsolete := range []string{"[--provider]", "[--output]", "[--force]", "[validate]\n"} {
		if strings.Contains(string(log), obsolete) {
			t.Errorf("command log contains obsolete/help-only invocation %q: %s", obsolete, log)
		}
	}
}

func TestCatalogGenerationToolingMakefileHasNoUnsafeSupersededRecipe(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join(repositoryRoot(t), "Makefile"))
	if err != nil {
		t.Fatalf("ReadFile Makefile: %v", err)
	}
	text := string(makefile)
	for _, forbidden := range []string{
		"curl -s https://models.dev/api.json -o internal/embedded",
		"run $(MAIN_PATH) validate\n",
		"update --provider $(PROVIDER) --output ",
		"update --output ",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("Makefile retains unsafe superseded catalog recipe %q", forbidden)
		}
	}
	for _, required := range []string{"catalog-generation-check:", "./scripts/generate-embedded-catalog.sh"} {
		if !strings.Contains(text, required) {
			t.Errorf("Makefile is missing %q", required)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	return root
}
