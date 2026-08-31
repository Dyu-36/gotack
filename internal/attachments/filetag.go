package attachments

import "strings"

// filetag.go -- role: recognise inline file references typed in the composer,
// for example @[C:\Users\me\report.xls] or @"C:\report.xls".
//
// Ported from C:/stack's FileTag::parse. Without it the tag reached the model as
// literal text: the user saw a file named in the prompt while the agent only
// received characters, never the file's content.

const (
	bracketTagPrefix = "@["
	quotedTagPrefix  = `@"`
)

// FileTags splits text into the user-visible text with the tags removed and the
// referenced paths in first-seen order.
func FileTags(text string) (string, []string) {
	var visible strings.Builder
	var paths []string
	seen := make(map[string]bool)

	rest := text
	for {
		start, closer := nextFileTag(rest)
		if start < 0 {
			visible.WriteString(rest)
			break
		}
		inner := strings.Index(rest[start+2:], closer)
		if inner < 0 {
			visible.WriteString(rest)
			break
		}
		path := strings.TrimSpace(rest[start+2 : start+2+inner])
		consumed := start + 2 + inner + len(closer)
		if path == "" {
			// Not a file reference: keep the characters exactly as typed.
			visible.WriteString(rest[:consumed])
			rest = rest[consumed:]
			continue
		}
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
		rest = stitch(&visible, rest[:start], rest[consumed:])
	}
	return strings.TrimSpace(visible.String()), paths
}

// nextFileTag returns the offset of the next file tag and the closing delimiter
// it expects, or -1 when the text holds none.
func nextFileTag(text string) (int, string) {
	bracket := strings.Index(text, bracketTagPrefix)
	quoted := strings.Index(text, quotedTagPrefix)
	switch {
	case bracket < 0 && quoted < 0:
		return -1, ""
	case quoted < 0 || (bracket >= 0 && bracket < quoted):
		return bracket, "]"
	default:
		return quoted, `"`
	}
}

// stitch appends the text before a removed tag and returns the remainder, with
// the gap the tag left behind collapsed into one separator. Indentation
// elsewhere is untouched, so pasted code in the same prompt survives intact.
func stitch(visible *strings.Builder, before, after string) string {
	head := strings.TrimRight(before, " \t")
	tail := strings.TrimLeft(after, " \t")
	visible.WriteString(head)
	if head == "" || tail == "" {
		return tail
	}
	if strings.HasSuffix(head, "\n") || strings.HasPrefix(tail, "\n") {
		return tail
	}
	visible.WriteString(" ")
	return tail
}
