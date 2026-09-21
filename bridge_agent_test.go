package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/google/uuid"
)

func TestBridgeAgentReadEditAndRemoteCompletion(t *testing.T) {
	api, root := bridgeTestRuntime(t)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	path := filepath.Join(root, "roundtrip.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var requests int
	var observedRead, observedEdit bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"contract-model","object":"model","owned_by":"test"}]}`)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected provider endpoint", http.StatusNotFound)
			return
		}
		var input struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role      string `json:"role"`
				Content   any    `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"messages"`
			Tools []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests++
		count := requests
		mu.Unlock()
		if count > 16 {
			http.Error(w, "agent exceeded expected contract steps", http.StatusBadRequest)
			return
		}
		read, edited := false, false
		for _, message := range input.Messages {
			for _, call := range message.ToolCalls {
				read = read || call.Function.Name == "read"
				edited = edited || call.Function.Name == "edit"
			}
		}
		text, tool, arguments := "Agent contract verified", "", ""
		if len(input.Tools) == 0 {
			text = "Roundtrip contract"
		} else if !read {
			tool = "read"
			raw, _ := json.Marshal(map[string]any{"file_path": path})
			arguments = string(raw)
		} else if !edited {
			tool = "edit"
			raw, _ := json.Marshal(map[string]any{"file_path": path, "old_string": "before", "new_string": "after"})
			arguments = string(raw)
		}
		mu.Lock()
		observedRead = observedRead || read
		observedEdit = observedEdit || edited
		mu.Unlock()
		id := "chatcmpl-" + uuid.NewString()
		finish := "stop"
		message := map[string]any{"role": "assistant", "content": text}
		if tool != "" {
			finish = "tool_calls"
			message["content"] = nil
			message["tool_calls"] = []any{map[string]any{"index": 0, "id": "call-" + tool, "type": "function", "function": map[string]any{"name": tool, "arguments": arguments}}}
		}
		if !input.Stream {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "object": "chat.completion", "created": time.Now().Unix(), "model": "contract-model", "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}, "usage": map[string]int{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120}})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		write := func(delta map[string]any, reason any) {
			raw, _ := json.Marshal(map[string]any{"id": id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": "contract-model", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		write(message, nil)
		write(map[string]any{}, finish)
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer provider.Close()
	ws, err := api.CreateWorkspace(ctx, root, true)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]any{
		"providers.contract": map[string]any{
			"name": "Contract provider", "type": "openai-compat", "base_url": provider.URL + "/v1", "api_key": "test-key", "discover_models": false,
			"models": []map[string]any{{"id": "contract-model", "name": "Contract model", "context_window": 32768, "default_max_tokens": 2048, "can_reason": false, "supports_attachments": false}},
		},
	}
	if err := api.SetConfigFields(ctx, ws.ID, engineapi.ConfigScopeWorkspace, fields); err != nil {
		t.Fatalf("configure contract provider: %v", err)
	}
	if err := api.SetPreferredModelPair(ctx, ws.ID, engineapi.ConfigScopeWorkspace, engineapi.SelectedModel{Provider: "contract", Model: "contract-model"}); err != nil {
		t.Fatal(err)
	}
	if err := api.InitAgent(ctx, ws.ID, false); err != nil {
		t.Fatalf("initialize real agent: %v", err)
	}
	visible, err := api.CreateSession(ctx, ws.ID, "desktop-visible")
	if err != nil {
		t.Fatal(err)
	}
	remote, err := api.CreateSession(ctx, ws.ID, "zalo-remote")
	if err != nil {
		t.Fatal(err)
	}
	if err := api.SetCurrentSession(ctx, ws.ID, visible.ID); err != nil {
		t.Fatal(err)
	}
	events, stop, err := api.Stream(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	runID := uuid.NewString()
	if err := api.SendPromptWithPurpose(ctx, ws.ID, remote.ID, "Read roundtrip.txt, replace before with after, then confirm the edit.", runID, "zalo", nil); err != nil {
		t.Fatal(err)
	}
	var completion engineapi.RunComplete
wait:
	for {
		select {
		case <-ctx.Done():
			mu.Lock()
			t.Logf("provider requests=%d, saw read=%v edit=%v", requests, observedRead, observedEdit)
			mu.Unlock()
			t.Fatal("real agent did not complete the non-selected remote session")
		case event, ok := <-events:
			if !ok {
				t.Fatal("engine event stream ended before completion")
			}
			if event.Kind != "run_complete" {
				continue
			}
			if err := json.Unmarshal(event.Payload, &completion); err != nil {
				t.Fatal(err)
			}
			if completion.RunID == runID {
				break wait
			}
		}
	}
	if completion.SessionID != remote.ID || completion.Error != "" || completion.Cancelled || !strings.Contains(completion.Text, "Agent contract verified") {
		t.Fatalf("incorrect real-agent terminal event: %+v", completion)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "after\n" {
		t.Fatalf("real edit did not change file: %q %v", data, err)
	}
	mu.Lock()
	read, edited := observedRead, observedEdit
	mu.Unlock()
	if !read || !edited {
		t.Fatal("provider did not receive the real tool results")
	}
	messages, err := api.Messages(ctx, ws.ID, remote.ID)
	if err != nil {
		t.Fatal(err)
	}
	var savedRead, savedEdit bool
	for _, message := range messages {
		for _, call := range engineapi.ExtractParts(message.Parts).ToolCalls {
			savedRead = savedRead || call.Name == "read"
			savedEdit = savedEdit || call.Name == "edit"
		}
	}
	if !savedRead || !savedEdit {
		t.Fatal("executed tool calls were not persisted")
	}
	visibleMessages, err := api.Messages(ctx, ws.ID, visible.ID)
	if err != nil || len(visibleMessages) != 0 {
		t.Fatalf("remote run leaked into selected desktop session: %d %v", len(visibleMessages), err)
	}
}
