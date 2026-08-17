package frontend

import "embed"

// Assets contains the production renderer built by Vite. The tracked marker
// keeps the embed target valid for Go-only tests before the renderer is built.
//
//go:embed all:dist
var Assets embed.FS
