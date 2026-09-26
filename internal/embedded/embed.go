package embedded

import (
	"embed"
)

// FS embeds the authored YAML catalog, generated bootstrap payload, metadata, logos,
// and external source data for offline operation.
//
//go:embed catalog sources
var FS embed.FS
