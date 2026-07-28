package update

import (
	"fmt"
	"os"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/pkg/errors"
)

// LoadCatalog creates a starmap instance using app context.
// If catalogPath is provided, it is the single local read/write workspace.
// Otherwise, the application composition supplies its configured default.
func LoadCatalog(app application.Application, catalogPath string, isQuiet bool) (*starmap.Client, error) {
	var sm *starmap.Client
	var err error

	if catalogPath != "" {
		sm, err = app.Starmap(starmap.WithCatalogPath(catalogPath))
		if err != nil {
			return nil, errors.WrapResource("create", "starmap", "catalog workspace", err)
		}
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "📁 Using catalog workspace: %s\n", catalogPath)
		}
	} else {
		// Use app's default starmap (may be embedded or configured via app config)
		sm, err = app.Starmap()
		if err != nil {
			return nil, errors.WrapResource("get", "starmap", "", err)
		}
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "📦 Using configured catalog workspace\n")
		}
	}

	return sm, nil
}
