package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type permissionWriter struct {
	bytes.Buffer
	ready chan struct{}
}

func (w *permissionWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if strings.Contains(string(p), "Allow?") {
		close(w.ready)
	}
	return n, err
}

func event(kind string, value any) engineapi.StreamEvent {
	payload, _ := json.Marshal(value)
	return engineapi.StreamEvent{Kind: kind, Payload: payload}
}

func TestPromptStreamsFiltersSessionsAndApprovesPermission(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	events := make(chan engineapi.StreamEvent, 10)
	lines := make(chan inputLine)
	diagnostic := &permissionWriter{ready: make(chan struct{})}
	var out bytes.Buffer
	var runID string
	api := engineapi.NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/v1/workspaces/ws/agent":
			var prompt struct {
				RunID     string `json:"run_id"`
				SessionID string `json:"session_id"`
				Prompt    string `json:"prompt"`
			}
			if err := json.NewDecoder(r.Body).Decode(&prompt); err != nil {
				t.Error(err)
			}
			if prompt.SessionID != "session" || prompt.Prompt != "hello" {
				t.Errorf("wrong prompt: %+v", prompt)
			}
			runID = prompt.RunID
			events <- event("run_complete", engineapi.RunComplete{SessionID: "other", RunID: runID})
			for _, text := range []string{"Hel", "Hello", "Hello"} {
				parts, _ := json.Marshal([]any{map[string]any{"type": "text", "data": map[string]string{"text": text}}})
				events <- event("message", engineapi.Message{ID: "m1", SessionID: "session", Role: "assistant", Parts: parts})
			}
			events <- event("permission_request", engineapi.PermissionRequest{ID: "permission", SessionID: "session", ToolName: "edit", Path: "file.txt"})
		case r.URL.Path == "/v1/workspaces/ws/permissions/grant":
			var grant struct {
				Action     string                      `json:"action"`
				Permission engineapi.PermissionRequest `json:"permission"`
			}
			if err := json.NewDecoder(r.Body).Decode(&grant); err != nil {
				t.Error(err)
			}
			if grant.Action != "allow" || grant.Permission.ID != "permission" {
				t.Errorf("wrong grant: %+v", grant)
			}
			events <- event("run_complete", engineapi.RunComplete{SessionID: "session", RunID: runID, Text: "Hello"})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header), Request: r}, nil
	})})
	go func() {
		select {
		case <-diagnostic.ready:
			select {
			case lines <- inputLine{text: "y"}:
			case <-ctx.Done():
			}
		case <-ctx.Done():
		}
	}()
	if err := runPrompt(ctx, api, "ws", "session", "hello", events, lines, &out, diagnostic); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Hello\n" {
		t.Fatalf("streamed output = %q", out.String())
	}
}

func TestCancelledPromptCancelsEngineWithFreshContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{})
	cancelled := make(chan struct{})
	api := engineapi.NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/cancel") {
			if r.Context().Err() != nil {
				t.Error("cancellation used expired context")
			}
			close(cancelled)
		} else {
			close(started)
		}
		return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header), Request: r}, nil
	})})
	done := make(chan error, 1)
	go func() {
		done <- runPrompt(ctx, api, "ws", "session", "hello", make(chan engineapi.StreamEvent), nil, io.Discard, io.Discard)
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("prompt did not cancel")
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("engine was not cancelled")
	}
}

func TestCLIRejectsInvalidArgumentsBeforeStartingEngine(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"run"}, {"chat", "unexpected"}, {"run", "--provider", "openai", "hello"}} {
		if err := Run(t.Context(), args, strings.NewReader(""), io.Discard, io.Discard); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
