package appconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPipeEndpointShape(t *testing.T) {

	uid := currentUID()
	wantSuffix := "tack-engine.sock"
	if uid != "" {
		wantSuffix = "tack-engine-" + uid + ".sock"
	}
	ep := PipeEndpoint()
	if runtime.GOOS == "windows" {
		if ep.Network != "npipe" {
			t.Errorf("windows network=%q want npipe", ep.Network)
		}
		if !strings.HasPrefix(ep.Address, `\\.\pipe\`) {
			t.Errorf("windows address=%q missing pipe prefix", ep.Address)
		}
		if !strings.HasSuffix(ep.Address, wantSuffix) {
			t.Errorf("windows address=%q does not end with %q", ep.Address, wantSuffix)
		}
		return
	}
	if ep.Network != "unix" {
		t.Errorf("unix network=%q want unix", ep.Network)
	}
	if !strings.HasSuffix(ep.Address, wantSuffix) {
		t.Errorf("unix address=%q does not end with %q", ep.Address, wantSuffix)
	}

	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	if !strings.HasPrefix(ep.Address, dir+string(filepath.Separator)) &&
		!strings.HasPrefix(ep.Address, "/tmp/"+wantSuffix) {
		t.Errorf("unix address=%q not under %q or /tmp", ep.Address, dir)
	}
}

func TestSocketDirLongPathFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix socket path length only applies to unix")
	}
	t.Setenv("XDG_RUNTIME_DIR", "/"+strings.Repeat("a", 200))
	ep := PipeEndpoint()
	if !strings.HasPrefix(ep.Address, "/tmp/") {
		t.Fatalf("expected /tmp/ fallback for long XDG path, got %q", ep.Address)
	}
	if len(ep.Address) >= 108 {
		t.Fatalf("address still over sun_path cap: %d", len(ep.Address))
	}
}

func TestDirOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path")
	}
	t.Setenv("AppData", `C:\Users\me\AppData\Roaming`)
	if got := Dir(); got != filepath.Join(`C:\Users\me\AppData\Roaming`, "gotack") {
		t.Fatalf("Dir()=%q", got)
	}
}

func TestEndpointNameUID(t *testing.T) {
	uid := currentUID()
	if uid == "" {
		if got := endpointName(); got != "tack-engine.sock" {
			t.Fatalf("no-uid name=%q want tack-engine.sock", got)
		}
		return
	}
	want := "tack-engine-" + uid + ".sock"
	if got := endpointName(); got != want {
		t.Fatalf("uid name=%q want %q", got, want)
	}
}
