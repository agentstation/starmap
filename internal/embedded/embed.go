package embedded

import (
	"embed"
)

// FS embeds the canonical provider-model YAML catalog, author metadata, logos,
// and external source data used for offline bootstrap.
//
//go:embed catalog sources
var FS embed.FS
