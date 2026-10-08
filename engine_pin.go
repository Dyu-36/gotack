package main

import "github.com/Dyu-36/gotack/internal/buildinfo"

var packagedEngineCommit = buildinfo.Revision()

func expectedEngineCommit(customBinary string) string {
	if customBinary != "" {
		return ""
	}
	return packagedEngineCommit
}
