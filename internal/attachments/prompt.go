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
	sb.WriteString(escapeBlockBody(strings.TrimRight(item.PromptBlock, "\n")))
	sb.WriteString("\n</" + attachmentTag + ">")
	return sb.String()
}

func escapeBlockBody(body string) string {
	body = strings.ReplaceAll(body, "</"+attachmentTag, "&lt;/"+attachmentTag)
	return strings.ReplaceAll(body, "<"+attachmentTag, "&lt;"+attachmentTag)
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
		start := findOpenTag(rest, openTag)
		if start < 0 {
			visible.WriteString(rest)
			break
		}
		attrStart := start + len(openTag)
		headRel := strings.Index(rest[attrStart:], ">")
		if headRel < 0 || strings.Contains(rest[attrStart:attrStart+headRel], "<") {
			visible.WriteString(rest[:start+1])
			rest = rest[start+1:]
			continue
		}
		head := attrStart + headRel
		endRel := strings.Index(rest[head:], closeTag)
		if endRel < 0 {
			visible.WriteString(rest)
			break
		}
		visible.WriteString(rest[:start])
		refs = append(refs, parseAttrs(rest[attrStart:head]))
		rest = rest[head+endRel+len(closeTag):]
	}
	return strings.TrimSpace(visible.String()), refs
}

func findOpenTag(text, tag string) int {
	for i := 0; i < len(text); {
		j := strings.Index(text[i:], tag)
		if j < 0 {
			return -1
		}
		i += j
		k := i + len(tag)
		if k == len(text) {
			return -1
		}
		switch text[k] {
		case '>', ' ', '\t', '\n', '\r':
			return i
		}
		i++
	}
	return -1
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
