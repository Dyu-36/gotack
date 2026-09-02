package appconfig

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/Dyu-36/gotack/internal/crushapi"
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
	if dir == "" {
		dir = "/tmp"
	}
	return dir
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
		return "crush.sock"
	}
	return "crush-" + uid + ".sock"
}

func PipeEndpoint() crushapi.Endpoint {
	name := endpointName()
	if runtime.GOOS == "windows" {
		return crushapi.Endpoint{
			Network: "npipe",
			Address: `\\.\pipe\` + name,
		}
	}
	dir := socketDir()
	addr := filepath.Join(dir, name)

	if len(addr) >= 108 {
		addr = filepath.Join("/tmp", name)
	}
	return crushapi.Endpoint{
		Network: "unix",
		Address: addr,
	}
}
