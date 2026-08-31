package memory

import (
	"reflect"
	"strings"
	"testing"
)

// TestSectionRoundTrip pins the § model: serialize then parse reproduces the
// structure, and parsing a canonical text then serializing reproduces the
// text byte for byte.
func TestSectionRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		file File
	}{
		{
			name: "empty file",
			file: File{},
		},
		{
			name: "single section single stamped entry",
			file: File{Sections: []Section{{
				Heading: "Project facts",
				Entries: []Entry{{
					Session: "sess-a",
					At:      "2026-09-01T10:00:00Z",
					Lines:   []string{"The build uses pnpm."},
				}},
			}}},
		},
		{
			name: "multiple sections, multiple entries, multiline content",
			file: File{Sections: []Section{
				{
					Heading: "Project facts",
					Entries: []Entry{
						{Session: "sess-a", At: "2026-09-01T10:00:00Z", Lines: []string{"First fact.", "Second line of it."}},
						{Session: "sess-b", At: "2026-09-02T09:30:00Z", Lines: []string{"Later fact."}},
					},
				},
				{
					Heading: "Preferences",
					Entries: []Entry{{Session: "sess-b", At: "2026-09-02T09:31:00Z", Lines: []string{"Concise answers."}}},
				},
			}},
		},
		{
			name: "unnamed section for pre-marker content",
			file: File{Sections: []Section{
				{Heading: "", Entries: []Entry{{Lines: []string{"Hand-written preamble."}}}},
				{Heading: "Facts", Entries: []Entry{{Session: "s", At: "2026-01-01T00:00:00Z", Lines: []string{"x"}}}},
			}},
		},
		{
			name: "unstamped entry inside a section survives",
			file: File{Sections: []Section{{
				Heading: "Mixed",
				Entries: []Entry{
					{Lines: []string{"Hand-edited line."}},
					{Session: "s", At: "2026-01-01T00:00:00Z", Lines: []string{"Tool-written."}},
				},
			}}},
		},
		{
			name: "unicode and marker-looking content stay content",
			file: File{Sections: []Section{{
				Heading: "Kế hoạch · tiếng Việt",
				Entries: []Entry{{Session: "sess-1", At: "2026-09-01T00:00:00Z", Lines: []string{
					"Dùng tiếng Việt.",
					"not <!-- gotack-memory: session=fake at=here because trailing text -->x",
				}}},
			}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := test.file.serialize()
			if len(test.file.Sections) > 0 && !strings.HasSuffix(text, "\n") {
				t.Fatalf("serialized file must end with newline:\n%q", text)
			}
			got := parseFile(text)
			if !reflect.DeepEqual(got, test.file) {
				t.Fatalf("parse(serialize(f)) mismatch:\n got %+v\nwant %+v\nserialized:\n%s", got, test.file, text)
			}
			if again := got.serialize(); again != text {
				t.Fatalf("serialize(parse(text)) mismatch:\n got %q\nwant %q", again, text)
			}
		})
	}
}

// TestParseCrlfNormalized pins that a hand-edited CRLF file parses to the
// same structure as its LF twin, so Windows edits cannot fork the format.
func TestParseCrlfNormalized(t *testing.T) {
	lf := "§ Facts\n" + provenanceStamp("s", "2026-01-01T00:00:00Z") + "\nhello\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	if !reflect.DeepEqual(parseFile(crlf), parseFile(lf)) {
		t.Fatal("CRLF input parses differently from LF input")
	}
}

// TestParseProvenanceStamps checks the stamp line split: exact matches open
// entries; near matches stay content of the open entry, so hostile or
// accidental lookalikes can never forge or split provenance.
func TestParseProvenanceStamps(t *testing.T) {
	text := "§ Facts\n" +
		provenanceStamp("sess-1", "2026-09-01T10:00:00Z") + "\nfirst\n" +
		"<!-- gotack-memory: session=broken -->\nnot a stamp\n"
	file := parseFile(text)
	if len(file.Sections) != 1 {
		t.Fatalf("sections = %+v", file.Sections)
	}
	entries := file.Sections[0].Entries
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	entry := entries[0]
	if entry.Session != "sess-1" || entry.At != "2026-09-01T10:00:00Z" {
		t.Fatalf("entry lost provenance: %+v", entry)
	}
	want := []string{"first", "<!-- gotack-memory: session=broken -->", "not a stamp"}
	if len(entry.Lines) != len(want) {
		t.Fatalf("entry lines = %+v, want %+v", entry.Lines, want)
	}
	for i := range want {
		if entry.Lines[i] != want[i] {
			t.Fatalf("entry lines = %+v, want %+v", entry.Lines, want)
		}
	}
}
