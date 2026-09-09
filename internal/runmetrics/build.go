package runmetrics

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime/debug"
	"sync"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

var applicationBuild = sync.OnceValue(func() engineapi.BuildTelemetry {
	build := engineapi.BuildTelemetry{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				build.Commit = setting.Value
			case "vcs.modified":
				build.Modified = setting.Value == "true"
			}
		}
	}
	if path, err := os.Executable(); err == nil {
		if file, err := os.Open(path); err == nil {
			defer file.Close()
			digest := sha256.New()
			if _, err := io.Copy(digest, file); err == nil {
				build.ID = hex.EncodeToString(digest.Sum(nil))
			}
		}
	}
	return build
})
