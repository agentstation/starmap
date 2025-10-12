package update

import (
	"fmt"
	"os"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/appcontext"
	"github.com/agentstation/starmap/pkg/errors"
)

// LoadCatalogWithApp creates a starmap instance using app context.
// If inputPath is provided, creates a custom instance. Otherwise, uses app's default.
func LoadCatalogWithApp(appCtx appcontext.Interface, inputPath string, isQuiet bool) (starmap.Starmap, error) {
	var sm starmap.Starmap
	var err error

	// If input path is provided, create custom starmap with that path
	if inputPath != "" {
		sm, err = appCtx.StarmapWithOptions(starmap.WithLocalPath(inputPath))
		if err != nil {
			return nil, errors.WrapResource("create", "starmap", "files catalog", err)
		}
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "📁 Using catalog from: %s\n", inputPath)
		}
	} else {
		// Use app's default starmap (may be embedded or configured via app config)
		sm, err = appCtx.Starmap()
		if err != nil {
			return nil, errors.WrapResource("get", "starmap", "", err)
		}
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "📦 Using default catalog\n")
		}
	}

	return sm, nil
}

// LoadCatalog creates a starmap instance with the appropriate catalog based on the input path.
// If inputPath is empty, uses the embedded catalog. Otherwise, loads from the specified directory.
//
// Deprecated: Use LoadCatalogWithApp for new code.
func LoadCatalog(inputPath string, isQuiet bool) (starmap.Starmap, error) {
	var sm starmap.Starmap
	var err error

	// If input path is provided, use it
	if inputPath != "" {
		// Use file-based catalog from input directory
		sm, err = starmap.New(starmap.WithLocalPath(inputPath))
		if err != nil {
			return nil, errors.WrapResource("create", "starmap", "files catalog", err)
		}
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "📁 Using catalog from: %s\n", inputPath)
		}
	} else {
		// Use default starmap with embedded catalog
		sm, err = starmap.New(starmap.WithEmbeddedCatalog())
		if err != nil {
			return nil, errors.WrapResource("create", "starmap", "", err)
		}
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "📦 Using embedded catalog\n")
		}
	}

	return sm, nil
}
