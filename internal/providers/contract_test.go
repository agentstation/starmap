package providers_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

type providerFixturePolicy struct {
	Providers []providerContract `yaml:"providers"`
}

type providerContract struct {
	ID       string `yaml:"id"`
	Role     string `yaml:"role"`
	Fixture  string `yaml:"fixture"`
	Reason   string `yaml:"reason"`
	Evidence string `yaml:"evidence"`
}

func TestProviderModuleAndFixtureContracts(t *testing.T) {
	data, err := os.ReadFile("fixture_policy.yaml")
	if err != nil {
		t.Fatalf("ReadFile policy: %v", err)
	}
	var policy providerFixturePolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		t.Fatalf("Unmarshal policy: %v", err)
	}
	contracts := make(map[string]providerContract, len(policy.Providers))
	for _, contract := range policy.Providers {
		if _, duplicate := contracts[contract.ID]; duplicate {
			t.Fatalf("duplicate provider policy %q", contract.ID)
		}
		contracts[contract.ID] = contract
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var providerDirs []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "clients" && entry.Name() != "testhelper" {
			providerDirs = append(providerDirs, entry.Name())
		}
	}
	sort.Strings(providerDirs)
	if len(contracts) != len(providerDirs) {
		t.Fatalf("policy/provider directory count = %d/%d", len(contracts), len(providerDirs))
	}
	for _, providerID := range providerDirs {
		t.Run(providerID, func(t *testing.T) {
			contract, found := contracts[providerID]
			if !found {
				t.Fatalf("provider directory %q has no fixture policy", providerID)
			}
			assertProviderRole(t, providerID, contract.Role)
			switch contract.Fixture {
			case "refreshable":
				for _, name := range []string{"models_list.json", "models_list.metadata.json"} {
					if _, err := os.Stat(filepath.Join(providerID, "testdata", name)); err != nil {
						t.Fatalf("refreshable fixture %s: %v", name, err)
					}
				}
			case "exception":
				if len(strings.Fields(contract.Reason)) < 8 || strings.TrimSpace(contract.Evidence) == "" {
					t.Fatalf("fixture exception lacks concrete reason/evidence: %#v", contract)
				}
				for _, evidence := range strings.Split(contract.Evidence, ",") {
					path := filepath.Join("..", "..", strings.TrimSpace(evidence))
					if _, err := os.Stat(path); err != nil {
						t.Fatalf("fixture exception evidence %q: %v", evidence, err)
					}
				}
			default:
				t.Fatalf("unsupported fixture policy %q", contract.Fixture)
			}
		})
	}
}

func TestProviderDocumentationCoversEveryRoleAndRefreshEntryPoint(t *testing.T) {
	for _, document := range []string{filepath.Join("..", "..", "docs", "ADDING_PROVIDERS.md"), filepath.Join("..", "..", "AGENTS.md")} {
		data, err := os.ReadFile(document)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", document, err)
		}
		text := string(data)
		for _, required := range []string{"YAML-only", "adapter", "client.go", "source.go", "pricing.go", "source_shape_test.go", "make testdata PROVIDER=", "provider-contract-check"} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s does not document %q", document, required)
			}
		}
		if strings.Contains(text, "go test ./internal/providers/<provider> -update") {
			t.Fatalf("%s promises the removed test-only refresh path", document)
		}
	}
}

func assertProviderRole(t *testing.T, directory, role string) {
	t.Helper()
	requirePair := func(production, test string) {
		if _, err := os.Stat(filepath.Join(directory, production)); err != nil {
			t.Fatalf("role %q requires %s", role, production)
		}
		if _, err := os.Stat(filepath.Join(directory, test)); err != nil {
			t.Fatalf("role %q requires %s", role, test)
		}
	}
	switch role {
	case "yaml_only":
		files, err := filepath.Glob(filepath.Join(directory, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if !strings.HasSuffix(file, "_test.go") {
				t.Fatalf("YAML-only provider has production Go %s", file)
			}
		}
		if _, err := os.Stat(filepath.Join(directory, "client_test.go")); err != nil {
			t.Fatal("YAML-only provider requires client_test.go")
		}
	case "adapter":
		requirePair("adapter.go", "adapter_test.go")
	case "native_client":
		requirePair("client.go", "client_test.go")
	case "regional_source":
		requirePair("source.go", "source_test.go")
	case "shared_transport":
		requirePair("client.go", "client_test.go")
	default:
		t.Fatalf("unsupported provider role %q", role)
	}
	if _, err := os.Stat(filepath.Join(directory, "source.go")); err == nil {
		if _, err := os.Stat(filepath.Join(directory, "source_test.go")); err != nil {
			t.Fatal("source.go requires source_test.go")
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "pricing.go")); err == nil {
		if _, err := os.Stat(filepath.Join(directory, "pricing_test.go")); err != nil {
			t.Fatal("pricing.go requires pricing_test.go")
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "source_shape_test.go")); err == nil {
		if _, err := os.Stat(filepath.Join(directory, "client_test.go")); err != nil {
			t.Fatal("source_shape_test.go supplements but cannot replace client_test.go")
		}
	}
}
