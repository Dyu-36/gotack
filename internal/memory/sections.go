package memory

import (
	"fmt"
	"regexp"
	"strings"
)

// SectionMarker delimits sections inside a memory file. The marker is the
// Hermes-compatible `§`; Crush injects the raw file into the system prompt,
// so the format stays plain text a model can read and a human can edit.
const SectionMarker = "§"

// provenancePattern matches the stamp lines the store writes ahead of every
// entry. Only exact matches open a new entry: anything else is content, so
// user text can never be misread into dropping or re-stamping an entry.
var provenancePattern = regexp.MustCompile(`^<!-- gotack-memory: session=(\S+) at=(\S+) -->$`)

// Entry is one stored item: a block of content plus the provenance recorded
// when it was written. Session and At are empty for unstamped entries —
// content that predates the tool (for example a hand-edited file).
type Entry struct {
	Session string
	At      string
	Lines   []string
}

// stamped reports whether the entry carries recorded provenance.
func (e Entry) stamped() bool { return e.Session != "" && e.At != "" }

// Section is one `§`-delimited block: a heading plus its entries.
type Section struct {
	Heading string
	Entries []Entry
}

// File is the parsed form of one memory file.
type File struct {
	Sections []Section
}

// findSection returns the index of the section with this heading, or -1.
func (f *File) findSection(heading string) int {
	for i := range f.Sections {
		if f.Sections[i].Heading == heading {
			return i
		}
	}
	return -1
}

// parseFile converts serialized text into a File. Parsing is total: every
// input round-trips, with lines that match no known structure attached to
// the current entry (or collected into an unnamed leading section).
func parseFile(text string) File {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	// Drop the empty tail produced by the mandatory final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var file File
	current := -1 // index into file.Sections; -1 before the first marker
	appendLine := func(line string) {
		if current < 0 {
			// Content before any marker: keep it in an unnamed section so
			// hand-edited files lose nothing on the next write.
			file.Sections = append(file.Sections, Section{})
			current = len(file.Sections) - 1
		}
		section := &file.Sections[current]
		if len(section.Entries) == 0 {
			section.Entries = append(section.Entries, Entry{})
		}
		entry := &section.Entries[len(section.Entries)-1]
		entry.Lines = append(entry.Lines, line)
	}

	for _, line := range lines {
		if strings.HasPrefix(line, SectionMarker) {
			heading := strings.TrimSpace(strings.TrimPrefix(line, SectionMarker))
			file.Sections = append(file.Sections, Section{Heading: heading})
			current = len(file.Sections) - 1
			continue
		}
		if match := provenancePattern.FindStringSubmatch(line); match != nil {
			if current < 0 {
				file.Sections = append(file.Sections, Section{})
				current = len(file.Sections) - 1
			}
			file.Sections[current].Entries = append(file.Sections[current].Entries, Entry{
				Session: match[1],
				At:      match[2],
			})
			continue
		}
		appendLine(line)
	}
	return file
}

// serialize renders the file. The output is canonical: parseFile(serialize(f))
// reproduces f, and the last line always ends with a newline so the file is
// well-formed text inside the system prompt.
func (f *File) serialize() string {
	if len(f.Sections) == 0 {
		return ""
	}
	var builder strings.Builder
	for sectionIndex, section := range f.Sections {
		if sectionIndex > 0 {
			builder.WriteByte('\n')
		}
		if section.Heading == "" {
			builder.WriteString(SectionMarker)
		} else {
			builder.WriteString(SectionMarker + " " + section.Heading)
		}
		for _, entry := range section.Entries {
			if entry.stamped() {
				builder.WriteByte('\n')
				builder.WriteString(provenanceStamp(entry.Session, entry.At))
			}
			for _, line := range entry.Lines {
				builder.WriteByte('\n')
				builder.WriteString(line)
			}
		}
	}
	builder.WriteByte('\n')
	return builder.String()
}

// provenanceStamp renders the exact stamp line provenancePattern parses.
func provenanceStamp(session, at string) string {
	return fmt.Sprintf("<!-- gotack-memory: session=%s at=%s -->", session, at)
}
