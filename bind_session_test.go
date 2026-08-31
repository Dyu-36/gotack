package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestToMessageInfoIncludesConversationModel(t *testing.T) {
	parts := json.RawMessage(`[
		{"type":"text","data":{"text":"hello"}},
		{"type":"binary","data":{"Path":"photo.png","MIMEType":"image/png","Data":"iVBORw=="}}
	]`)
	got := toMessageInfo(crushapi.Message{
		ID:        "message-1",
		Role:      "assistant",
		Parts:     parts,
		Model:     "gpt-5.6",
		Provider:  "openai",
		CreatedAt: 42,
	})

	if got.Text != "hello" {
		t.Fatalf("Text = %q, want hello", got.Text)
	}
	if got.Model != "gpt-5.6" || got.Provider != "openai" {
		t.Fatalf("model selection = %q/%q, want openai/gpt-5.6", got.Provider, got.Model)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].FileName != "photo.png" || got.Attachments[0].Content != "iVBORw==" {
		t.Fatalf("attachments = %#v", got.Attachments)
	}
}

func TestDecodePromptAttachmentsSniffsAndLimitsContent(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	got, err := decodePromptAttachments([]PromptAttachment{{
		FileName: `C:\tmp\photo.png`,
		Content:  base64.StdEncoding.EncodeToString(png),
	}}, true)
	if err != nil {
		t.Fatalf("decodePromptAttachments() error = %v", err)
	}
	if len(got) != 1 || got[0].FileName != "photo.png" || !strings.HasSuffix(got[0].FilePath, "photo.png") {
		t.Fatalf("decoded attachment = %#v", got)
	}
	if got[0].MimeType != "image/png" || string(got[0].Content) != string(png) {
		t.Fatalf("decoded content = %#v", got[0])
	}

	// Text-only mode converts image to text attachment with metadata/OCR
	gotNonVision, err := decodePromptAttachments([]PromptAttachment{{
		FileName: `C:\tmp\photo.png`,
		Content:  base64.StdEncoding.EncodeToString(png),
	}}, false)
	if err != nil {
		t.Fatalf("decodePromptAttachments(non-vision) error = %v", err)
	}
	if len(gotNonVision) != 1 || gotNonVision[0].MimeType != "text/plain; charset=utf-8" {
		t.Fatalf("expected text fallback for non-vision attachment, got %#v", gotNonVision)
	}

	jsonCode := []byte(`{"version": "1.0.0"}`)
	gotText, err := decodePromptAttachments([]PromptAttachment{{
		FileName: "config.json",
		MimeType: "application/json",
		Content:  base64.StdEncoding.EncodeToString(jsonCode),
	}}, true)
	if err != nil {
		t.Fatalf("decodePromptAttachments(json) error = %v", err)
	}
	if len(gotText) != 1 || gotText[0].MimeType != "text/plain; charset=utf-8" || !strings.Contains(string(gotText[0].Content), `{"version": "1.0.0"}`) {
		t.Fatalf("decoded text attachment = %#v", gotText)
	}

	tooLarge := strings.Repeat("A", base64.StdEncoding.EncodedLen(maxPromptAttachmentSize)+1)
	if _, err := decodePromptAttachments([]PromptAttachment{{FileName: "large.bin", Content: tooLarge}}, true); err == nil || !strings.Contains(err.Error(), "5 MB") {
		t.Fatalf("oversized attachment error = %v, want 5 MB limit", err)
	}
}

type contextBody struct {
	ctx context.Context
}

func (b contextBody) Read([]byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (contextBody) Close() error { return nil }

func TestSetCurrentSessionReattachesMissingEventStream(t *testing.T) {
	workspacePath := t.TempDir()
	var currentSessionCalls atomic.Int32
	var streamCalls atomic.Int32

	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		response := func(status int, body io.ReadCloser) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: body, Header: make(http.Header), Request: req}, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces":
			body := `[{"id":"ws-1","path":` + strconvQuote(workspacePath) + `}]`
			return response(http.StatusOK, io.NopCloser(strings.NewReader(body)))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/current-session":
			if currentSessionCalls.Add(1) == 1 {
				return response(http.StatusConflict, io.NopCloser(strings.NewReader(`{"message":"client not attached"}`)))
			}
			return response(http.StatusOK, io.NopCloser(strings.NewReader("")))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/events":
			streamCalls.Add(1)
			resp, err := response(http.StatusOK, contextBody{ctx: req.Context()})
			resp.Header.Set("Content-Type", "text/event-stream")
			return resp, err
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	api := crushapi.NewClient(&http.Client{Transport: transport})
	ws := workspace.NewService(api)
	if _, err := ws.Open(context.Background(), workspacePath); err != nil {
		t.Fatalf("open workspace: %v", err)
	}

	a := NewApp()
	a.ctx = context.Background()
	a.log = slog.New(slog.DiscardHandler)
	a.conn.Store(&conn{
		api: api,
		ws:  ws,
		fwd: uievents.NewForwarder(a.log, func(string, any) {}, nil, nil),
	})
	if err := a.setCurrentSession("session-1"); err != nil {
		t.Fatalf("setCurrentSession() error = %v", err)
	}
	t.Cleanup(func() {
		if c := a.getConn(); c != nil && c.cancelStream != nil {
			c.cancelStream()
		}
	})

	if got := currentSessionCalls.Load(); got != 2 {
		t.Fatalf("current-session calls = %d, want 2", got)
	}
	if got := streamCalls.Load(); got != 1 {
		t.Fatalf("event stream attaches = %d, want 1", got)
	}
}

func strconvQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
