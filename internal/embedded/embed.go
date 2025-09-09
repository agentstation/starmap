package embedded

import (
	"embed"
)

// FS embeds all catalog yaml files, logos, and external source data at build time.
// This includes model definitions under authors and providers, as well as fallback data like models.dev api.json.
//
//go:embed catalog sources
var FS embed.FS
