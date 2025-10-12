package update

import (
	"fmt"
	"os"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/appcontext"
	"github.com/agentstation/starmap/pkg/errors"
)

// LoadCatalog creates a starmap instance using app context.
// If inputPath is provided, creates a custom instance. Otherwise, uses app's default.
func LoadCatalog(appCtx appcontext.Interface, inputPath string, isQuiet bool) (starmap.Starmap, error) {
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
