package engineobserver

import (
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// TestMergeHappyPath asserts that a fully populated registry produces the
// expected span on telemetry and evicts the entry.
func TestMergeHappyPath(t *testing.T) {
	reg := crushapi.NewTTFBRegistry()
	t0 := time.Unix(1700000000, 0)
	reg.BindFirstByte("r1", 0, "", t0)
	reg.MarkSSEFirst("r1", 0, "")

	obs := New(reg)
	telem := &crushapi.RunTelemetry{RunID: "r1"}
	spans := obs.Merge(telem)
	if spans == nil {
		t.Fatal("expected merged span")
	}
	if fb, ss, ok := regEntryExists(reg, "r1", 0, ""); ok {
		t.Fatalf("entry not evicted after merge: fb=%v ss=%v", fb, ss)
	} else if fb != nil || ss != nil {
		t.Fatalf("expected nil stamps, got fb=%v ss=%v", fb, ss)
	}
}

// TestMergeMissingFirstByte asserts that no span is written when only the
// SSE-first stamp is present (e.g. the body wrapper never saw data).
func TestMergeMissingFirstByte(t *testing.T) {
	reg := crushapi.NewTTFBRegistry()
	reg.MarkSSEFirst("r1", 0, "")

	obs := New(reg)
	telem := &crushapi.RunTelemetry{RunID: "r1"}
	if spans := obs.Merge(telem); spans != nil {
		t.Fatalf("expected no span, got %#v", spans)
	}
	if len(telem.SpansMicros) != 0 {
		t.Fatal("telemetry mutated despite missing firstByte")
	}
}

// TestMergeMissingSSEFirst asserts that no span is written when only the
// first-byte stamp is present (the SSE stream never produced a frame).
func TestMergeMissingSSEFirst(t *testing.T) {
	reg := crushapi.NewTTFBRegistry()
	t0 := time.Unix(1700000000, 0)
	reg.BindFirstByte("r1", 0, "", t0)

	obs := New(reg)
	telem := &crushapi.RunTelemetry{RunID: "r1"}
	if spans := obs.Merge(telem); spans != nil {
		t.Fatalf("expected no span, got %#v", spans)
	}
	if len(telem.SpansMicros) != 0 {
		t.Fatal("telemetry mutated despite missing sseFirst")
	}
}

// TestRememberPurposeFallback asserts that when telemetry.Purpose is empty,
// the observer uses the cached purpose from RememberPurpose.
func TestRememberPurposeFallback(t *testing.T) {
	reg := crushapi.NewTTFBRegistry()
	t0 := time.Unix(1700000000, 0)
	reg.BindFirstByte("r1", 0, "", t0)
	reg.MarkSSEFirst("r1", 0, "")

	obs := New(reg)
	obs.RememberPurpose("r1", "title")
	telem := &crushapi.RunTelemetry{RunID: "r1"}
	if spans := obs.Merge(telem); spans == nil {
		t.Fatal("expected merge via remembered purpose")
	}
}

// TestEvictRemovesAllAttempts verifies the Evict helper drops every purpose
// variant so the next attempt starts clean.
func TestEvictRemovesAllAttempts(t *testing.T) {
	reg := crushapi.NewTTFBRegistry()
	t0 := time.Unix(1700000000, 0)
	for _, p := range []string{"title", "tool_loop", "summarize", "retry", "prep_error", "queued_cancellation"} {
		reg.BindFirstByte("r1", 0, p, t0)
		reg.MarkSSEFirst("r1", 0, p)
	}
	obs := New(reg)
	obs.RememberPurpose("r1", "title")
	obs.Evict("r1")
	for _, p := range []string{"title", "tool_loop", "summarize", "retry", "prep_error", "queued_cancellation"} {
		if _, _, ok := regEntryExists(reg, "r1", 0, p); ok {
			t.Fatalf("entry for %q survived Evict", p)
		}
	}
}

// regEntryExists reports whether the registry has an entry for (runID,
// attempt, purpose). It exists because crushapi.TTFBRegistry.Look returns
// two time pointers, not an "ok" boolean.
func regEntryExists(reg *crushapi.TTFBRegistry, runID string, attempt int, purpose string) (*time.Time, *time.Time, bool) {
	fb, ss := reg.Look(runID, attempt, purpose)
	return fb, ss, fb != nil || ss != nil
}

// TestMergeNilSafe asserts the observer does not panic on nil inputs.
func TestMergeNilSafe(t *testing.T) {
	obs := New(nil)
	if spans := obs.Merge(nil); spans != nil {
		t.Fatalf("expected nil, got %#v", spans)
	}
	obs = New(crushapi.NewTTFBRegistry())
	if spans := obs.Merge(nil); spans != nil {
		t.Fatalf("expected nil, got %#v", spans)
	}
	if spans := obs.Merge(&crushapi.RunTelemetry{}); spans != nil {
		t.Fatalf("expected nil for empty telemetry, got %#v", spans)
	}
}

// TestRememberPurposeNoop asserts that empty inputs are silently dropped.
func TestRememberPurposeNoop(t *testing.T) {
	obs := New(crushapi.NewTTFBRegistry())
	obs.RememberPurpose("", "title")
	obs.RememberPurpose("r1", "")
	if v := obs.lastPurposeByRun["r1"]; v != "" {
		t.Fatalf("empty purpose leaked into cache: %q", v)
	}
}

// TestSpanKeyContract pins the public span key so external readers can rely
// on its shape.
func TestSpanKeyContract(t *testing.T) {
	if SpanFirstByteToFirstSSE != "first_byte_to_first_sse" {
		t.Fatalf("SpanFirstByteToFirstSSE = %q, want %q", SpanFirstByteToFirstSSE, "first_byte_to_first_sse")
	}
}
