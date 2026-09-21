package main

import (
	_ "embed"
	"strings"
)

//go:embed .tack-pin
var packagedEngineCommit string

func expectedEngineCommit(customBinary string) string {
	if customBinary != "" {
		return ""
	}
	return strings.TrimSpace(packagedEngineCommit)
}
