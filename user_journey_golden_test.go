package starmap

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestP2UserJourneyGoldenFixtures(t *testing.T) {
	const directory = "testdata/journeys"
	goFixtures := map[string][]string{
		"in_process_library.go.golden": {
			"github.com/agentstation/starmap",
		},
		"embeddable_server.go.golden": {
			"github.com/agentstation/starmap",
			"github.com/agentstation/starmap/server",
		},
		"remote_reactive_consumer.go.golden": {
			"github.com/agentstation/starmap/remote",
		},
	}
	for name, requiredImports := range goFixtures {
		name, requiredImports := name, requiredImports
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(directory, name)
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			imports := make([]string, 0, len(parsed.Imports))
			for _, imported := range parsed.Imports {
				imports = append(imports, strings.Trim(imported.Path.Value, `"`))
			}
			for _, required := range requiredImports {
				if !slices.Contains(imports, required) {
					t.Fatalf("imports = %#v, missing %q", imports, required)
				}
			}
			for _, imported := range imports {
				if strings.Contains(imported, "/internal/") || strings.Contains(imported, "/cmd/") {
					t.Fatalf("consumer fixture imports non-public implementation %q", imported)
				}
			}
		})
	}

	for _, name := range []string{"cli_workspace.json", "embedded_upgrade.json"} {
		name := name
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			var fixture journeyFixture
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if fixture.ID == "" || len(fixture.Given) == 0 || len(fixture.When) == 0 ||
				len(fixture.Then) == 0 || len(fixture.FutureGates) == 0 {
				t.Fatalf("incomplete journey fixture: %#v", fixture)
			}
			wantForbidden := []string{"definitions", "offerings", "overrides"}
			if !slices.Equal(fixture.ForbiddenPaths, wantForbidden) {
				t.Fatalf("forbidden paths = %#v, want %#v", fixture.ForbiddenPaths, wantForbidden)
			}
		})
	}
}

type journeyFixture struct {
	ID             string   `json:"id"`
	Given          []string `json:"given"`
	When           []string `json:"when"`
	Then           []string `json:"then"`
	ForbiddenPaths []string `json:"forbidden_paths"`
	FutureGates    []string `json:"future_gates"`
}
