package attachments

import (
	"strconv"
	"strings"
)

const attachmentTag = "gotack-attachment"

func ComposePrompt(text string, items []Prepared) string {
	trimmed := strings.TrimSpace(text)
	var warnings, blocks []string
	files := 0
	for _, item := range items {
		if item.Warning != "" {
			warnings = append(warnings, "> ⚠️ "+item.Warning)
			continue
		}
		if item.PromptBlock == "" && item.Attachment == nil {
			continue
		}
		files++
		if item.PromptBlock != "" {
			blocks = append(blocks, wrapBlock(item))
		}
	}
	if files == 0 && len(warnings) == 0 {
		return trimmed
	}

	head := trimmed
	if head == "" && files > 0 {
		head = "Hãy xem và xử lý tệp đính kèm sau:"
		if files > 1 {
			head = "Hãy xem và xử lý các tệp đính kèm sau:"
		}
	}
	sections := make([]string, 0, len(blocks)+2)
	if head != "" {
		sections = append(sections, head)
	}
	if len(warnings) > 0 {
		sections = append(sections, strings.Join(warnings, "\n"))
	}
	return strings.Join(append(sections, blocks...), "\n\n")
}

func wrapBlock(item Prepared) string {
	var sb strings.Builder
	sb.WriteString("<" + attachmentTag)
	sb.WriteString(` name="` + escapeAttr(item.DisplayName) + `"`)
	if item.MimeType != "" {
		sb.WriteString(` mime="` + escapeAttr(item.MimeType) + `"`)
	}
	sb.WriteString(` size="` + strconv.Itoa(item.Size) + `"`)
	if item.Path != "" {
		sb.WriteString(` path="` + escapeAttr(item.Path) + `"`)
	}
	sb.WriteString(">\n")
	sb.WriteString(strings.TrimRight(item.PromptBlock, "\n"))
	sb.WriteString("\n</" + attachmentTag + ">")
	return sb.String()
}

type Ref struct {
	FileName string
	MimeType string
	Path     string
	Size     int
}

func ParseAttachmentBlocks(prompt string) (string, []Ref) {
	openTag := "<" + attachmentTag
	closeTag := "</" + attachmentTag + ">"
	var refs []Ref
	var visible strings.Builder
	rest := prompt
	for {
		start := strings.Index(rest, openTag)
		if start < 0 {
			visible.WriteString(rest)
			break
		}
		head := strings.Index(rest[start:], ">")
		end := strings.Index(rest[start:], closeTag)
		if head < 0 || end < head {
			visible.WriteString(rest)
			break
		}
		visible.WriteString(rest[:start])
		refs = append(refs, parseAttrs(rest[start+len(openTag):start+head]))
		rest = rest[start+end+len(closeTag):]
	}
	return strings.TrimSpace(visible.String()), refs
}

func parseAttrs(head string) Ref {
	ref := Ref{}
	rest := strings.TrimSpace(head)
	for {
		eq := strings.Index(rest, `="`)
		if eq < 0 {
			return ref
		}
		key := strings.TrimSpace(rest[:eq])
		rest = rest[eq+2:]
		quote := strings.Index(rest, `"`)
		if quote < 0 {
			return ref
		}
		value := unescapeAttr(rest[:quote])
		rest = rest[quote+1:]
		switch key {
		case "name":
			ref.FileName = value
		case "mime":
			ref.MimeType = value
		case "path":
			ref.Path = value
		case "size":
			if n, err := strconv.Atoi(value); err == nil {
				ref.Size = n
			}
		}
	}
}

func escapeAttr(in string) string {
	out := strings.ReplaceAll(in, "&", "&amp;")
	out = strings.ReplaceAll(out, `"`, "&quot;")
	out = strings.ReplaceAll(out, "<", "&lt;")
	return strings.ReplaceAll(out, ">", "&gt;")
}

func unescapeAttr(in string) string {
	out := strings.ReplaceAll(in, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	return strings.ReplaceAll(out, "&amp;", "&")
}
