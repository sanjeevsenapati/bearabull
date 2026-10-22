package web

import "embed"

// Files contains the embedded web static files.
//
//go:embed all:*
var Files embed.FS
