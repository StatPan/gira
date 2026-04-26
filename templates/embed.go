package templates

import "embed"

// FS contains the built-in Gira templates used by the Go CLI.
//
//go:embed all:default
var FS embed.FS
