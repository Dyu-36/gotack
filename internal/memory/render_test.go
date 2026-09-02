package memory

import (
	"strings"
	"testing"
)

func TestHeadersAndDelimiterMatchHermesFormat(t *testing.T) {
	if EntryDelimiter != "\n§\n" {
		t.Fatalf("delimiter = %q", EntryDelimiter)
	}
	if got := Header(TargetMemory, 1474, MemoryCap); got != "MEMORY (your personal notes) [67% — 1,474/2,200 chars]" {
		t.Fatalf("memory header = %q", got)
	}
	if got := Header(TargetUser, 275, UserCap); got != "USER PROFILE (who the user is) [20% — 275/1,375 chars]" {
		t.Fatalf("user header = %q", got)
	}
}

func TestRenderRoundTripPreservesMultilineUnicode(t *testing.T) {
	entries := []string{"Dùng pnpm.\nKhông dùng npm.", "A literal § inside a line is content."}
	body := serializeEntries(entries)
	if strings.Count(body, EntryDelimiter) != 1 {
		t.Fatalf("body = %q", body)
	}
	rendered := Render(TargetMemory, body)
	parsed := parseFile(rendered).Entries
	if len(parsed) != len(entries) || parsed[0] != entries[0] || parsed[1] != entries[1] {
		t.Fatalf("round trip = %#v", parsed)
	}
}

func TestParseLegacyGotackFormatRemovesOnlyProvenance(t *testing.T) {
	legacy := "MEMORY (your personal notes) [10% — 220/2,200 chars]\n" +
		"§ Project facts\n<!-- gotack-memory: session=old at=2026-08-31T09:00:00Z -->\nThe build uses pnpm.\n" +
		"§ Preferences\nShort answers.\n"
	entries := parseFile(legacy).Entries
	want := []string{"Project facts\nThe build uses pnpm.", "Preferences\nShort answers."}
	if len(entries) != len(want) || entries[0] != want[0] || entries[1] != want[1] {
		t.Fatalf("migration = %#v, want %#v", entries, want)
	}
}

func TestParseNormalizesCRLFWithoutInventingDedupe(t *testing.T) {
	raw := "one\r\n§\r\ntwo\r\n§\r\none\r\n"
	entries := parseFile(raw).Entries
	if len(entries) != 3 || entries[0] != "one" || entries[1] != "two" || entries[2] != "one" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestEmptyRenderIsEmpty(t *testing.T) {
	if got := Render(TargetMemory, ""); got != "" {
		t.Fatalf("empty memory rendered %q", got)
	}
	if got := Render(TargetUser, ""); got != "" {
		t.Fatalf("empty user rendered %q", got)
	}
}
