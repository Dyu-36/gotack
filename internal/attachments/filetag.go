package attachments

import "strings"

const (
	bracketTagPrefix = "@["
	quotedTagPrefix  = `@"`
)

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
