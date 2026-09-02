//go:build !production

package main

import "testing/fstest"

// Wails serves the frontend development server outside production builds. A
// minimal in-memory filesystem keeps Go tooling independent of generated UI
// artifacts while preserving the production-only embed contract.
var assets = fstest.MapFS{
	"frontend/dist/index.html": &fstest.MapFile{
		Data: []byte("<!doctype html><title>Gotack development assets</title>"),
		Mode: 0o444,
	},
}
