package productpaths_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agentstation/starmap/pkg/productpaths"
)

func TestNativeUserRootsSeparateProductsAndDirectoryRoles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	roaming, local := filepath.Join(home, "roaming"), filepath.Join(home, "local")
	t.Setenv("APPDATA", roaming)
	t.Setenv("LOCALAPPDATA", local)
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(name, "")
	}
	for _, product := range []productpaths.Product{productpaths.Starmap, productpaths.Starport} {
		roots, err := productpaths.Resolve(productpaths.UserDefaults(product))
		if err != nil {
			t.Fatal(err)
		}
		var expected map[productpaths.Root]string
		p := string(product)
		switch runtime.GOOS {
		case "linux":
			expected = map[productpaths.Root]string{productpaths.Config: filepath.Join(home, ".config", p), productpaths.Data: filepath.Join(home, ".local", "share", p), productpaths.State: filepath.Join(home, ".local", "state", p), productpaths.Cache: filepath.Join(home, ".cache", p)}
		case "darwin":
			base := filepath.Join(home, "Library", "Application Support", p)
			expected = map[productpaths.Root]string{productpaths.Config: filepath.Join(base, "config"), productpaths.Data: filepath.Join(base, "data"), productpaths.State: filepath.Join(base, "state"), productpaths.Cache: filepath.Join(home, "Library", "Caches", p)}
		case "windows":
			expected = map[productpaths.Root]string{productpaths.Config: filepath.Join(roaming, p), productpaths.Data: filepath.Join(local, p, "data"), productpaths.State: filepath.Join(local, p, "state"), productpaths.Cache: filepath.Join(local, p, "cache")}
		default:
			t.Fatal("native default qualification requires a supported platform")
		}
		for root, path := range expected {
			if roots[root].Path != path {
				t.Fatalf("%s %s: got %q, want %q", p, root, roots[root].Path, path)
			}
		}
	}
}
