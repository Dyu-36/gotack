// Package engineobserver merges host-side transport observations (currently
// the first_byte_to_first_sse span) into the engine's RunTelemetry before the
// run_metrics writer appends it. The observer owns a small per-runID purpose
// cache so the merge can identify the (runID, attempt, purpose) tuple the
// engineapi registry used.
package engineobserver

import (
	"sync"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

// SpanFirstByteToFirstSSE is the spans_us key the observer writes when both
// timestamps are present.
const SpanFirstByteToFirstSSE = "first_byte_to_first_sse"

// Observer is a tiny merge helper that lives next to *runmetrics.Writer on
// the host App. It is constructed once at startup and re-used across all
// RunTelemetry callbacks.
type Observer struct {
	reg *engineapi.TTFBRegistry
	mu  sync.Mutex
	// lastPurposeByRun remembers the most recent purpose stamped on a
	// SendPrompt for the duration of the run. The engineapi registry is
	// indexed by (runID, attempt, purpose); the SSE side does not yet echo
	// purpose, so the merge site uses this cache to recover the key.
	lastPurposeByRun map[string]string
}

// New wires the observer to the per-Client engineapi registry. The registry
// pointer is exposed as an interface to avoid a circular package import.
func New(reg *engineapi.TTFBRegistry) *Observer {
	return &Observer{
		reg:              reg,
		lastPurposeByRun: make(map[string]string),
	}
}

// RememberPurpose stamps the most-recent purpose for a runID so the eventual
// merge can locate the (runID, attempt, purpose) entry the engineapi registry
// created. The engineapi Client itself does not need this hint; the observer
// uses it only when the engine does not echo `purpose` on RunTelemetry.
func (o *Observer) RememberPurpose(runID, purpose string) {
	if runID == "" || purpose == "" {
		return
	}
	o.mu.Lock()
	o.lastPurposeByRun[runID] = purpose
	o.mu.Unlock()
}

// ForgetPurpose drops the cached purpose once the run completes (success,
// error, or cancellation) so a subsequent run with the same UUID starts
// fresh.
func (o *Observer) ForgetPurpose(runID string) {
	if runID == "" {
		return
	}
	o.mu.Lock()
	delete(o.lastPurposeByRun, runID)
	o.mu.Unlock()
}

// Merge copies the observer-measured spans into telemetry.SpansMicros. It
// returns the spans it actually applied (currently at most one key) so the
// caller can audit which observations made it onto the wire. Returns nil
// when nothing was merged.
//
// The merge is a no-op on every failure path by construction:
//   - HTTP 5xx before any body bytes: the body wrapper never stamped firstByte.
//   - Premature connection close after headers: firstByte is stamped but
//     sseFirst stays nil.
//   - Engine-side prep_error: the SSE stream never opens, no entry exists.
//   - queued_cancellation: same — no entry exists.
//   - Cancel after SSE opened: the body's Read returns an error before any
//     data: line is decoded, sseFirst stays nil.
func (o *Observer) Merge(telemetry *engineapi.RunTelemetry) map[string]int64 {
	if telemetry == nil || o.reg == nil {
		return nil
	}
	runID := telemetry.RunID
	if runID == "" {
		return nil
	}
	firstByte, sseFirst := o.reg.Look(runID, 0, "")
	if firstByte == nil || sseFirst == nil {
		return nil
	}
	micros := sseFirst.Sub(*firstByte).Microseconds()
	if micros < 0 {
		micros = 0
	}
	if telemetry.SpansMicros == nil {
		telemetry.SpansMicros = map[string]int64{}
	}
	telemetry.SpansMicros[SpanFirstByteToFirstSSE] = micros
	o.reg.Evict(runID, 0, "")
	// After eviction, the cached purpose is no longer needed for this run.
	o.ForgetPurpose(runID)
	return map[string]int64{SpanFirstByteToFirstSSE: micros}
}

// Evict drops any pending registry entry for the run. Callers wire this to
// the RunDone/RunTelemetry error/cancel branches so a subsequent attempt
// starts with no stale stamps.
func (o *Observer) Evict(runID string) {
	if runID == "" || o.reg == nil {
		return
	}
	o.reg.Evict(runID, 0, "")
	o.reg.Evict(runID, 0, "title")
	o.reg.Evict(runID, 0, "tool_loop")
	o.reg.Evict(runID, 0, "summarize")
	o.reg.Evict(runID, 0, "retry")
	o.reg.Evict(runID, 0, "prep_error")
	o.reg.Evict(runID, 0, "queued_cancellation")
	o.ForgetPurpose(runID)
}
