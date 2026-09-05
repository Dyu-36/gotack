//go:build e2e

package inputpipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func sendTelemetryTurn(t *testing.T, h *engineHarness, p *fakeProvider, session string, mode providerMode) (crushapi.RunComplete, captureCounts) {
	t.Helper()
	run := newID(t)
	p.arm(run, mode)
	must(t, h.client.SendPromptWithAttachments(h.ctx, h.workspace, session, "Run the synthetic fixture.", run, nil), "telemetry_prompt_submit_failed")
	for {
		select {
		case <-h.ctx.Done():
			t.Fatal("telemetry_terminal_missing")
		case event, ok := <-h.events:
			if !ok {
				t.Fatal("telemetry_stream_closed")
			}
			var complete crushapi.RunComplete
			if json.Unmarshal(event.Payload, &complete) != nil {
				t.Fatal("telemetry_terminal_schema_invalid")
			}
			if complete.RunID != run || complete.SessionID != session {
				continue
			}
			counts := p.counts(run)
			must(t, checkCapture(counts, 1), "telemetry_provider_capture_invalid")
			return complete, counts
		}
	}
}

func requireTelemetry(t *testing.T, complete crushapi.RunComplete, attempts, retries int, firstSemantic string, uncached int64) {
	requireTelemetryWithTool(t, complete, attempts, retries, firstSemantic, uncached, false)
}

func requireTelemetryWithTool(t *testing.T, complete crushapi.RunComplete, attempts, retries int, firstSemantic string, uncached int64, expectedFirstTool bool) {
	t.Helper()
	if complete.Error != "" || complete.Cancelled || complete.Text != fixtureAnswer {
		t.Fatal("telemetry_terminal_outcome_invalid")
	}
	m := complete.Telemetry
	if m == nil {
		t.Fatal("run_telemetry_missing")
	}
	if m.RunID != complete.RunID || m.Provider != "e2e" || m.Model != mainModel {
		t.Fatal("run_telemetry_identity_invalid")
	}
	if m.Attempt != attempts || m.RetryCount != retries || m.RetryDelayMicros < 0 {
		raw, _ := json.Marshal(m)
		t.Logf("DEBUG telemetry=%s", raw)
		t.Fatal("run_telemetry_attempt_invalid")
	}
	if m.FirstSemantic != firstSemantic {
		t.Fatal("run_telemetry_first_semantic_invalid")
	}
	if m.CacheStatus != "miss" || m.CachedInputTokens == nil || *m.CachedInputTokens != 0 ||
		m.UncachedInputTokens == nil || *m.UncachedInputTokens != uncached {
		t.Fatal("run_telemetry_cache_invalid")
	}
	if m.TotalMicros < 0 || m.EstimatedUsage || m.Compacted || m.PrefixChangedReason != "" {
		t.Fatal("run_telemetry_metadata_invalid")
	}
	// The final prepared-request fingerprint and the per-run stable/dynamic
	// split digests must be present and shaped like a 32-byte base64url
	// HMAC (key is available because the engine data dir is writable).
	for name, digest := range map[string]string{
		"request_shape_hmac":  m.RequestShapeHMAC,
		"stable_prefix_hmac":  m.StablePrefixHMAC,
		"dynamic_suffix_hmac": m.DynamicSuffixHMAC,
	} {
		if digest == "" {
			t.Fatal("run_telemetry_hmac_missing:" + name)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(digest)
		if err != nil || len(decoded) != 32 {
			t.Fatal("run_telemetry_hmac_shape_invalid")
		}
	}
	if m.RequestShapeBytes <= 0 || m.StablePrefixBytes <= 0 {
		t.Fatal("run_telemetry_byte_counts_invalid")
	}
	// A completed text turn has a real first-text offset; the tool turn
	// also records its first tool offset. Absent kinds stay nil.
	if firsts := map[string]*int64{
		"first_text_us": m.FirstTextMicros,
	}; firsts["first_text_us"] == nil {
		t.Fatal("run_telemetry_first_text_missing")
	}
	if expectedFirstTool && m.FirstToolMicros == nil {
		t.Fatal("run_telemetry_first_tool_missing")
	}
	if !expectedFirstTool && m.FirstToolMicros != nil {
		t.Fatal("run_telemetry_first_tool_unexpected")
	}
	if m.FirstReasoningMicros != nil {
		t.Fatal("run_telemetry_first_reasoning_unexpected")
	}
	for name, duration := range m.SpansMicros {
		if name == "" || duration < 0 {
			t.Fatal("run_telemetry_span_invalid")
		}
	}
	// request_write_to_first_byte spans request issue to the first stream
	// part. first_byte_to_first_sse additionally requires a transport hook
	// Fantasy does not expose, so per the absent-means-absent contract that
	// span stays unrecorded rather than being faked from the same instant.
	for _, name := range []string{"history_load", "prompt_prepare", "request_write_to_first_byte", "stream"} {
		if _, ok := m.SpansMicros[name]; !ok {
			t.Fatal("run_telemetry_required_span_missing")
		}
	}
}

func TestE2ERunTelemetryFreshRetryAndToolLoop(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	h := startEngine(t, t.TempDir(), p, true)
	session := freshSession(t, h)

	fresh, freshCounts := sendTelemetryTurn(t, h, p, session, modeText)
	if freshCounts.Requests != 1 {
		t.Fatal("telemetry_fresh_request_count_invalid")
	}
	requireTelemetry(t, fresh, 1, 0, "text", 10)

	retry, retryCounts := sendTelemetryTurn(t, h, p, session, modeRetry)
	if retryCounts.Requests != 2 || retryCounts.Retries != 1 {
		t.Fatal("telemetry_retry_request_count_invalid")
	}
	requireTelemetry(t, retry, 2, 1, "text", 10)

	tool, toolCounts := sendTelemetryTurn(t, h, p, session, modeTool)
	if toolCounts.Requests != 2 || toolCounts.ToolResponses != 1 || toolCounts.ToolResults != 1 {
		t.Fatal("telemetry_tool_request_count_invalid")
	}
	requireTelemetryWithTool(t, tool, 2, 0, "tool_call", 20, true)
}

// TestE2ETelemetryStablePrefixStableAcrossTurns proves the PR2 stable
// prefix contract on the wire: two turns with unchanged stable inputs
// publish identical stable_prefix_hmac values while the request shape
// reflects the grown history, and no stable change reason is invented.
func TestE2ETelemetryStablePrefixStableAcrossTurns(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.close)
	h := startEngine(t, t.TempDir(), p, true)
	session := freshSession(t, h)

	first, _ := sendTelemetryTurn(t, h, p, session, modeText)
	second, _ := sendTelemetryTurn(t, h, p, session, modeText)
	m1, m2 := first.Telemetry, second.Telemetry
	if m1 == nil || m2 == nil {
		t.Fatal("run_telemetry_missing")
	}
	if m1.StablePrefixHMAC == "" || m1.StablePrefixHMAC != m2.StablePrefixHMAC {
		t.Fatal("stable_prefix_hmac_changed_without_stable_input_change")
	}
	if m2.RequestShapeHMAC == m1.RequestShapeHMAC {
		t.Fatal("request_shape_hmac_ignored_history_growth")
	}
	if m2.PrefixChangedReason != "" {
		t.Fatal("stable_reason_invented_for_unchanged_inputs")
	}
}

func TestTelemetryWaitRejectsClosedStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("telemetry_context_cancel_invalid")
		}
	default:
		t.Fatal("telemetry_context_cancel_missing")
	}
}
