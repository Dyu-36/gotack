package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

type memoryAPI struct {
	t           *testing.T
	setKeys     []string
	setValues   map[string]json.RawMessage
	removedKeys []string
}

func (f *memoryAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/config/set"):
		var payload struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			f.t.Errorf("decode config set request: %v", err)
			return jsonHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}
		f.setKeys = append(f.setKeys, payload.Key)
		if f.setValues == nil {
			f.setValues = map[string]json.RawMessage{}
		}
		f.setValues[payload.Key] = payload.Value
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/config/remove"):
		var payload struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			f.t.Errorf("decode config remove request: %v", err)
			return jsonHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}
		f.removedKeys = append(f.removedKeys, payload.Key)
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	default:
		f.t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
	}
}

func newMemoryTestApp(t *testing.T, fake *memoryAPI) *App {
	t.Helper()
	api := crushapi.NewClient(&http.Client{Transport: fake})
	app := NewApp()
	app.ctx = context.Background()
	app.swapConn(func(c *conn) *conn {
		c.api = api
		c.ws = workspace.NewService(api)
		c.sess = session.NewService(api, c.ws)
		return c
	})
	scope, started := app.link.BeginConnect(context.Background())
	if !started || !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
		t.Fatal("link rejected the test connect scope")
	}
	app.link.MarkRunning()
	return app
}

func withMemoryResolver(t *testing.T, command string) {
	t.Helper()
	previous := resolveMemoryCommand
	resolveMemoryCommand = func() string { return command }
	t.Cleanup(func() { resolveMemoryCommand = previous })
}

func TestRegisterMemoryToolsWritesStdioEntry(t *testing.T) {
	fake := &memoryAPI{t: t}
	app := newMemoryTestApp(t, fake)
	withMemoryResolver(t, `C:\bundled\memory.exe`)

	app.registerMemoryTools("ws-1")

	value, ok := fake.setValues["mcp_servers.gotack-memory"]
	if !ok {
		t.Fatalf("mcp_servers.gotack-memory never written; keys: %v", fake.setKeys)
	}
	var entry map[string]any
	if err := json.Unmarshal(value, &entry); err != nil {
		t.Fatalf("registered value is not an object: %v", err)
	}
	want := map[string]any{"command": `C:\bundled\memory.exe`, "type": "stdio", "timeout": float64(30)}
	if len(entry) != len(want) {
		t.Fatalf("entry = %v, want exactly %v", entry, want)
	}
	for key, expect := range want {
		if entry[key] != expect {
			t.Fatalf("entry[%q] = %v, want %v (full: %v)", key, entry[key], expect, entry)
		}
	}
	if len(fake.removedKeys) != 0 {
		t.Fatalf("unexpected removals: %v", fake.removedKeys)
	}
}

func TestRegisterMemoryToolsRemovesKeyWhenBinaryMissing(t *testing.T) {
	fake := &memoryAPI{t: t}
	app := newMemoryTestApp(t, fake)
	withMemoryResolver(t, "")

	app.registerMemoryTools("ws-1")

	if len(fake.setKeys) != 0 {
		t.Fatalf("unexpected writes: %v", fake.setKeys)
	}
	if len(fake.removedKeys) != 1 || fake.removedKeys[0] != "mcp_servers.gotack-memory" {
		t.Fatalf("removals = %v, want exactly [mcp_servers.gotack-memory]", fake.removedKeys)
	}
}

func TestMemoryEntryShape(t *testing.T) {
	entry := memoryEntry("/x/memory.exe")
	if entry["command"] != "/x/memory.exe" || entry["type"] != "stdio" || entry["timeout"] != 30 {
		t.Fatalf("memoryEntry = %v", entry)
	}
}
