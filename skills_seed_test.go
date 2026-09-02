package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

type skillsAPI struct {
	t           *testing.T
	setValues   map[string]json.RawMessage
	removedKeys []string
}

func (f *skillsAPI) RoundTrip(request *http.Request) (*http.Response, error) {
	switch {
	case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/config/set"):
		var payload struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			f.t.Fatalf("decode config set request: %v", err)
		}
		if f.setValues == nil {
			f.setValues = make(map[string]json.RawMessage)
		}
		f.setValues[payload.Key] = payload.Value
		return skillsHTTPResponse(http.StatusOK, "{}"), nil
	case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/config/remove"):
		var payload struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			f.t.Fatalf("decode config remove request: %v", err)
		}
		f.removedKeys = append(f.removedKeys, payload.Key)
		return skillsHTTPResponse(http.StatusOK, "{}"), nil
	default:
		f.t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		return skillsHTTPResponse(http.StatusNotFound, "{}"), nil
	}
}

func skillsHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newSkillsTestApp(t *testing.T, transport http.RoundTripper) *App {
	t.Helper()
	api := crushapi.NewClient(&http.Client{Transport: transport})
	app := NewApp()
	app.ctx = context.Background()
	app.swapConn(func(connection *conn) *conn {
		connection.api = api
		connection.ws = workspace.NewService(api)
		connection.sess = session.NewService(api, connection.ws)
		return connection
	})
	scope, started := app.link.BeginConnect(context.Background())
	if !started || !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
		t.Fatal("link rejected the test connect scope")
	}
	app.link.MarkRunning()
	return app
}

func withSkillsResolver(t *testing.T, command string) {
	t.Helper()
	previous := resolveSkillsCommand
	resolveSkillsCommand = func() string { return command }
	t.Cleanup(func() { resolveSkillsCommand = previous })
}

func TestRegisterSkillsToolsUsesUserScope(t *testing.T) {
	fake := &skillsAPI{t: t}
	app := newSkillsTestApp(t, fake)
	withSkillsResolver(t, "C:/bundled/skills.exe")

	app.registerSkillsTools("workspace-1")

	raw, ok := fake.setValues["mcp_servers.gotack-skills"]
	if !ok {
		t.Fatalf("skills MCP entry not written: %v", fake.setValues)
	}
	var entry struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
		Type    string   `json:"type"`
		Timeout int      `json:"timeout"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Command != "C:/bundled/skills.exe" || entry.Type != "stdio" || entry.Timeout != 30 {
		t.Fatalf("entry = %+v", entry)
	}
	if len(entry.Args) != 2 || entry.Args[0] != "--root" || entry.Args[1] != userSkillsDir() {
		t.Fatalf("skills args = %v, want [--root %s]", entry.Args, userSkillsDir())
	}
	if strings.Contains(filepathSlash(entry.Args[1]), "/.agents/skills") {
		t.Fatalf("project-owned directory leaked into writer args: %v", entry.Args)
	}
}

func TestMissingSkillsBinaryUnlocksConfig(t *testing.T) {
	fake := &skillsAPI{t: t}
	app := newSkillsTestApp(t, fake)
	withSkillsResolver(t, "")

	app.registerSkillsTools("workspace-1")

	if len(fake.setValues) != 0 {
		t.Fatalf("unexpected config writes: %v", fake.setValues)
	}
	if len(fake.removedKeys) != 1 || fake.removedKeys[0] != "mcp_servers.gotack-skills" {
		t.Fatalf("removed keys = %v", fake.removedKeys)
	}
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}
