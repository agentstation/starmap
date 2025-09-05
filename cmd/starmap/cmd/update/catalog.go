package update

import (
	"fmt"
	"os"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// LoadCatalog creates a starmap instance with the appropriate catalog based on the input path.
// If inputPath is empty, uses the embedded catalog. Otherwise, loads from the specified directory.
func LoadCatalog(inputPath string, isQuiet bool) (starmap.Starmap, error) {
	var sm starmap.Starmap
	var err error

	if inputPath != "" {
		// Use file-based catalog from input directory
		filesCatalog, err := catalogs.New(catalogs.WithFiles(inputPath))
		if err != nil {
			return nil, errors.WrapResource("create", "catalog", inputPath, err)
		}
		sm, err = starmap.New(starmap.WithInitialCatalog(filesCatalog))
		if err != nil {
			return nil, errors.WrapResource("create", "starmap", "files catalog", err)
		}
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "📁 Using catalog from: %s\n", inputPath)
		}
	} else {
		// Use default starmap with embedded catalog
		sm, err = starmap.New()
		if err != nil {
			return nil, errors.WrapResource("create", "starmap", "", err)
		}
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "📦 Using embedded catalog\n")
		}
	}

	return sm, nil
}
