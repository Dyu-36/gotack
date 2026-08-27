package appconfig

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// paths.go -- role: OS-specific path resolution.
//
// Config dir, data dir, log dir, and the Crush socket or named pipe identifier.
// Windows uses a named pipe, unix uses a socket file, see internal/crushapi.

// Dir returns the per-user config directory for gotack.
// On Windows it is %AppData%\gotack; elsewhere it is whatever os.UserConfigDir
// resolves to (XDG_CONFIG_HOME or $HOME/.config) with /gotack appended.
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

// socketDir returns the per-user runtime directory for unix sockets, falling
// back to /tmp when XDG_RUNTIME_DIR is empty.
func socketDir() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return dir
}

// currentUID returns the textual user id (UID on unix, SID on Windows) or ""
// if it cannot be resolved.
func currentUID() string {
	if u, err := user.Current(); err == nil && u != nil {
		return u.Uid
	}
	return ""
}

// endpointName returns the crush-named endpoint, always suffixed .sock. When
// no UID is available it falls back to the plain "crush.sock" name.
func endpointName() string {
	uid := currentUID()
	if uid == "" {
		return "crush.sock"
	}
	return "crush-" + uid + ".sock"
}

// PipeEndpoint returns the default Crush endpoint for this OS.
//
// The vendored crush server names its endpoint after user.Current().Uid on
// every platform: on unix that is the numeric uid, on Windows it is the
// account SID. The endpoint name is therefore `crush-<uid>.sock` (or
// `crush.sock` when no uid is available). On Windows the same name is used
// as the pipe name suffix after `\\.\pipe\`.
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
	// Unix socket paths are capped at 108 bytes (sun_path on Linux). When
	// XDG_RUNTIME_DIR pushes the joined path over the limit, fall back to
	// /tmp so listen(2) does not fail with ENAMETOOLONG.
	if len(addr) >= 108 {
		addr = filepath.Join("/tmp", name)
	}
	return crushapi.Endpoint{
		Network: "unix",
		Address: addr,
	}
}
