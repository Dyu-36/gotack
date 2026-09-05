package inputpipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func expectCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || err.Error() != code {
		t.Fatalf("expected diagnostic %s", code)
	}
}
func TestHarnessNegativeControls(t *testing.T) {
	t.Run("missing_binary", func(t *testing.T) {
		expectCode(t, requireBinary(filepath.Join(t.TempDir(), "missing.exe")), "binary_missing")
		expectCode(t, requireBinary("missing.exe"), "binary_absolute_required")
		file := filepath.Join(t.TempDir(), "present.exe")
		if os.WriteFile(file, []byte("fixture"), 0o600) != nil {
			t.Fatal("fixture_write_failed")
		}
		if requireBinary(file) != nil {
			t.Fatal("valid_binary_path_rejected")
		}
	})
	t.Run("endpoint_never_ready", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		calls := 0
		err := waitReady(ctx, func(context.Context) error { calls++; return errors.New("not_ready") })
		expectCode(t, err, "endpoint_not_ready")
		if calls == 0 {
			t.Fatal("negative_probe_not_exercised")
		}
		if waitReady(context.Background(), func(context.Context) error { return nil }) != nil {
			t.Fatal("ready_probe_rejected")
		}
	})
	t.Run("zero_captures", func(t *testing.T) {
		expectCode(t, checkCapture(captureCounts{}, 1), "provider_capture_missing")
		expectCode(t, checkCapture(captureCounts{Requests: 1, Invalid: 1}, 1), "provider_schema_invalid")
		if checkCapture(captureCounts{Requests: 1}, 1) != nil {
			t.Fatal("valid_capture_rejected")
		}
	})
	t.Run("malformed_provider_schema", func(t *testing.T) {
		_, err := validateRequest([]byte(`{"model":"gpt-5-fixture-main","stream":true,"input":[]}`))
		expectCode(t, err, "provider_schema_invalid")
		_, err = readCompleteFrames([]byte("data: not-json\n\n"))
		expectCode(t, err, "sse_schema_invalid")
	})
	t.Run("dropped_terminal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_, err := waitTerminal(ctx, func(ctx context.Context) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }, "run", "session")
		expectCode(t, err, "terminal_missing")
		data := []byte(`{"run_id":"run","session_id":"session","text":"ok"}`)
		result, err := waitTerminal(context.Background(), func(context.Context) ([]byte, error) { return data, nil }, "run", "session")
		if err != nil || result.Text != "ok" {
			t.Fatal("valid_terminal_rejected")
		}
	})
}
func TestResponseFixtureLifecycle(t *testing.T) {
	for _, tool := range []string{"", "mcp_e2e_fixture_echo"} {
		w := httptest.NewRecorder()
		writeResponse(w, true, "test", tool, fixtureAnswer)
		events, err := readCompleteFrames(w.Body.Bytes())
		if err != nil || len(events) < 7 || !w.Flushed {
			t.Fatal("fixture_lifecycle_invalid")
		}
		raw := w.Body.Bytes()
		last := bytes.LastIndex(raw, []byte("event: response.completed"))
		_, err = readCompleteFrames(raw[:last])
		expectCode(t, err, "sse_terminal_missing")
		_, err = readCompleteFrames(raw[:len(raw)-1])
		expectCode(t, err, "sse_incomplete")
	}
}
func TestFakeProviderActuallyCapturesAndRetries(t *testing.T) {
	p := newFakeProvider()
	defer p.close()
	p.arm("unit", modeRetry)
	hc := &http.Client{Timeout: time.Second}
	for i := 0; i < 2; i++ {
		body := `{"model":"gpt-5-fixture-main","stream":true,"input":[{"role":"user","content":"synthetic"}]}`
		req, _ := http.NewRequest(http.MethodPost, p.server.URL+"/v1/responses", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer synthetic-test-key")
		resp, err := hc.Do(req)
		if err != nil {
			t.Fatal("fixture_request_failed")
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			t.Fatal("fixture_read_failed")
		}
		if i == 0 && resp.StatusCode != 429 {
			t.Fatal("fixture_retry_not_returned")
		}
		if i == 1 {
			if resp.StatusCode != 200 {
				t.Fatal("fixture_success_missing")
			}
			if _, err = readCompleteFrames(data); err != nil {
				t.Fatal("fixture_stream_invalid")
			}
		}
	}
	c := p.counts("unit")
	if c.Requests != 2 || c.Retries != 1 || c.Invalid != 0 {
		t.Fatal("fixture_count_invalid")
	}
}
func TestMCPProtocol(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"fixture_echo","arguments":{}}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	var audit []string
	err := serveMCP(strings.NewReader(input), &out, func(kind string) error { audit = append(audit, kind); return nil })
	if err != nil || strings.Join(audit, ",") != "initialize,call" {
		t.Fatal("mcp_protocol_failed")
	}
	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Fatal("mcp_notification_replied_or_response_missing")
	}
	for _, line := range lines {
		var response map[string]any
		if json.Unmarshal(line, &response) != nil || response["jsonrpc"] != "2.0" || response["result"] == nil {
			t.Fatal("mcp_stdout_not_protocol")
		}
	}
	if !bytes.Contains(lines[2], []byte(fixtureToolOutput)) {
		t.Fatal("mcp_result_missing")
	}
}
func TestToolResultOrder(t *testing.T) {
	call := map[string]json.RawMessage{"type": json.RawMessage(`"function_call"`), "call_id": json.RawMessage(`"call_fixture_echo"`)}
	result := map[string]json.RawMessage{"type": json.RawMessage(`"function_call_output"`), "call_id": json.RawMessage(`"call_fixture_echo"`), "output": json.RawMessage(`"fixture-ok"`)}
	if !hasToolResult(responseRequest{Input: []map[string]json.RawMessage{call, result}}) {
		t.Fatal("valid_tool_pair_rejected")
	}
	if hasToolResult(responseRequest{Input: []map[string]json.RawMessage{result, call}}) {
		t.Fatal("orphan_tool_result_accepted")
	}
}

func TestMCPSafeAuditFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "mcp-audit.txt")
	record := auditMCP(file)
	if record("initialize") != nil || record("call") != nil {
		t.Fatal("mcp_audit_failed")
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "initialize\ncall\n" {
		t.Fatal("mcp_audit_not_safe_counts")
	}
}
func TestAuxiliaryCallsCannotSatisfyCapture(t *testing.T) {
	p := newFakeProvider()
	defer p.close()
	p.arm("unit", modeText)
	req, _ := http.NewRequest(http.MethodPost, p.server.URL+"/v1/responses", strings.NewReader(
		`{"model":"gpt-5-fixture-title","stream":false,"input":[{"role":"user","content":"synthetic"}]}`))
	req.Header.Set("Authorization", "Bearer synthetic-test-key")
	hc := &http.Client{Timeout: time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal("title_fixture_request_failed")
	}
	defer resp.Body.Close()
	var body struct {
		Model string `json:"model"`
	}
	if json.NewDecoder(resp.Body).Decode(&body) != nil || body.Model != titleModel {
		t.Fatal("title_fixture_response_invalid")
	}
	expectCode(t, checkCapture(p.counts("unit"), 1), "provider_capture_missing")
}
func TestFakeMalformedStream(t *testing.T) {
	p := newFakeProvider()
	defer p.close()
	p.arm("unit", modeMalformed)
	req, _ := http.NewRequest(http.MethodPost, p.server.URL+"/v1/responses", strings.NewReader(
		`{"model":"gpt-5-fixture-main","stream":true,"input":[{"role":"user","content":"synthetic"}]}`))
	req.Header.Set("Authorization", "Bearer synthetic-test-key")
	hc := &http.Client{Timeout: time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal("malformed_fixture_request_failed")
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal("malformed_fixture_read_failed")
	}
	_, err = readCompleteFrames(data)
	expectCode(t, err, "sse_schema_invalid")
	if p.counts("unit").Requests != 1 {
		t.Fatal("malformed_fixture_not_exercised")
	}
}

func TestEgressGuardSeparatesDeniedUpdateFromProviderTraffic(t *testing.T) {
	p := newFakeProvider()
	defer p.close()
	p.arm("unit", modeText)
	for _, probe := range []struct {
		method, host string
		invalid      int
	}{
		{http.MethodConnect, "api.github.com:443", 0},
		{http.MethodConnect, "unapproved.invalid:443", 1},
		{http.MethodGet, "api.github.com:443", 2},
	} {
		req := httptest.NewRequest(probe.method, "http://"+probe.host, nil)
		req.Host = probe.host
		response := httptest.NewRecorder()
		p.denyEgress(response, req)
		if response.Code != http.StatusForbidden || p.counts("unit").Invalid != probe.invalid {
			t.Fatal("egress_guard_policy_invalid")
		}
		if p.counts("unit").Requests != 0 {
			t.Fatal("egress_satisfied_provider_capture")
		}
	}
}
