package main

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
	"github.com/Dyu-36/gotack/internal/schedule"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
)

type reviewRequest struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt"`
}

type reviewTransport struct {
	t       *testing.T
	mu      sync.Mutex
	deleted []string
	agentCh chan reviewRequest
}

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

func (f *reviewTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	switch {
	case req.Method == http.MethodGet && path == "/v1/workspaces":
		return jsonHTTPResponse(http.StatusOK, `[]`), nil
	case req.Method == http.MethodPost && path == "/v1/workspaces":
		return jsonHTTPResponse(http.StatusOK, `{"id":"ws-1","path":"C:/"}`), nil
	case req.Method == http.MethodGet && strings.HasSuffix(path, "/sessions/src-1/messages"):
		return jsonHTTPResponse(http.StatusOK, `[{"id":"m-1","role":"user","session_id":"src-1","parts":[{"type":"text","data":{"text":"Prefer concise answers"}}]}]`), nil
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/sessions"):
		return jsonHTTPResponse(http.StatusOK, `{"id":"review-1","title":"Background review"}`), nil
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/agent"):
		var body reviewRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			f.t.Fatalf("decode agent request: %v", err)
		}
		f.agentCh <- body
		return jsonHTTPResponse(http.StatusAccepted, `{}`), nil
	case req.Method == http.MethodDelete && strings.HasSuffix(path, "/sessions/review-1"):
		f.mu.Lock()
		f.deleted = append(f.deleted, path)
		f.mu.Unlock()
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	default:
		f.t.Fatalf("unexpected request: %s %s", req.Method, path)
		return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
	}
}

func reviewTestApp(t *testing.T, transport *reviewTransport) *App {
	t.Helper()
	redirectAppData(t)
	oldMemory, oldSkills := resolveMemoryCommand, resolveSkillsCommand
	resolveMemoryCommand = func() string { return "memory" }
	resolveSkillsCommand = func() string { return "skills" }
	t.Cleanup(func() {
		resolveMemoryCommand = oldMemory
		resolveSkillsCommand = oldSkills
	})

	api := crushapi.NewClient(&http.Client{Transport: transport})
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
		t.Fatal("link rejected test connection")
	}
	app.link.MarkRunning()
	app.startReflection()
	if _, err := app.getConn().ws.Open(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestHostRunsBoundedReviewAndRemovesDetachedSession(t *testing.T) {
	transport := &reviewTransport{t: t, agentCh: make(chan reviewRequest, 1)}
	app := reviewTestApp(t, transport)
	app.reflection.Hydrate("src-1", 9)
	app.reflection.UserTurnAccepted("src-1")
	app.runDone(uievents.SessionDonePayload{SessionID: "src-1", Text: "done"})

	var req reviewRequest
	select {
	case req = <-transport.agentCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for background review")
	}
	if req.SessionID != "review-1" || !strings.Contains(req.Prompt, "Prefer concise answers") {
		t.Fatalf("review request = %+v", req)
	}
	roster := filepath.Join(appconfig.Dir(), guard.ReviewRosterFileName)
	if !guard.ReviewRosterContains(roster, "review-1") {
		t.Fatal("review was sent before its restricted-session marker")
	}

	app.runDone(uievents.SessionDonePayload{SessionID: "review-1", Text: "Nothing to save."})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		transport.mu.Lock()
		deleted := len(transport.deleted) == 1
		transport.mu.Unlock()
		if deleted && !guard.ReviewRosterContains(roster, "review-1") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("detached review session or roster marker was not cleaned")
}

func TestScheduledRunSuppressesAndForgetsBackgroundReview(t *testing.T) {
	transport := &reviewTransport{t: t, agentCh: make(chan reviewRequest, 1)}
	app := reviewTestApp(t, transport)

	schedulePath := filepath.Join(t.TempDir(), schedule.FileName)
	scheduleData, err := json.Marshal(schedule.File{Jobs: []*schedule.Job{{
		ID: "job-1", Name: "scheduled", Prompt: "run", Every: "1h", Enabled: true,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schedulePath, scheduleData, 0o600); err != nil {
		t.Fatal(err)
	}
	launched := make(chan struct{}, 1)
	app.scheduler = schedule.New(schedulePath, schedule.Runtime{
		CreateSession:  func(context.Context, string) (string, error) { return "src-1", nil },
		MarkUnattended: func(context.Context, string) error { return nil },
		SendPrompt: func(context.Context, string, string) error {
			launched <- struct{}{}
			return nil
		},
	}, nil)
	if err := app.scheduler.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.scheduler.Stop)
	app.scheduler.SetEngineReady(true)
	select {
	case <-launched:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for scheduled run")
	}

	app.reflection.Hydrate("src-1", 9)
	app.reflection.UserTurnAccepted("src-1")
	app.runDone(uievents.SessionDonePayload{SessionID: "src-1", Text: "scheduled result"})

	app.reflection.UserTurnAccepted("src-1")
	app.runDone(uievents.SessionDonePayload{SessionID: "src-1", Text: "foreground result"})
	select {
	case request := <-transport.agentCh:
		t.Fatalf("scheduled run leaked into background review: %+v", request)
	case <-time.After(100 * time.Millisecond):
	}
}
