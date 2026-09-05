//go:build e2e

package inputpipeline

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// sendCustomPromptTurn submits a caller-supplied prompt and waits for the
// matching terminal event. The standard sendTurn uses a constant prompt;
// compaction proofs need per-turn distinct prompts.
func sendCustomPromptTurn(t testing.TB, h *engineHarness, p *fakeProvider, session string, mode providerMode, prompt string) (terminalPayload, captureCounts) {
	t.Helper()
	run := newID(t)
	p.arm(run, mode)
	must(t, h.client.SendPromptWithAttachments(h.ctx, h.workspace, session, prompt, run, nil), "prompt_submit_failed")
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
	must(t, checkCapture(counts, 1), "provider_capture_invalid")
	return terminal, counts
}

// TestE2ECanonicalPromptAcrossRestarts proves the rendered prompt is
// canonical across at least 20 process restarts (ImplementPlan section 2):
// the system-prompt bytes captured by the fake provider must be identical
// after every restart.
func TestE2ECanonicalPromptAcrossRestarts(t *testing.T) {
	if os.Getenv("TACK_E2E_SKIP_RESTARTS") == "1" {
		t.Fatal("unexpected_skip")
	}
	p := newFakeProvider()
	t.Cleanup(p.close)
	root := t.TempDir()
	h := startEngine(t, root, p, false)
	session := freshSession(t, h)
	terminal, c := sendTurn(t, h, p, session, modeText)
	success(t, terminal)
	reference := c.SystemPrompt
	if reference == "" {
		t.Fatal("canonical_prompt_capture_missing")
	}

	const restarts = 20
	for i := 0; i < restarts; i++ {
		h.stop()
		h = startEngine(t, root, p, false)
		must(t, h.client.SetCurrentSession(h.ctx, h.workspace, session), "restart_current_session_failed")
		terminal, c = sendTurn(t, h, p, session, modeText)
		success(t, terminal)
		if c.SystemPrompt != reference {
			t.Fatalf("restart %d changed the canonical prompt bytes", i+1)
		}
	}
}

// writeTwoMCPFixtureConfig writes a fixture config with two stdio MCP
// servers advertising distinct instructions. The server names are fixed;
// reverse only flips the physical order of the two JSON entries, so a
// sorted renderer must produce identical prompt bytes for both files.
func writeTwoMCPFixtureConfig(t testing.TB, root string, p *fakeProvider, reverse bool) {
	t.Helper()
	executable, err := os.Executable()
	must(t, err, "mcp_fixture_executable_missing")
	alpha := `{"type":"stdio","command":` + jsonStringArg(executable) +
		`,"args":[` + jsonStringArg("--gotack-e2e-mcp") + "," + jsonStringArg(filepath.ToSlash(filepath.Join(root, "mcp-alpha.txt"))) + "," + jsonStringArg("alpha") + `],"timeout":10}`
	bravo := `{"type":"stdio","command":` + jsonStringArg(executable) +
		`,"args":[` + jsonStringArg("--gotack-e2e-mcp") + "," + jsonStringArg(filepath.ToSlash(filepath.Join(root, "mcp-bravo.txt"))) + "," + jsonStringArg("bravo") + `],"timeout":10}`
	servers := `"mcp":{"e2e-alpha":` + alpha + `,"e2e-bravo":` + bravo + `}`
	if reverse {
		servers = `"mcp":{"e2e-bravo":` + bravo + `,"e2e-alpha":` + alpha + `}`
	}
	model := func(id string) map[string]any {
		return map[string]any{"id": id, "name": id, "context_window": 200000, "default_max_tokens": 4096}
	}
	config := map[string]any{
		"providers": map[string]any{"e2e": map[string]any{"name": "Synthetic E2E", "type": "openai",
			"base_url": p.server.URL + "/v1", "api_key": "synthetic-test-key", "discover_models": false,
			"models": []any{model(mainModel), model(titleModel)}}},
		"models": map[string]any{"large": map[string]any{"provider": "e2e", "model": mainModel},
			"small": map[string]any{"provider": "e2e", "model": titleModel}},
		"options": map[string]any{"disable_metrics": true, "disable_provider_auto_update": true,
			"disable_default_providers": true, "disable_auto_summarize": true},
	}
	data, err := json.Marshal(config)
	must(t, err, "fixture_config_encoding_failed")
	text := string(data[:len(data)-1]) + "," + servers + "}"
	must(t, os.MkdirAll(filepath.Join(root, "global-config"), 0o700), "fixture_config_dir_failed")
	must(t, os.WriteFile(filepath.Join(root, "global-config", "crush.json"), []byte(text), 0o600), "fixture_config_write_failed")
}

// jsonStringArg quotes one JSON string argument.
func jsonStringArg(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic("fixture_json_string_failed")
	}
	return string(data)
}

// TestE2EMCPInstructionOrderDeterministic proves the rendered MCP
// instruction order follows the canonical server-name order and never the
// config file order or connection race: reversing the config order across
// a restart must keep the instructions byte order identical.
func TestE2EMCPInstructionOrderDeterministic(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	root := t.TempDir()

	writeTwoMCPFixtureConfig(t, root, p, false)
	h := startEngineExistingConfig(t, root, p)
	session := freshSession(t, h)
	terminal, c := sendTurn(t, h, p, session, modeText)
	success(t, terminal)
	forward := c.SystemPrompt
	if !strings.Contains(forward, "alpha instructions") || !strings.Contains(forward, "bravo instructions") {
		t.Fatal("mcp_instructions_missing")
	}
	alphaIndex, bravoIndex := strings.Index(forward, "alpha instructions"), strings.Index(forward, "bravo instructions")
	if alphaIndex > bravoIndex {
		t.Fatal("mcp_instruction_order_not_canonical")
	}

	// Reverse the config order; the rendered order must not follow it.
	h.stop()
	writeTwoMCPFixtureConfig(t, root, p, true)
	h = startEngineExistingConfig(t, root, p)
	must(t, h.client.SetCurrentSession(h.ctx, h.workspace, session), "restart_current_session_failed")
	terminal, c = sendTurn(t, h, p, session, modeText)
	success(t, terminal)
	reversed := c.SystemPrompt
	if reversed != forward {
		t.Fatal("mcp_instruction_order_followed_config_order")
	}
}

// TestE2ECompactionPreservesLatestAnchorGroup proves the bounded PR5
// history-selection contract on the wire: after the auto-summarize
// boundary, the next request replays the committed summary plus the
// latest complete assistant anchor turn, and the compacted-away turns are
// gone.
func TestE2ECompactionPreservesLatestAnchorGroup(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	root := t.TempDir()

	// An 18-token context window yields a 15-token summarize limit; the
	// fixture usage (10 input + 5 output tokens per request) crosses it
	// exactly at the end of the first turn, so the first run stops and
	// performs the summarize request before returning.
	writeCompactionFixtureConfig(t, root, p)
	h := startEngineExistingConfig(t, root, p)
	session := freshSession(t, h)

	terminal, c := sendCustomPromptTurn(t, h, p, session, modeText, "alpha task one")
	success(t, terminal)
	if c.Requests != 2 {
		t.Fatal("compaction_summarize_request_missing")
	}

	// The post-compaction turn must carry the committed summary plus the
	// latest complete assistant anchor turn, and the compacted-away
	// prompt must be gone from the wire request.
	terminal, c = sendCustomPromptTurn(t, h, p, session, modeText, "beta task two")
	success(t, terminal)
	if strings.Contains(c.LastInputText, "alpha task one") {
		t.Fatal("compaction_evicted_history_still_replayed")
	}
	// The summary renders as a user-role message; the anchor assistant
	// turn replays before the current prompt.
	if !strings.Contains(c.LastInputText, fixtureAnswer) {
		t.Fatal("compaction_summary_or_anchor_missing")
	}
}

// writeCompactionFixtureConfig is the fixture config with auto-summarize
// enabled and a context window small enough for fixture usage to cross.
func writeCompactionFixtureConfig(t testing.TB, root string, p *fakeProvider) {
	t.Helper()
	model := func(id string, window int) map[string]any {
		return map[string]any{"id": id, "name": id, "context_window": window, "default_max_tokens": 4096}
	}
	config := map[string]any{
		"providers": map[string]any{"e2e": map[string]any{"name": "Synthetic E2E", "type": "openai",
			"base_url": p.server.URL + "/v1", "api_key": "synthetic-test-key", "discover_models": false,
			"models": []any{model(mainModel, 18), model(titleModel, 200000)}}},
		"models": map[string]any{"large": map[string]any{"provider": "e2e", "model": mainModel},
			"small": map[string]any{"provider": "e2e", "model": titleModel}},
		"options": map[string]any{"disable_metrics": true, "disable_provider_auto_update": true,
			"disable_default_providers": true, "disable_auto_summarize": false},
	}
	must(t, os.MkdirAll(filepath.Join(root, "global-config"), 0o700), "fixture_config_dir_failed")
	data, err := json.Marshal(config)
	must(t, err, "fixture_config_encoding_failed")
	must(t, os.WriteFile(filepath.Join(root, "global-config", "crush.json"), data, 0o600), "fixture_config_write_failed")
}

// TestE2EPromptCanaryStaysInAllowedSinks seeds a synthetic canary into
// the prompt and scans every diagnostic sink the run produces. The
// prompt may exist in the session store (the replay DB) and in the fake
// provider's in-memory capture; diagnostic logs, the SSE telemetry
// payload, the MCP audit file and any unexpected data-dir file must be
// canary-free.
func TestE2EPromptCanaryStaysInAllowedSinks(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	root := t.TempDir()
	h := startEngine(t, root, p, true)
	session := freshSession(t, h)

	const canary = "CANARY-gotack-7f3a9b-noise-prompt"
	run := newID(t)
	p.arm(run, modeText)
	must(t, h.client.SendPromptWithAttachments(h.ctx, h.workspace, session, "Summarize this: "+canary, run, nil), "prompt_submit_failed")
	for {
		select {
		case <-h.ctx.Done():
			t.Fatal("canary_terminal_missing")
		case event, ok := <-h.events:
			if !ok {
				t.Fatal("canary_stream_closed")
			}
			var complete crushapi.RunComplete
			if json.Unmarshal(event.Payload, &complete) != nil {
				continue
			}
			if complete.RunID != run {
				continue
			}
			// The terminal event (including its telemetry projection)
			// must not echo the prompt.
			if strings.Contains(string(event.Payload), canary) {
				t.Fatal("canary_leaked_into_sse_telemetry")
			}
			must(t, checkCapture(p.counts(run), 1), "canary_provider_capture_invalid")
		}
		break
	}

	// The MCP audit file records protocol kinds only.
	audit, err := os.ReadFile(filepath.Join(root, "mcp-audit.txt"))
	if err == nil && strings.Contains(string(audit), canary) {
		t.Fatal("canary_leaked_into_mcp_audit")
	}

	// Every file under the engine's isolated data roots is scanned except
	// the session store (the replay DB) and key material: the prompt is
	// contract-allowed only there and in the in-memory provider capture.
	roots := []string{filepath.Join(root, "server-data"), filepath.Join(root, "workspace"), filepath.Join(root, "global-data"), filepath.Join(root, "config")}
	for _, base := range roots {
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			if ext == ".db" || ext == ".db-wal" || ext == ".db-shm" || ext == ".key" {
				// Allowed replay sink or key material.
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			if len(data) > 8<<20 {
				data = data[:8<<20]
			}
			if strings.Contains(string(data), canary) {
				t.Fatalf("canary leaked into diagnostic sink")
			}
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("canary scan failed: %v", err)
		}
	}
}
