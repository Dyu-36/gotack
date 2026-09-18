package appconfig

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

func Dir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("AppData")
		if base == "" {
			if fallback, err := os.UserConfigDir(); err == nil && fallback != "" {
				base = fallback
			} else {
				base = os.TempDir()
			}
		}
		return filepath.Join(base, "gotack")
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = filepath.Join(os.TempDir(), "gotack")
	}
	return filepath.Join(base, "gotack")
}

func socketDir() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDirUsable(dir) {
		return dir
	}
	fallback := Dir()
	_ = os.MkdirAll(fallback, 0o700)
	return fallback
}

func runtimeDirUsable(dir string) bool {
	if dir == "" || !filepath.IsAbs(dir) {
		return false
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o077 == 0
}

func currentUID() string {
	if u, err := user.Current(); err == nil && u != nil {
		return u.Uid
	}
	return ""
}

func endpointName() string {
	uid := currentUID()
	if uid == "" {
		return "tack-engine.sock"
	}
	return "tack-engine-" + uid + ".sock"
}

func PipeEndpoint() engineapi.Endpoint {
	name := endpointName()
	if runtime.GOOS == "windows" {
		return engineapi.Endpoint{
			Network: "npipe",
			Address: `\\.\pipe\` + name,
		}
	}
	dir := socketDir()
	addr := filepath.Join(dir, name)

	if len(addr) >= 108 {
		addr = filepath.Join("/tmp", name)
	}
	return engineapi.Endpoint{
		Network: "unix",
		Address: addr,
	}
}
