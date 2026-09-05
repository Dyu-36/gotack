//go:build e2e

package inputpipeline

import (
	"context"
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
	t.Cleanup(p.server.Close)
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
	requireTelemetry(t, tool, 2, 0, "tool", 20)
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
