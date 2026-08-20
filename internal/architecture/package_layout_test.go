package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type approvedPackage struct {
	path string
	name string
}

func TestApprovedInternalPackageLayout(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	approved := []approvedPackage{
		{path: "internal/bootstrap/manifest", name: "manifest"},
		{path: "internal/bootstrap/budget", name: "budget"},
		{path: "internal/test/catalog", name: "catalog"},
		{path: "internal/test/logging", name: "logging"},
		{path: "internal/test/providerfixture", name: "providerfixture"},
	}
	removed := []string{
		filepath.Join("internal", "bootstrap"+"manifest"),
		filepath.Join("internal", "embedded"+"budget"),
		filepath.Join("internal", "source"+"payload"),
		filepath.Join("internal", "sources", "payload"),
		filepath.Join("internal", "test"+"catalog"),
		filepath.Join("internal", "test"+"logging"),
		filepath.Join("internal", "providers", "test"+"helper"),
		filepath.Join("internal", "providers", "cere"+"bras"),
		filepath.Join("internal", "providers", "deep"+"seek"),
		filepath.Join("internal", "providers", "gr"+"oq"),
		filepath.Join("internal", "providers", "moonshot"+"-ai"),
	}

	for _, path := range removed {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Errorf("removed package path %q exists or cannot be checked: %v", path, err)
		}
	}

	for _, pkg := range approved {
		assertPackageName(t, filepath.Join(root, pkg.path), pkg.name)
	}

	args := []string{"list", "-f", "{{.ImportPath}}\t{{.Name}}"}
	for _, pkg := range approved {
		args = append(args, "./"+pkg.path)
	}
	command := exec.Command("go", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list approved packages: %v\n%s", err, output)
	}
	listed := make(map[string]string, len(approved))
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			t.Fatalf("unexpected go list line %q", line)
		}
		listed[fields[0]] = fields[1]
	}
	for _, pkg := range approved {
		importPath := "github.com/agentstation/starmap/" + pkg.path
		if got := listed[importPath]; got != pkg.name {
			t.Errorf("package %q name = %q, want %q", importPath, got, pkg.name)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func assertPackageName(t *testing.T, dir, expected string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read approved package %q: %v", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly)
		if err != nil {
			t.Errorf("parse package declaration %q: %v", path, err)
			continue
		}
		if file.Name.Name != expected && file.Name.Name != expected+"_test" {
			t.Errorf("%s package = %q, want %q or %q", path, file.Name.Name, expected, expected+"_test")
		}
	}
}
