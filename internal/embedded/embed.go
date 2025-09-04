package embedded

import (
	"embed"
)

// FS embeds all catalog yaml files and logos at build time, including model definitions under authors and providers.
//
//go:embed catalog/*
var FS embed.FS
