package memory

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	EntryMarker    = "§"
	EntryDelimiter = "\n§\n"
	blockSeparator = "══════════════════════════════════════════════"

	memoryHeaderLabel = "MEMORY (your personal notes)"
	userHeaderLabel   = "USER PROFILE (who the user is)"
)

type File struct {
	Entries []string
}

func Header(target Target, size, capacity int) string {
	percent := 0
	if capacity > 0 {
		percent = size * 100 / capacity
		if percent > 100 {
			percent = 100
		}
	}
	label := memoryHeaderLabel
	if target == TargetUser {
		label = userHeaderLabel
	}
	return fmt.Sprintf("%s [%d%% — %s/%s chars]", label, percent, group(size), group(capacity))
}

func Render(target Target, body string) string {
	if body == "" {
		return ""
	}
	header := Header(target, utf8.RuneCountInString(body), CapFor(target))
	return blockSeparator + "\n" + header + "\n" + blockSeparator + "\n" + body
}

func serializeEntries(entries []string) string {
	return strings.Join(entries, EntryDelimiter)
}

func parseFile(text string) File {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = stripBlockHeader(text)
	if strings.TrimSpace(text) == "" {
		return File{}
	}

	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, EntryMarker+"\n") || strings.HasPrefix(trimmed, EntryMarker+" ") {
		return File{Entries: parseLegacyEntries(trimmed)}
	}

	parts := strings.Split(text, EntryDelimiter)
	entries := make([]string, 0, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry != "" {
			entries = append(entries, entry)
		}
	}
	return File{Entries: entries}
}

func stripBlockHeader(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) >= 3 && lines[0] == blockSeparator && isMemoryHeader(lines[1]) && lines[2] == blockSeparator {
		return strings.Join(lines[3:], "\n")
	}

	if len(lines) > 0 && isMemoryHeader(lines[0]) {
		return strings.Join(lines[1:], "\n")
	}
	return text
}

func isMemoryHeader(line string) bool {
	return (strings.HasPrefix(line, memoryHeaderLabel+" [") || strings.HasPrefix(line, userHeaderLabel+" [")) && strings.HasSuffix(line, " chars]")
}

func parseLegacyEntries(text string) []string {
	var entries []string
	var current []string
	flush := func() {
		entry := strings.TrimSpace(strings.Join(current, "\n"))
		if entry != "" {
			entries = append(entries, entry)
		}
		current = nil
	}
	for _, line := range strings.Split(text, "\n") {
		if line == EntryMarker || strings.HasPrefix(line, EntryMarker+" ") {
			flush()
			if label := strings.TrimSpace(strings.TrimPrefix(line, EntryMarker)); label != "" {
				current = append(current, label)
			}
			continue
		}
		if strings.HasPrefix(line, "<!-- gotack-memory:") && strings.HasSuffix(line, "-->") {
			continue
		}
		current = append(current, line)
	}
	flush()
	return entries
}

func group(value int) string {
	digits := strconv.Itoa(value)
	var out strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}
