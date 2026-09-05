package inputpipeline

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const fixtureAnswer = "fixture-answer"
const fixtureToolOutput = "fixture-ok"
const fixtureCallID = "call_fixture_echo"
const mainModel = "gpt-5-fixture-main"
const titleModel = "gpt-5-fixture-title"

type providerMode string

const (
	modeText      providerMode = "text"
	modeRetry     providerMode = "retry"
	modeTool      providerMode = "tool"
	modeReplay    providerMode = "replay"
	modeMalformed providerMode = "malformed"
)

type captureCounts struct {
	Requests       int
	Retries        int
	ToolResponses  int
	ToolResults    int
	Invalid        int
	RejectedRoute  int
	RejectedAuth   int
	RejectedSchema int
}

type fakeProvider struct {
	server         *httptest.Server
	proxy          *httptest.Server
	mu             sync.Mutex
	active         string
	mode           providerMode
	runs           map[string]captureCounts
	auxiliary      int
	serial         int
	rejected       int
	rejectedRoute  int
	rejectedAuth   int
	rejectedSchema int
}

func newFakeProvider() *fakeProvider {
	p := &fakeProvider{runs: make(map[string]captureCounts)}
	p.server = httptest.NewServer(http.HandlerFunc(p.serve))
	p.proxy = httptest.NewServer(http.HandlerFunc(p.denyEgress))
	return p
}

// The pinned app unconditionally checks GitHub releases in the background.
// Deny that known auxiliary CONNECT without classifying it as a malformed model
// request. All other egress is a fixture failure; no proxy request is forwarded.
func (p *fakeProvider) denyEgress(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.Method != http.MethodConnect || r.Host != "api.github.com:443" {
		p.rejected++
		p.rejectedRoute++
	}
	w.WriteHeader(http.StatusForbidden)
}

func (p *fakeProvider) close() {
	p.server.Close()
	p.proxy.Close()
}
func (p *fakeProvider) arm(run string, mode providerMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active, p.mode = run, mode
	p.runs[run] = captureCounts{}
}
func (p *fakeProvider) counts(run string) captureCounts {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.runs[run]
	c.Invalid += p.rejected
	c.RejectedRoute = p.rejectedRoute
	c.RejectedAuth = p.rejectedAuth
	c.RejectedSchema = p.rejectedSchema
	return c
}

type responseRequest struct {
	Model  string                       `json:"model"`
	Stream bool                         `json:"stream"`
	Input  []map[string]json.RawMessage `json:"input"`
	Tools  []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"tools"`
}

func jsonString(value json.RawMessage) string {
	var s string
	_ = json.Unmarshal(value, &s)
	return s
}
func validateRequest(data []byte) (responseRequest, error) {
	var req responseRequest
	if json.Unmarshal(data, &req) != nil || len(req.Input) == 0 ||
		(req.Model != mainModel && req.Model != titleModel) ||
		(req.Model == mainModel && !req.Stream) {
		return req, errors.New("provider_schema_invalid")
	}
	for _, item := range req.Input {
		kind := jsonString(item["type"])
		role := jsonString(item["role"])
		if kind == "" && role == "" {
			return req, errors.New("provider_input_invalid")
		}
	}
	return req, nil
}
func hasToolResult(req responseRequest) bool {
	callSeen := false
	for _, item := range req.Input {
		if jsonString(item["call_id"]) != fixtureCallID {
			continue
		}
		switch jsonString(item["type"]) {
		case "function_call":
			callSeen = true
		case "function_call_output":
			if callSeen && strings.Contains(jsonString(item["output"]), fixtureToolOutput) {
				return true
			}
		}
	}
	return false
}
func rejectProvider(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"fixture_schema_invalid"}}`)
}
func (p *fakeProvider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// This also rejects proxy traffic: the harness never forwards it upstream.
	if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" || r.URL.IsAbs() {
		p.rejectedRoute++
		p.rejected++
		rejectProvider(w)
		return
	}
	if r.Header.Get("Authorization") != "Bearer synthetic-test-key" {
		p.rejectedAuth++
		p.rejected++
		rejectProvider(w)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	req, schemaErr := validateRequest(body)
	if err != nil || schemaErr != nil {
		p.rejectedSchema++
		p.rejected++
		rejectProvider(w)
		return
	}
	if req.Model == titleModel {
		p.auxiliary++
		writeResponse(w, req.Stream, fmt.Sprintf("title_%d", p.auxiliary), "", "fixture-title")
		return
	}
	if p.active == "" {
		p.rejected++
		rejectProvider(w)
		return
	}
	c := p.runs[p.active]
	c.Requests++
	p.serial++
	defer func() { p.runs[p.active] = c }()
	if p.mode == modeRetry && c.Requests == 1 {
		c.Retries++
		w.Header().Set("Retry-After", "0")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"type":"rate_limit_error","message":"synthetic_retry"}}`)
		return
	}
	if p.mode == modeMalformed {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.created\ndata: {invalid-json\n\n")
		w.(http.Flusher).Flush()
		return
	}
	toolName := ""
	if p.mode == modeTool && c.Requests == 1 {
		for _, tool := range req.Tools {
			if tool.Type == "function" && strings.HasSuffix(tool.Name, "fixture_echo") {
				toolName = tool.Name
				break
			}
		}
		if toolName == "" {
			c.Invalid++
			rejectProvider(w)
			return
		}
		c.ToolResponses++
	} else if p.mode == modeTool || p.mode == modeReplay {
		if !hasToolResult(req) {
			c.Invalid++
			rejectProvider(w)
			return
		}
		c.ToolResults++
	}
	writeResponse(w, true, fmt.Sprintf("main_%d", p.serial), toolName, fixtureAnswer)
}

// The fake exercises the actual Responses event lifecycle, not chat completions
// or a one-shot JSON body masquerading as an SSE stream.
func writeResponse(w http.ResponseWriter, stream bool, suffix, toolName, text string) {
	model := mainModel
	if strings.HasPrefix(suffix, "title") {
		model = titleModel
	}
	itemID := "msg_" + suffix
	part := map[string]any{"type": "output_text", "text": text, "annotations": []any{}, "logprobs": []any{}}
	item := map[string]any{"id": itemID, "type": "message", "role": "assistant", "status": "completed", "content": []any{part}}
	if toolName != "" {
		itemID = "fc_" + suffix
		item = map[string]any{"id": itemID, "type": "function_call", "call_id": fixtureCallID,
			"name": toolName, "arguments": "{}", "status": "completed"}
	}
	response := map[string]any{"id": "resp_" + suffix, "object": "response", "created_at": 1,
		"model": model, "status": "completed", "error": nil, "incomplete_details": nil,
		"output": []any{item}, "usage": map[string]any{"input_tokens": 10, "output_tokens": 5,
			"total_tokens": 15, "input_tokens_details": map[string]any{"cached_tokens": 0},
			"output_tokens_details": map[string]any{"reasoning_tokens": 0}}}
	if !stream {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	sequence := 0
	emit := func(kind string, fields map[string]any) {
		fields["type"], fields["sequence_number"] = kind, sequence
		sequence++
		data, _ := json.Marshal(fields)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, data)
		w.(http.Flusher).Flush()
	}
	start := map[string]any{"id": response["id"], "object": "response", "created_at": 1,
		"model": model, "status": "in_progress", "output": []any{}, "error": nil}
	emit("response.created", map[string]any{"response": start})
	emit("response.in_progress", map[string]any{"response": start})
	added := make(map[string]any)
	for k, v := range item {
		added[k] = v
	}
	added["status"] = "in_progress"
	if toolName == "" {
		added["content"] = []any{}
	} else {
		added["arguments"] = ""
	}
	emit("response.output_item.added", map[string]any{"output_index": 0, "item": added})
	field := func() map[string]any { return map[string]any{"item_id": itemID, "output_index": 0, "content_index": 0} }
	if toolName == "" {
		f := field()
		f["part"] = map[string]any{"type": "output_text", "text": "", "annotations": []any{}, "logprobs": []any{}}
		emit("response.content_part.added", f)
		f = field()
		f["delta"] = text
		f["logprobs"] = []any{}
		emit("response.output_text.delta", f)
		f = field()
		f["text"] = text
		f["logprobs"] = []any{}
		emit("response.output_text.done", f)
		f = field()
		f["part"] = part
		emit("response.content_part.done", f)
	} else {
		f := field()
		f["delta"] = "{}"
		emit("response.function_call_arguments.delta", f)
		f = field()
		f["arguments"] = "{}"
		f["name"] = toolName
		emit("response.function_call_arguments.done", f)
	}
	emit("response.output_item.done", map[string]any{"output_index": 0, "item": item})
	emit("response.completed", map[string]any{"response": response})
}

// JSON-RPC stdout only. Audit records contain no arguments, IDs, or tool output.
func serveMCP(in io.Reader, out io.Writer, audit func(string) error) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	enc := json.NewEncoder(out)
	for scanner.Scan() {
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &req) != nil || req.JSONRPC != "2.0" {
			return errors.New("mcp_request_invalid")
		}
		if len(req.ID) == 0 {
			continue
		}
		var result any
		switch req.Method {
		case "initialize":
			var params struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if json.Unmarshal(req.Params, &params) != nil || params.ProtocolVersion == "" {
				return errors.New("mcp_initialize_invalid")
			}
			if err := audit("initialize"); err != nil {
				return err
			}
			result = map[string]any{"protocolVersion": params.ProtocolVersion,
				"capabilities": map[string]any{"tools": map[string]any{"listChanged": false}},
				"serverInfo":   map[string]any{"name": "gotack-e2e", "version": "1"}, "instructions": "Synthetic fixture tools only."}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "fixture_echo", "description": "Return a fixed synthetic value.",
				"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}}}}
		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if json.Unmarshal(req.Params, &params) != nil || params.Name != "fixture_echo" || len(params.Arguments) != 0 {
				return errors.New("mcp_call_invalid")
			}
			if err := audit("call"); err != nil {
				return err
			}
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": fixtureToolOutput}}, "isError": false}
		case "ping":
			result = map[string]any{}
		case "resources/list":
			result = map[string]any{"resources": []any{}}
		case "resources/templates/list":
			result = map[string]any{"resourceTemplates": []any{}}
		case "prompts/list":
			result = map[string]any{"prompts": []any{}}
		default:
			if enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "method_not_found"}}) != nil {
				return errors.New("mcp_write_failed")
			}
			continue
		}
		if enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}) != nil {
			return errors.New("mcp_write_failed")
		}
	}
	if scanner.Err() != nil {
		return errors.New("mcp_read_failed")
	}
	return nil
}
func auditMCP(filename string) func(string) error {
	return func(kind string) error {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return errors.New("mcp_audit_failed")
		}
		_, writeErr := fmt.Fprintln(f, kind)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			return errors.New("mcp_audit_failed")
		}
		return nil
	}
}
func requireBinary(filename string) error {
	if !filepath.IsAbs(filename) {
		return errors.New("binary_absolute_required")
	}
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("binary_missing")
	}
	return nil
}
func waitReady(ctx context.Context, probe func(context.Context) error) error {
	timer := time.NewTicker(25 * time.Millisecond)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return errors.New("endpoint_not_ready")
		}
		attempt, cancel := context.WithTimeout(ctx, time.Second)
		err := probe(attempt)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("endpoint_not_ready")
		case <-timer.C:
		}
	}
}
func checkCapture(c captureCounts, minRequests int) error {
	if c.Requests < minRequests {
		return errors.New("provider_capture_missing")
	}
	if c.Invalid != 0 {
		return errors.New("provider_schema_invalid")
	}
	return nil
}
func readCompleteFrames(data []byte) ([]string, error) {
	// Used to validate the fixture itself; incomplete last frames must not count.
	frames := bytes.Split(data, []byte("\n\n"))
	if len(frames) < 2 || len(frames[len(frames)-1]) != 0 {
		return nil, errors.New("sse_incomplete")
	}
	var kinds []string
	for _, frame := range frames[:len(frames)-1] {
		var raw []byte
		for _, line := range bytes.Split(frame, []byte("\n")) {
			if bytes.HasPrefix(line, []byte("data: ")) {
				raw = line[6:]
			}
		}
		var e struct {
			Type     string `json:"type"`
			Sequence int    `json:"sequence_number"`
		}
		if json.Unmarshal(raw, &e) != nil || e.Type == "" || e.Sequence != len(kinds) {
			return nil, errors.New("sse_schema_invalid")
		}
		kinds = append(kinds, e.Type)
	}
	if len(kinds) < 2 || kinds[0] != "response.created" || kinds[len(kinds)-1] != "response.completed" {
		return nil, errors.New("sse_terminal_missing")
	}
	return kinds, nil
}

type terminalPayload struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
	Text      string `json:"text"`
	Error     string `json:"error"`
	Cancelled bool   `json:"cancelled"`
}

func waitTerminal(ctx context.Context, next func(context.Context) ([]byte, error), run, session string) (terminalPayload, error) {
	for {
		data, err := next(ctx)
		if err != nil {
			return terminalPayload{}, errors.New("terminal_missing")
		}
		var terminal terminalPayload
		if json.Unmarshal(data, &terminal) != nil {
			return terminal, errors.New("terminal_schema_invalid")
		}
		if terminal.RunID == run && terminal.SessionID == session {
			return terminal, nil
		}
	}
}
