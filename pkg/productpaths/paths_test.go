package productpaths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/productpaths"
)

func TestRootSpecificityPresenceAndProvenance(t *testing.T) {
	base := t.TempDir()
	defaults := func(root productpaths.Root) (string, error) { return filepath.Join(base, "default", string(root)), nil }
	result, err := productpaths.Resolve(defaults,
		productpaths.Layer{Name: "environment", Values: map[productpaths.Root]string{productpaths.Home: filepath.Join(base, "group"), productpaths.Cache: filepath.Join(base, "fast-cache")}},
		productpaths.Layer{Name: "file", Values: map[productpaths.Root]string{productpaths.Data: filepath.Join(base, "durable")}},
	)
	if err != nil {
		t.Fatal(err)
	}
	for root, want := range map[productpaths.Root]struct{ path, origin string }{
		productpaths.Config: {filepath.Join(base, "group", "config"), "environment"},
		productpaths.Data:   {filepath.Join(base, "durable"), "file"},
		productpaths.State:  {filepath.Join(base, "group", "state"), "environment"},
		productpaths.Cache:  {filepath.Join(base, "fast-cache"), "environment"},
	} {
		if result[root].Path != want.path || result[root].Origin != want.origin {
			t.Fatalf("incorrect root selection for %s: %+v", root, result[root])
		}
	}
	files, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatal("root resolution created files")
	}
}

func TestExplicitRootsDoNotRequireAHomeDirectory(t *testing.T) {
	defaults := func(productpaths.Root) (string, error) {
		t.Fatal("fully explicit roots probed platform defaults")
		return "", nil
	}
	if _, err := productpaths.Resolve(defaults, productpaths.Layer{Name: "options", Values: map[productpaths.Root]string{productpaths.Home: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidRootSelectorsFail(t *testing.T) {
	for name, values := range map[string]map[productpaths.Root]string{
		"empty": {productpaths.Home: ""}, "relative": {productpaths.Data: "relative"}, "unknown": {productpaths.Root("dataa"): "value"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := productpaths.Resolve(nil, productpaths.Layer{Name: "environment", Values: values}); err == nil {
				t.Fatal("invalid root accepted")
			}
		})
	}
}

func TestLeafPathsUseConfigurationAnchor(t *testing.T) {
	config := productpaths.Path{Path: t.TempDir(), Origin: "environment"}
	first, err := productpaths.Leaf(config, "artifacts/catalog", "file")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	second, err := productpaths.Leaf(config, "artifacts/catalog", "file")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.Anchor != config.Path || first.Path != filepath.Join(config.Path, "artifacts", "catalog") {
		t.Fatal("relative leaf depends on the working directory")
	}
	if _, err := productpaths.Leaf(config, "", "file"); err == nil {
		t.Fatal("empty required leaf accepted")
	}
}

func TestInstanceIDsArePortableSingleComponents(t *testing.T) {
	for _, id := range []string{"default", "replica-2", "local_dev"} {
		if err := productpaths.ValidateInstanceID(id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"", "../escape", "a/b", "a\\b", "CON", "con", "aux", "com1", "lpt9", "trailing.", "mixedCase", "a:b"} {
		if err := productpaths.ValidateInstanceID(id); err == nil {
			t.Fatalf("unsafe instance ID accepted: %q", id)
		}
	}
}

func TestHigherRootValueReplacesInvalidLowerValue(t *testing.T) {
	base := t.TempDir()
	roots, err := productpaths.Resolve(nil,
		productpaths.Layer{Name: "flags", Values: map[productpaths.Root]string{productpaths.Home: base, productpaths.Data: filepath.Join(base, "valid")}},
		productpaths.Layer{Name: "environment", Values: map[productpaths.Root]string{productpaths.Data: "invalid-relative"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if roots[productpaths.Data].Path != filepath.Join(base, "valid") {
		t.Fatal("higher root did not replace invalid lower value")
	}
}
