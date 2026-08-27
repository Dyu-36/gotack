package changes

import (
	"strings"
	"testing"
)

func TestRenderDiffChange(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng\nh\n"
	new := "a\nb\nC\nd\ne\nf\ng\nh\n"
	out, err := RenderDiff(old, new, "file.txt", 0)
	if err != nil {
		t.Fatalf("RenderDiff: %v", err)
	}
	if !strings.HasPrefix(out, "--- file.txt\n+++ file.txt\n") {
		t.Fatalf("missing file headers:\n%s", out)
	}
	// Context clipping at file boundaries yields a 1,6 hunk here.
	if !strings.Contains(out, "@@ -1,6 +1,6 @@") {
		t.Fatalf("missing hunk header:\n%s", out)
	}
	if !strings.Contains(out, "\n-c\n") || !strings.Contains(out, "\n+C\n") {
		t.Fatalf("missing change lines:\n%s", out)
	}
}

func TestRenderDiffIdentical(t *testing.T) {
	out, err := RenderDiff("same\n", "same\n", "f", 0)
	if err != nil {
		t.Fatalf("RenderDiff: %v", err)
	}
	if strings.Contains(out, "@@") {
		t.Fatalf("expected no hunks for identical input:\n%s", out)
	}
}

func TestRenderDiffTruncate(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("old line\n")
	}
	new := strings.ReplaceAll(sb.String(), "old", "new-replacement-value")
	out, err := RenderDiff(sb.String(), new, "f", 200)
	if err != nil {
		t.Fatalf("RenderDiff: %v", err)
	}
	if int64(len(out)) > 200+int64(len(truncationMarker)) {
		t.Fatalf("output exceeds cap: %d bytes", len(out))
	}
	if !strings.HasSuffix(out, truncationMarker) {
		t.Fatalf("missing truncation marker at end:\n%q", out[len(out)-60:])
	}
}

func TestRenderDiffFullReplaceFallback(t *testing.T) {
	old := strings.Repeat("x\n", maxDiffLines+1)
	new := strings.Repeat("y\n", maxDiffLines+1)
	out, err := RenderDiff(old, new, "big.bin", 0)
	if err != nil {
		t.Fatalf("RenderDiff: %v", err)
	}
	if !strings.Contains(out, "-x") || !strings.Contains(out, "+y") {
		t.Fatalf("full replace hunk missing:\n%s", out[:120])
	}
}

func TestSplitLines(t *testing.T) {
	if got := splitLines(""); len(got) != 1 || got[0] != "" {
		t.Fatalf("splitLines(\"\") = %q, want [\"\"]", got)
	}
	if got := splitLines("a\nb\n"); len(got) != 3 || got[0] != "a" || got[2] != "" {
		t.Fatalf("splitLines trailing newline: %q", got)
	}
}
