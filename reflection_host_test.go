package main

// reflection_host_test.go -- role: host-wiring regression tests for the
// Phase 6 reflection loop (WP8). The unit semantics (gate, budget, recursion
// guard) live in internal/reflection; here we prove the host plumbing:
// run_complete opens the gate through App.RunDone, the firing walks the real
// REST session seam (create -> mark unattended -> send), session deletion is
// the session-end trigger, and a reflection run's own run_complete never
// starts another run.

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/guard"
	"github.com/Dyu-36/gotack/internal/reflection"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
)

// reflectionRequest is one decoded agent POST: the prompt the engine
// received for the reflection session.
type reflectionRequest struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt"`
}

// reflectionAPI is a fake Crush transport covering exactly the endpoints a
// reflection firing touches: workspace list/create, session create, agent
// prompt, session delete. It records the request order so the create ->
// send -> delete sequence can be asserted.
type reflectionAPI struct {
	t       *testing.T
	mu      sync.Mutex
	order   []string
	titles  []string
	deleted []string
	agentCh chan reflectionRequest
}

func (f *reflectionAPI) record(method, label string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.order = append(f.order, method+" "+label)
}

func (f *reflectionAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	switch {
	case req.Method == http.MethodGet && path == "/v1/workspaces":
		f.record(req.Method, path)
		return jsonHTTPResponse(http.StatusOK, `[]`), nil
	case req.Method == http.MethodPost && path == "/v1/workspaces":
		f.record(req.Method, path)
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return jsonHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}
		out, _ := json.Marshal(map[string]string{"id": "ws-1", "path": body.Path})
		return jsonHTTPResponse(http.StatusOK, string(out)), nil
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/sessions"):
		f.record(req.Method, path)
		var body struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return jsonHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}
		f.mu.Lock()
		f.titles = append(f.titles, body.Title)
		f.mu.Unlock()
		out, _ := json.Marshal(map[string]string{"id": "refl-1", "title": body.Title})
		return jsonHTTPResponse(http.StatusOK, string(out)), nil
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/agent"):
		f.record(req.Method, path)
		var reqBody reflectionRequest
		if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
			return jsonHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}
		select {
		case f.agentCh <- reqBody:
		default:
			f.t.Errorf("agent request with no waiter: %+v", reqBody)
		}
		return jsonHTTPResponse(http.StatusAccepted, `{}`), nil
	case req.Method == http.MethodDelete && strings.Contains(path, "/sessions/"):
		f.record(req.Method, path)
		f.mu.Lock()
		f.deleted = append(f.deleted, path)
		f.mu.Unlock()
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	default:
		f.t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		return jsonHTTPResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	}
}

// redirectAppData points appconfig.Dir() at a temp dir so the unattended
// roster the reflection mark writes can be inspected without touching the
// real user profile.
func redirectAppData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	return dir
}

// newReflectionTestApp wires an App over the fake transport with a live
// connection, a configured model (preflight passes) and a started tracker,
// then opens a workspace so session calls resolve a workspace id.
func newReflectionTestApp(t *testing.T, fake *reflectionAPI) *App {
	t.Helper()
	api := crushapi.NewClient(&http.Client{Transport: fake})
	app := NewApp()
	app.ctx = context.Background()
	app.cfg = &appconfig.Config{Model: "test-model"}
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
	app.startReflection()
	if _, err := app.getConn().ws.Open(context.Background(), t.TempDir()); err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	return app
}

func waitAgent(t *testing.T, ch chan reflectionRequest) reflectionRequest {
	t.Helper()
	select {
	case req := <-ch:
		return req
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the reflection prompt")
		return reflectionRequest{}
	}
}

// TestRunDoneThresholdTriggersReflection proves the run_complete path end to
// end: after the turn threshold the host creates a tagged session, marks it
// unattended BEFORE the prompt, and submits the D3-routed reflection prompt.
func TestRunDoneThresholdTriggersReflection(t *testing.T) {
	appData := redirectAppData(t)
	fake := &reflectionAPI{t: t, agentCh: make(chan reflectionRequest, 4)}
	app := newReflectionTestApp(t, fake)

	for i := 0; i < reflection.DefaultTurnThreshold; i++ {
		app.RunDone(uievents.SessionDonePayload{SessionID: "src-1"})
	}
	req := waitAgent(t, fake.agentCh)

	if req.SessionID != "refl-1" {
		t.Fatalf("prompt went to session %q, want the created reflection session", req.SessionID)
	}
	fake.mu.Lock()
	title := fake.titles[len(fake.titles)-1]
	fake.mu.Unlock()
	if want := reflection.TitlePrefix + "src-1"; title != want {
		t.Fatalf("reflection session title = %q, want %q", title, want)
	}
	if !strings.Contains(req.Prompt, "src-1") {
		t.Errorf("prompt does not reference the source session: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "memory tool") {
		t.Errorf("prompt does not route memory writes through the gotack-memory tool: %q", req.Prompt)
	}
	// The unattended mark must precede the prompt (ADR 0002): the roster
	// file already contains the session by the time the agent call lands.
	roster, err := os.ReadFile(filepath.Join(appData, "gotack", guard.UnattendedRosterFileName))
	if err != nil {
		t.Fatalf("read unattended roster: %v", err)
	}
	if !strings.Contains(string(roster), "refl-1") {
		t.Errorf("roster %q does not contain the reflection session", roster)
	}
}

// TestReflectionCompletionDoesNotRetrigger proves the recursion guard at the
// host boundary: run_complete events emitted by the reflection run itself
// never start another reflection run.
func TestReflectionCompletionDoesNotRetrigger(t *testing.T) {
	redirectAppData(t)
	fake := &reflectionAPI{t: t, agentCh: make(chan reflectionRequest, 4)}
	app := newReflectionTestApp(t, fake)

	for i := 0; i < reflection.DefaultTurnThreshold; i++ {
		app.RunDone(uievents.SessionDonePayload{SessionID: "src-1"})
	}
	waitAgent(t, fake.agentCh)

	for i := 0; i < reflection.DefaultTurnThreshold*2; i++ {
		app.RunDone(uievents.SessionDonePayload{SessionID: "refl-1"})
	}
	select {
	case req := <-fake.agentCh:
		t.Fatalf("reflection completion retriggered the loop: %+v", req)
	case <-time.After(300 * time.Millisecond):
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.titles) != 1 {
		t.Fatalf("sessions created = %v, want exactly one", fake.titles)
	}
}

// TestDeleteSessionTriggersSessionEndReflection proves the session-end gate:
// one completed turn plus session deletion fires a reflection BEFORE the
// delete reaches the engine, so the source conversation is still readable.
func TestDeleteSessionTriggersSessionEndReflection(t *testing.T) {
	redirectAppData(t)
	fake := &reflectionAPI{t: t, agentCh: make(chan reflectionRequest, 4)}
	app := newReflectionTestApp(t, fake)

	app.RunDone(uievents.SessionDonePayload{SessionID: "src-2"})
	if err := app.DeleteSession("src-2"); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	req := waitAgent(t, fake.agentCh)
	if !strings.Contains(req.Prompt, "src-2") {
		t.Errorf("prompt does not reference the deleted session: %q", req.Prompt)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	want := []string{
		"POST /v1/workspaces/ws-1/sessions",
		"POST /v1/workspaces/ws-1/agent",
		"DELETE /v1/workspaces/ws-1/sessions/src-2",
	}
	var sessionOps []string
	for _, op := range fake.order {
		if strings.Contains(op, "/sessions") || strings.Contains(op, "/agent") {
			if !strings.HasPrefix(op, "GET") && op != "GET /v1/workspaces" {
				sessionOps = append(sessionOps, op)
			}
		}
	}
	if len(sessionOps) != len(want) {
		t.Fatalf("session operations = %v, want %v", sessionOps, want)
	}
	for i := range want {
		if sessionOps[i] != want[i] {
			t.Fatalf("operation %d = %q, want %q (full order: %v)", i, sessionOps[i], want[i], fake.order)
		}
	}
}

// TestReflectionSkipsWithoutModel proves the preflight: with no model
// configured the gate refuses cleanly and no engine call is made.
func TestReflectionSkipsWithoutModel(t *testing.T) {
	redirectAppData(t)
	fake := &reflectionAPI{t: t, agentCh: make(chan reflectionRequest, 4)}
	app := newReflectionTestApp(t, fake)
	app.cfg = &appconfig.Config{}

	for i := 0; i < reflection.DefaultTurnThreshold; i++ {
		app.RunDone(uievents.SessionDonePayload{SessionID: "src-1"})
	}
	select {
	case req := <-fake.agentCh:
		t.Fatalf("reflection fired without a model: %+v", req)
	case <-time.After(300 * time.Millisecond):
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.titles) != 0 {
		t.Fatalf("sessions created = %v, want none", fake.titles)
	}
}
