// Package buildinfo identifies all executables built from this repository.
package buildinfo

import "runtime/debug"

const Protocol = 1

var Commit = ""
var SourceDigest = ""

func Revision() string {
	if Commit != "" {
		return Commit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return ""
}
