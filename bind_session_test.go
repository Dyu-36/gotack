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

	"github.com/Dyu-36/gotack/internal/attachments"
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

func TestDecodePromptAttachmentsRoutesBytesAndDerivedText(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	got := decodePromptAttachments([]PromptAttachment{{
		FileName: `C:\tmp\photo.png`,
		Content:  base64.StdEncoding.EncodeToString(png),
	}}, true)
	if len(got) != 1 || got[0].DisplayName != "photo.png" || !strings.HasSuffix(got[0].Path, "photo.png") {
		t.Fatalf("prepared attachment = %#v", got)
	}
	if got[0].MimeType != "image/png" || got[0].Attachment == nil {
		t.Fatalf("a vision model must receive the raw image, got %#v", got[0])
	}
	if string(got[0].Attachment.Content) != string(png) {
		t.Fatalf("attachment bytes = %q, want the uploaded bytes", got[0].Attachment.Content)
	}

	// A text-only model gets derived text in the prompt instead of image bytes.
	gotNonVision := decodePromptAttachments([]PromptAttachment{{
		FileName: `C:\tmp\photo.png`,
		Content:  base64.StdEncoding.EncodeToString(png),
	}}, false)
	if len(gotNonVision) != 1 || gotNonVision[0].Attachment != nil {
		t.Fatalf("a text-only model must not receive image bytes, got %#v", gotNonVision)
	}
	if gotNonVision[0].PromptBlock == "" {
		t.Fatalf("expected a derived prompt block for the image fallback")
	}

	// Text files travel inside the prompt, never as a Crush binary attachment.
	jsonCode := []byte(`{"version": "1.0.0"}`)
	gotText := decodePromptAttachments([]PromptAttachment{{
		FileName: "config.json",
		MimeType: "application/json",
		Content:  base64.StdEncoding.EncodeToString(jsonCode),
	}}, true)
	if len(gotText) != 1 || gotText[0].Attachment != nil {
		t.Fatalf("text attachment = %#v", gotText)
	}
	if !strings.Contains(gotText[0].PromptBlock, `{"version": "1.0.0"}`) {
		t.Fatalf("prompt block = %q, want the file contents", gotText[0].PromptBlock)
	}
}

func TestDecodePromptAttachmentsFailsSoftPerFile(t *testing.T) {
	tooLarge := strings.Repeat("A", base64.StdEncoding.EncodedLen(attachments.MaxAttachmentSize)+1)
	got := decodePromptAttachments([]PromptAttachment{
		{FileName: "large.bin", Content: tooLarge},
		{FileName: "broken.txt", Content: "not-base64!!"},
		{FileName: "notes.txt", Content: base64.StdEncoding.EncodeToString([]byte("readable"))},
	}, true)
	if len(got) != 3 {
		t.Fatalf("decodePromptAttachments() returned %d items, want 3", len(got))
	}
	if !strings.Contains(got[0].Warning, "5 MB") {
		t.Fatalf("oversize warning = %q, want the 5 MB limit", got[0].Warning)
	}
	if got[1].Warning == "" {
		t.Fatalf("invalid base64 must degrade into a warning, got %#v", got[1])
	}
	if got[2].Warning != "" || !strings.Contains(got[2].PromptBlock, "readable") {
		t.Fatalf("one bad file must not drop the readable one, got %#v", got[2])
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
