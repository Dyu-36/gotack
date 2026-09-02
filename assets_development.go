//go:build !production

package main

import "testing/fstest"

var assets = fstest.MapFS{
	"frontend/dist/index.html": &fstest.MapFile{
		Data: []byte("<!doctype html><title>Gotack development assets</title>"),
		Mode: 0o444,
	},
}
