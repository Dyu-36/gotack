//go:build e2e

package inputpipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "--gotack-e2e-mcp" {
		if serveMCP(os.Stdin, os.Stdout, auditMCP(os.Args[2])) != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	// Direct go test is also fail-closed; the wrapper is not the only guard.
	if runtime.GOOS != "windows" {
		fmt.Fprintln(os.Stderr, "input-pipeline: required_platform_windows")
		os.Exit(1)
	}
	root, node := os.Getenv("TACK_E2E_REPO_ROOT"), os.Getenv("TACK_E2E_NODE")
	binary, provenance := os.Getenv("TACK_ENGINE_BINARY"), os.Getenv("TACK_ENGINE_PROVENANCE")
	if !filepath.IsAbs(root) || !filepath.IsAbs(node) || !filepath.IsAbs(provenance) || requireBinary(binary) != nil {
		fmt.Fprintln(os.Stderr, "input-pipeline: explicit_artifacts_required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cmd := exec.CommandContext(ctx, node, filepath.Join(root, "scripts", "input-pipeline", "run.mjs"),
		"--skip-build", "--verify-only", "--binary", binary, "--provenance", provenance)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	err := cmd.Run()
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "input-pipeline: provenance_verification_failed")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func newID(t testing.TB) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal("random_id_unavailable")
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func must(t testing.TB, err error, code string) {
	t.Helper()
	if err != nil {
		t.Fatal(code)
	} // Never echo REST paths, IDs, provider payloads, or child stderr.
}

type engineHarness struct {
	client    *crushapi.Client
	events    <-chan crushapi.StreamEvent
	workspace string
	ctx       context.Context
	stop      func()
}

func isolatedEnv(t testing.TB, root, proxy string) []string {
	t.Helper()
	system := os.Getenv("SystemRoot")
	if system == "" {
		system = os.Getenv("SYSTEMROOT")
	}
	if !filepath.IsAbs(system) {
		t.Fatal("windows_system_root_missing")
	}
	values := map[string]string{
		"SystemRoot": system, "WINDIR": system, "SystemDrive": filepath.VolumeName(system),
		"COMSPEC": filepath.Join(system, "System32", "cmd.exe"), "PATHEXT": ".COM;.EXE;.BAT;.CMD",
		"PATH": filepath.Join(system, "System32") + ";" + system,
		"HOME": root, "USERPROFILE": root, "HOMEDRIVE": filepath.VolumeName(root),
		"HOMEPATH":                           strings.TrimPrefix(root, filepath.VolumeName(root)),
		"CRUSH_DISABLE_PROVIDER_AUTO_UPDATE": "1", "CRUSH_DISABLE_METRICS": "1",
		"CATWALK_URL": proxy, "HTTP_PROXY": proxy, "HTTPS_PROXY": proxy, "ALL_PROXY": proxy,
		"NO_PROXY": "127.0.0.1,localhost", "NO_COLOR": "1", "TERM": "dumb", "AWS_EC2_METADATA_DISABLED": "true",
	}
	for key, dir := range map[string]string{
		"APPDATA": "config", "LOCALAPPDATA": "local", "XDG_CONFIG_HOME": "config", "XDG_DATA_HOME": "global-data",
		"XDG_CACHE_HOME": "cache", "CRUSH_GLOBAL_CONFIG": "global-config", "CRUSH_GLOBAL_DATA": "global-data",
		"TEMP": "temp", "TMP": "temp", "TMPDIR": "temp",
	} {
		values[key] = filepath.Join(root, dir)
		must(t, os.MkdirAll(values[key], 0o700), "isolated_directory_failed")
	}
	env := make([]string, 0, len(values))
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}
func writeFixtureConfig(t testing.TB, root string, p *fakeProvider, withMCP bool, modelOptions ...map[string]any) {
	t.Helper()
	model := func(id string) map[string]any {
		return map[string]any{"id": id, "name": id, "context_window": 200000, "default_max_tokens": 4096}
	}
	main := model(mainModel)
	if len(modelOptions) > 0 && modelOptions[0] != nil {
		main["provider_options"] = modelOptions[0]
	}
	config := map[string]any{
		"providers": map[string]any{"e2e": map[string]any{"name": "Synthetic E2E", "type": "openai",
			"base_url": p.server.URL + "/v1", "api_key": "synthetic-test-key", "discover_models": false,
			"models": []any{main, model(titleModel)}}},
		"models": map[string]any{"large": map[string]any{"provider": "e2e", "model": mainModel},
			"small": map[string]any{"provider": "e2e", "model": titleModel}},
		"options": map[string]any{"disable_metrics": true, "disable_provider_auto_update": true,
			"disable_default_providers": true, "disable_auto_summarize": true},
	}
	if withMCP {
		executable, err := os.Executable()
		must(t, err, "mcp_fixture_executable_missing")
		config["mcp"] = map[string]any{"e2e": map[string]any{"type": "stdio", "command": executable,
			"args": []string{"--gotack-e2e-mcp", filepath.Join(root, "mcp-audit.txt")}, "timeout": 10}}
	}
	data, err := json.Marshal(config)
	must(t, err, "fixture_config_encoding_failed")
	must(t, os.WriteFile(filepath.Join(root, "global-config", "crush.json"), data, 0o600), "fixture_config_write_failed")
}
func startEngine(t testing.TB, root string, p *fakeProvider, withMCP bool, modelOptions ...map[string]any) *engineHarness {
	t.Helper()
	env := isolatedEnv(t, root, p.proxy.URL)
	writeFixtureConfig(t, root, p, withMCP, modelOptions...)
	workspace, db := filepath.Join(root, "workspace"), filepath.Join(root, "workspace-data")
	must(t, os.MkdirAll(workspace, 0o700), "workspace_directory_failed")
	pipe := `\\.\pipe\gotack-e2e-` + newID(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Getenv("TACK_ENGINE_BINARY"), "server", "--host", "npipe://"+pipe,
		"--data-dir", filepath.Join(root, "server-data"))
	cmd.Dir, cmd.Env = workspace, env
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	cmd.WaitDelay = 5 * time.Second
	must(t, cmd.Start(), "engine_start_failed")
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	var once sync.Once
	var hc *http.Client
	var detach func()
	stop := func() {
		once.Do(func() {
			if detach != nil {
				detach()
			}
			select {
			case <-done:
			default:
				killCtx, killCancel := context.WithTimeout(context.Background(), 10*time.Second)
				killer := exec.CommandContext(killCtx, filepath.Join(os.Getenv("SystemRoot"), "System32", "taskkill.exe"),
					"/PID", fmt.Sprint(cmd.Process.Pid), "/T", "/F")
				killer.Stdout, killer.Stderr = io.Discard, io.Discard
				killErr := killer.Run()
				killCancel()
				if killErr != nil {
					t.Error("engine_tree_cleanup_unverified")
					_ = cmd.Process.Kill()
				}
			}
			cancel()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("engine_cleanup_timeout")
			}
			if hc != nil {
				hc.CloseIdleConnections()
			}
		})
	}
	t.Cleanup(stop)
	var err error
	hc, err = crushapi.Dial(ctx, crushapi.Endpoint{Network: "npipe", Address: pipe})
	must(t, err, "pipe_transport_failed")
	readyCtx, readyCancel := context.WithTimeout(ctx, 20*time.Second)
	err = waitReady(readyCtx, func(probeCtx context.Context) error {
		select {
		case <-done:
			return errors.New("engine_exited")
		default:
		}
		return crushapi.Ping(hc, probeCtx)
	})
	readyCancel()
	must(t, err, "engine_not_ready")
	client := crushapi.NewClient(hc)
	ws, err := client.CreateWorkspaceWithDataDir(ctx, workspace, db, true)
	must(t, err, "workspace_create_failed")
	if ws.ID == "" {
		t.Fatal("workspace_id_missing")
	}
	events, stopStream, err := client.Stream(ctx, ws.ID, "run_complete")
	must(t, err, "sse_attach_failed")
	detach = stopStream
	must(t, client.InitAgent(ctx, ws.ID, false), "agent_init_failed")
	must(t, client.SetPermissionsSkip(ctx, ws.ID, true), "fixture_permission_failed")
	return &engineHarness{client: client, events: events, workspace: ws.ID, ctx: ctx, stop: stop}
}
func freshSession(t testing.TB, h *engineHarness) string {
	t.Helper()
	session, err := h.client.CreateSession(h.ctx, h.workspace, "synthetic-e2e")
	must(t, err, "session_create_failed")
	if session.ID == "" {
		t.Fatal("session_id_missing")
	}
	must(t, h.client.SetCurrentSession(h.ctx, h.workspace, session.ID), "current_session_failed")
	return session.ID
}
func sendTurn(t testing.TB, h *engineHarness, p *fakeProvider, session string, mode providerMode) (terminalPayload, captureCounts) {
	t.Helper()
	run := newID(t)
	p.arm(run, mode)
	must(t, h.client.SendPromptWithAttachments(h.ctx, h.workspace, session, "Run the synthetic fixture.", run, nil), "prompt_submit_failed")
	terminal, err := waitTerminal(h.ctx, func(ctx context.Context) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-h.events:
			if !ok {
				return nil, errors.New("stream_closed")
			}
			return event.Payload, nil
		}
	}, run, session)
	must(t, err, "matching_terminal_missing")
	counts := p.counts(run)
	t.Logf("e2e_evidence requests=%d retries=%d tool_responses=%d tool_results=%d invalid=%d", counts.Requests, counts.Retries, counts.ToolResponses, counts.ToolResults, counts.Invalid)
	t.Logf("e2e_rejections route=%d auth=%d schema=%d", counts.RejectedRoute, counts.RejectedAuth, counts.RejectedSchema)
	must(t, checkCapture(counts, 1), "provider_capture_invalid")
	return terminal, counts
}
func success(t *testing.T, terminal terminalPayload) {
	t.Helper()
	if terminal.Error != "" || terminal.Cancelled || terminal.Text != fixtureAnswer {
		t.Fatal("terminal_outcome_invalid")
	}
}
func TestE2EInputPipelineFreshTurn(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	h := startEngine(t, t.TempDir(), p, false)
	terminal, c := sendTurn(t, h, p, freshSession(t, h), modeText)
	success(t, terminal)
	if c.Requests != 1 {
		t.Fatal("fresh_turn_request_count_invalid")
	}
}
func TestE2ERetryBehavior(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	h := startEngine(t, t.TempDir(), p, false)
	terminal, c := sendTurn(t, h, p, freshSession(t, h), modeRetry)
	success(t, terminal)
	if c.Requests != 2 || c.Retries != 1 {
		t.Fatal("retry_count_invalid")
	}
}
func TestE2EToolLoopAndRestart(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	root := t.TempDir()
	h := startEngine(t, root, p, true)
	session := freshSession(t, h)
	terminal, c := sendTurn(t, h, p, session, modeTool)
	success(t, terminal)
	if c.Requests != 2 || c.ToolResponses != 1 || c.ToolResults != 1 {
		t.Fatal("tool_loop_count_invalid")
	}
	h.stop()
	audit, err := os.ReadFile(filepath.Join(root, "mcp-audit.txt"))
	must(t, err, "mcp_audit_missing")
	if strings.Count(string(audit), "call\n") != 1 || !strings.Contains(string(audit), "initialize\n") {
		t.Fatal("mcp_call_not_proven")
	}
	h = startEngine(t, root, p, true)
	messages, err := h.client.Messages(h.ctx, h.workspace, session)
	must(t, err, "restart_history_read_failed")
	if len(messages) == 0 {
		t.Fatal("restart_history_missing")
	}
	must(t, h.client.SetCurrentSession(h.ctx, h.workspace, session), "restart_current_session_failed")
	terminal, c = sendTurn(t, h, p, session, modeReplay)
	success(t, terminal)
	if c.Requests != 1 || c.ToolResults != 1 {
		t.Fatal("restart_replay_invalid")
	}
	h.stop()
	audit, err = os.ReadFile(filepath.Join(root, "mcp-audit.txt"))
	must(t, err, "restart_mcp_audit_missing")
	if strings.Count(string(audit), "call\n") != 1 {
		t.Fatal("restart_duplicated_tool_execution")
	}
}
func TestE2ERejectMalformedProvider(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	h := startEngine(t, t.TempDir(), p, false)
	terminal, _ := sendTurn(t, h, p, freshSession(t, h), modeMalformed)
	if terminal.Error == "" || terminal.Text != "" || terminal.Cancelled {
		t.Fatal("malformed_stream_false_pass")
	}
}

// The provider-option merge must union user configuration with the required
// encrypted-reasoning include instead of clobbering it, and an explicit user
// reasoning summary must reach the wire verbatim.
func TestE2EProviderOptionsPreserved(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	options := map[string]any{"openai": map[string]any{
		"include":           []any{"file_search_call.results"},
		"reasoning_summary": "concise",
	}}
	h := startEngine(t, t.TempDir(), p, false, options)
	terminal, c := sendTurn(t, h, p, freshSession(t, h), modeText)
	success(t, terminal)
	if len(c.Include) != 2 ||
		c.Include[0] != "file_search_call.results" || c.Include[1] != "reasoning.encrypted_content" {
		t.Fatal("provider_options_include_clobbered")
	}
	if c.ReasoningSummary != "concise" {
		t.Fatal("provider_options_summary_clobbered")
	}
}

// Invalid Responses options must fail model preparation before any network
// attempt: the terminal reports an error and the provider saw zero requests.
func TestE2EInvalidOptionsPreNetwork(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	options := map[string]any{"openai": map[string]any{"reasoning_summary": "none"}}
	h := startEngine(t, t.TempDir(), p, false, options)
	run := newID(t)
	p.arm(run, modeText)
	session := freshSession(t, h)
	must(t, h.client.SendPromptWithAttachments(h.ctx, h.workspace, session, "Run the synthetic fixture.", run, nil), "prompt_submit_failed")
	terminal, err := waitTerminal(h.ctx, func(ctx context.Context) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-h.events:
			if !ok {
				return nil, errors.New("stream_closed")
			}
			return event.Payload, nil
		}
	}, run, session)
	must(t, err, "matching_terminal_missing")
	if terminal.Error == "" || terminal.Cancelled || terminal.Text != "" {
		t.Fatal("invalid_options_not_rejected")
	}
	if c := p.counts(run); c.Requests != 0 {
		t.Fatal("invalid_options_reached_network")
	}
}

// The todo reminder must reflect the session's real state at the next model
// call after tool updates, survive a restart from persistence, and never be
// persisted as a synthetic transcript message.
func TestE2ETodoReminderReflectsState(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	root := t.TempDir()
	h := startEngine(t, root, p, false)
	session := freshSession(t, h)
	terminal, c := sendTurn(t, h, p, session, modeTodo)
	success(t, terminal)
	if c.Requests != 2 || c.ToolResponses != 1 || c.ToolResults != 1 {
		t.Fatal("todo_tool_loop_count_invalid")
	}
	if !strings.Contains(c.LastInputText, "synthetic-task-alpha") ||
		!strings.Contains(c.LastInputText, "synthetic-task-beta") {
		t.Fatal("todo_reminder_missing_real_state")
	}
	if strings.Contains(c.LastInputText, "currently empty") {
		t.Fatal("todo_reminder_false_empty")
	}
	h.stop()

	h = startEngine(t, root, p, false)
	must(t, h.client.SetCurrentSession(h.ctx, h.workspace, session), "restart_current_session_failed")
	run := newID(t)
	p.arm(run, modeText)
	must(t, h.client.SendPromptWithAttachments(h.ctx, h.workspace, session, "Run the synthetic fixture.", run, nil), "restart_prompt_submit_failed")
	terminal, err := waitTerminal(h.ctx, func(ctx context.Context) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-h.events:
			if !ok {
				return nil, errors.New("stream_closed")
			}
			return event.Payload, nil
		}
	}, run, session)
	must(t, err, "restart_matching_terminal_missing")
	success(t, terminal)
	c = p.counts(run)
	if !strings.Contains(c.LastInputText, "synthetic-task-alpha") ||
		!strings.Contains(c.LastInputText, "synthetic-task-beta") {
		t.Fatal("todo_state_lost_after_restart")
	}
	h.stop()
	messages, err := h.client.Messages(h.ctx, h.workspace, session)
	must(t, err, "restart_history_read_failed")
	if len(messages) == 0 {
		t.Fatal("restart_history_missing")
	}
	transcript, err := json.Marshal(messages)
	must(t, err, "restart_history_encode_failed")
	if strings.Contains(string(transcript), "system_reminder") ||
		strings.Contains(string(transcript), "currently empty") {
		t.Fatal("synthetic_reminder_persisted_to_transcript")
	}
}
