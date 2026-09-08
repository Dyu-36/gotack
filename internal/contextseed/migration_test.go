package contextseed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/memory"
)

func writeContextFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Migration is now additive: rollback is the untouched source, not a mutable
// legacy/layered state machine. Preserve the no-loss/restart/overwrite invariants.
func TestLegacyPersonalImportIsAdditiveAndRestartSafe(t *testing.T) {
	data := t.TempDir()
	old := filepath.Join(data, "context", "memory", "USER.md")
	writeContextFixture(t, old, "Prefers Vietnamese")
	s := New(data, nil)
	if err := s.Seed(""); err != nil {
		t.Fatal(err)
	}
	if got := string(mustRead(t, old)); got != "Prefers Vietnamese" {
		t.Fatal("legacy original was changed")
	}
	if got := readSeeded(t, s, memory.ProfileFileName); !strings.Contains(got, "Prefers Vietnamese") {
		t.Fatal("legacy profile was not imported")
	}
	writeContextFixture(t, filepath.Join(s.ContextDir(), memory.ProfileFileName), "A newer preference")
	if err := New(data, nil).Seed(""); err != nil {
		t.Fatal(err)
	}
	if got := readSeeded(t, s, memory.ProfileFileName); got != "A newer preference" {
		t.Fatal("restart overwrote a newer profile")
	}
	report, err := memory.ReadImportReport(data)
	if err != nil || report.Version != 1 {
		t.Fatalf("import report: %+v, %v", report, err)
	}
	if got := string(mustRead(t, filepath.Join(report.BackupDir, "memory-USER.md"))); got != "Prefers Vietnamese" {
		t.Fatal("legacy backup is not intact")
	}
}

func TestLegacyCustomInstructionsArePreservedButNotPromoted(t *testing.T) {
	data := t.TempDir()
	for _, name := range []string{"USER.md", "TACK.md"} {
		writeContextFixture(t, filepath.Join(data, "context", name), "custom-legacy-policy")
	}
	s := New(data, nil)
	if err := s.Seed(""); err != nil {
		t.Fatal(err)
	}
	report, err := memory.ReadImportReport(data)
	if err != nil || len(report.NeedsReview) != 2 {
		t.Fatalf("custom context was not surfaced for review: %+v, %v", report, err)
	}
	gen, err := s.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mustRead(t, filepath.Join(gen, managedCoreName))), "custom-legacy-policy") {
		t.Fatal("legacy user text was promoted into product policy")
	}
	for _, name := range []string{"USER.md", "TACK.md"} {
		if got := string(mustRead(t, filepath.Join(data, "context", name))); got != "custom-legacy-policy" {
			t.Fatal("legacy custom instructions were removed")
		}
	}
}

func TestInterruptedImportPreservesAlreadyWrittenProfile(t *testing.T) {
	data := t.TempDir()
	s := New(data, nil)
	writeContextFixture(t, filepath.Join(data, "context", "memory", "USER.md"), "Older profile")
	writeContextFixture(t, filepath.Join(data, "context", "memory", "MEMORY.md"), "Durable fact")
	writeContextFixture(t, filepath.Join(s.ContextDir(), memory.ProfileFileName), "New profile")
	if err := s.Seed(""); err != nil {
		t.Fatal(err)
	}
	if got := readSeeded(t, s, memory.ProfileFileName); got != "New profile" {
		t.Fatal("import overwrote a completed/newer profile")
	}
	if !strings.Contains(readSeeded(t, s, memory.MemoryFileName), "Durable fact") {
		t.Fatal("retry did not import the remaining memory file")
	}
}

func TestLegacyOversizedEntriesRemainAvailableInOriginal(t *testing.T) {
	data := t.TempDir()
	body := "Durable fact" + memory.EntryDelimiter + strings.Repeat("x", memory.MemoryCap+1)
	old := filepath.Join(data, "context", "memory", "MEMORY.md")
	writeContextFixture(t, old, body)
	s := New(data, nil)
	if err := s.Seed(""); err != nil {
		t.Fatal(err)
	}
	if string(mustRead(t, old)) != body {
		t.Fatal("bounded import destroyed the original")
	}
	report, err := memory.ReadImportReport(data)
	if err != nil || len(report.NeedsReview) == 0 {
		t.Fatal("omitted entries were not reported for review")
	}
	if memory.PromptBodyChars(readSeeded(t, s, memory.MemoryFileName)) > memory.MemoryCap {
		t.Fatal("import bypassed the hot-memory cap")
	}
}
