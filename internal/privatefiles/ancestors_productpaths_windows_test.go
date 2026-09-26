package privatefiles_test

import (
	"os"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/productpaths"
)

func TestWindowsCanonicalAncestorsRemainPassive(t *testing.T) {
	for _, product := range []productpaths.Product{productpaths.Starmap, productpaths.Starport} {
		roots, err := productpaths.Resolve(productpaths.UserDefaults(product))
		if err != nil {
			t.Fatal(err)
		}
		for role, path := range roots {
			t.Run(string(product)+"/"+string(role), func(t *testing.T) {
				before, beforeErr := os.Lstat(path.Path)
				if beforeErr != nil && !os.IsNotExist(beforeErr) {
					t.Fatal(beforeErr)
				}
				if err := privatefiles.ValidateAncestors(path.Path); err != nil {
					t.Fatalf("native default %s: %v", path.Path, err)
				}
				after, afterErr := os.Lstat(path.Path)
				if os.IsNotExist(beforeErr) {
					if !os.IsNotExist(afterErr) {
						t.Fatalf("validation created %s: %v", path.Path, afterErr)
					}
				} else if afterErr != nil || !os.SameFile(before, after) {
					t.Fatalf("validation changed %s: %v", path.Path, afterErr)
				}
			})
		}
	}
}
