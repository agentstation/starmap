package ciworkflow

import (
	"strings"
	"testing"
)

func TestCatalogGenerationWorkflowUsesSixHourAcquisitionCredentialScope(t *testing.T) {
	workflow := readFixture(t, "../../.github/workflows/catalog-generation.yaml")
	if !strings.Contains(workflow, `- cron: "17 */6 * * *"`) {
		t.Error("catalog generation workflow does not request a six-hour schedule")
	}

	refreshStart := strings.Index(workflow, "- name: Refresh candidate catalog")
	refreshEnd := strings.Index(workflow, "- name: Classify catalog semantic change")
	if refreshStart < 0 || refreshEnd <= refreshStart {
		t.Fatal("catalog generation workflow does not contain the bounded refresh step")
	}
	refreshStep := workflow[refreshStart:refreshEnd]

	credentialNames := []string{
		"ANTHROPIC_API_KEY",
		"CEREBRAS_API_KEY",
		"DASHSCOPE_API_KEY",
		"DEEPSEEK_API_KEY",
		"FIREWORKS_API_KEY",
		"GOOGLE_API_KEY",
		"GROQ_API_KEY",
		"HETZNER_API_KEY",
		"MISTRAL_API_KEY",
		"MOONSHOT_API_KEY",
		"OPENAI_API_KEY",
	}
	for _, name := range credentialNames {
		reference := name + ": ${{ secrets." + name + " }}"
		if strings.Count(workflow, reference) != 1 {
			t.Errorf("catalog generation workflow reference count for %s is not one", name)
		}
		if !strings.Contains(refreshStep, reference) {
			t.Errorf("catalog generation workflow does not scope %s to the refresh step", name)
		}
	}

	for _, forbidden := range []string{
		"AZURE_OPENAI_API_KEY",
		"COHERE_API_KEY",
		"DEEPINFRA_API_KEY",
		"DEEPINFRA_TOKEN",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"VOYAGE_API_KEY",
	} {
		if strings.Contains(refreshStep, forbidden) {
			t.Errorf("catalog refresh step contains unavailable or inference-only credential %s", forbidden)
		}
	}
}
