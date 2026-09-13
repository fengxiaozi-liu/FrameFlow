package web

import "embed"

// Files contains the production Vue bundle. The Windows build script refreshes
// this directory before compiling the self-contained executable.
//
//go:embed dist/*
var Files embed.FS
