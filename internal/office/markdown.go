// Package office reads, creates and edits Word, Excel and PowerPoint files.
// It backs the gotack-office MCP server so the agent can work with office
// documents through typed tools instead of ad-hoc shell scripting.
package office

import (
	"strings"
)

// markdown.go -- role: the shared document source model. office_create and
// office_edit accept one small markdown subset so a single grammar drives
// Word, Excel and PowerPoint generation.

// Block kinds produced by ParseBlocks.
const (
	kindHeading   = "heading"
	kindParagraph = "paragraph"
	kindBullet    = "bullet"
	kindNumber    = "number"
	kindTable     = "table"
	kindDivider   = "divider"
)

// Block is one block-level element of the source document.
type Block struct {
	Kind  string
	Level int      // heading level 1..3
	Text  string   // heading, paragraph and list item text
	Rows  []string // table rows in TSV form
}

// Span is an inline run with simple emphasis flags.
type Span struct {
	Text   string
	Bold   bool
	Italic bool
}

// ParseBlocks splits source text into blocks: # headings, - or * bullets,
// 1. numbered items, pipe tables and blank-line-separated paragraphs. ---
// yields a divider. Unrecognized line prefixes become paragraphs.
func ParseBlocks(src string) []Block {
	var blocks []Block
	flush := func(lines []string) {
		if text := strings.TrimSpace(strings.Join(lines, " ")); text != "" {
			blocks = append(blocks, Block{Kind: kindParagraph, Text: text})
		}
	}
	var paragraph []string
	var table []string

	flushTable := func() {
		if len(table) > 0 {
			blocks = append(blocks, Block{Kind: kindTable, Rows: table})
			table = nil
		}
	}

	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			flushTable()
			flush(paragraph)
			paragraph = nil
		case trimmed == "---":
			flushTable()
			flush(paragraph)
			paragraph = nil
			blocks = append(blocks, Block{Kind: kindDivider})
		case strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|"):
			// A separator row of dashes is structural, not content.
			if isTableSeparator(trimmed) {
				continue
			}
			if len(paragraph) > 0 {
				flush(paragraph)
				paragraph = nil
			}
			var cells []string
			for _, cell := range strings.Split(strings.Trim(trimmed, "|"), "|") {
				cells = append(cells, strings.TrimSpace(cell))
			}
			table = append(table, strings.Join(cells, "\t"))
		default:
			flushTable()
			switch {
			case len(trimmed) > 2 && trimmed[:2] == "# ":
				flush(paragraph)
				paragraph = nil
				blocks = append(blocks, Block{Kind: kindHeading, Level: 1, Text: trimmed[2:]})
			case len(trimmed) > 3 && trimmed[:3] == "## ":
				flush(paragraph)
				paragraph = nil
				blocks = append(blocks, Block{Kind: kindHeading, Level: 2, Text: trimmed[3:]})
			case len(trimmed) > 4 && trimmed[:4] == "### ":
				flush(paragraph)
				paragraph = nil
				blocks = append(blocks, Block{Kind: kindHeading, Level: 3, Text: trimmed[4:]})
			case trimmed[0] == '-' || trimmed[0] == '*':
				flush(paragraph)
				paragraph = nil
				blocks = append(blocks, Block{Kind: kindBullet, Text: strings.TrimSpace(trimmed[1:])})
			case isNumberedItem(trimmed):
				flush(paragraph)
				paragraph = nil
				dot := strings.Index(trimmed, ". ")
				blocks = append(blocks, Block{Kind: kindNumber, Text: trimmed[dot+2:]})
			default:
				paragraph = append(paragraph, trimmed)
			}
		}
	}
	flushTable()
	flush(paragraph)
	return blocks
}

// Inline splits text into bold/italic runs. **bold** and *italic* are
// supported; markers never nest.
func Inline(text string) []Span {
	var spans []Span
	var plain strings.Builder

	emit := func() {
		if plain.Len() > 0 {
			spans = append(spans, Span{Text: plain.String()})
			plain.Reset()
		}
	}

	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "**"):
			end := strings.Index(text[i+2:], "**")
			if end < 0 {
				plain.WriteString(text[i:])
				i = len(text)
				continue
			}
			emit()
			spans = append(spans, Span{Text: text[i+2 : i+2+end], Bold: true})
			i += 2 + end + 2
		case strings.HasPrefix(text[i:], "*"):
			end := strings.Index(text[i+1:], "*")
			if end <= 0 {
				plain.WriteByte(text[i])
				i++
				continue
			}
			emit()
			spans = append(spans, Span{Text: text[i+1 : i+1+end], Italic: true})
			i += 1 + end + 1
		default:
			plain.WriteByte(text[i])
			i++
		}
	}
	emit()
	return spans
}

func isTableSeparator(line string) bool {
	fields := strings.Split(strings.Trim(line, "|"), "|")
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if strings.TrimSpace(strings.ReplaceAll(field, "-", "")) != "" {
			return false
		}
	}
	return true
}

func isNumberedItem(line string) bool {
	dot := strings.Index(line, ". ")
	if dot <= 0 {
		return false
	}
	for _, r := range line[:dot] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
