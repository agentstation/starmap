package starmap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// TestNewRejectsRuntimeOptions proves that the offline constructors accept no
// connected-runtime option. Every connected option carries the option type of
// github.com/agentstation/starmap/runtime, so New and NewContext reject it
// before the program builds. This test reads both package sources and fails
// when a connected-runtime option constructor returns to the offline option
// type.
func TestNewRejectsRuntimeOptions(t *testing.T) {
	offline := []string{
		"WithCatalogPath",
		"WithCatalogStore",
		"WithEmbeddedBootstrapMaxAge",
		"WithEmbeddedBootstrapMaxSizeBytes",
	}
	if got := optionConstructors(t, "."); !slices.Equal(got, offline) {
		t.Fatalf("offline option constructors = %v, want %v", got, offline)
	}

	connected := optionConstructors(t, "runtime")
	for _, name := range []string{
		"WithCatalogSource",
		"WithAcquisitionEnabled",
		"WithStartupSpread",
		"WithStateDirectory",
		"WithRefreshTimeout",
		"WithSourcePollInterval",
	} {
		if !slices.Contains(connected, name) {
			t.Errorf("the runtime package declares no %s option", name)
		}
	}

	if _, err := New(WithCatalogPath(t.TempDir())); err != nil {
		t.Fatalf("New rejected an offline option: %v", err)
	}
}

// optionConstructors returns the sorted names of the exported functions in one
// package directory that return the Option type of that package.
func optionConstructors(t *testing.T, directory string) []string {
	t.Helper()
	parsed, err := parser.ParseDir(token.NewFileSet(), directory, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", directory, err)
	}
	var names []string
	for _, pkg := range parsed {
		for path, file := range pkg.Files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			names = append(names, optionConstructorNames(file)...)
		}
	}
	slices.Sort(names)
	return names
}

// optionConstructorNames returns the exported functions in one file whose only
// result is the Option type of their own package.
func optionConstructorNames(file *ast.File) []string {
	var names []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || !function.Name.IsExported() {
			continue
		}
		results := function.Type.Results
		if results == nil || len(results.List) != 1 {
			continue
		}
		identifier, ok := results.List[0].Type.(*ast.Ident)
		if ok && identifier.Name == "Option" {
			names = append(names, function.Name.Name)
		}
	}
	return names
}
