from pathlib import Path


path = Path("bind_session.go")
text = path.read_text(encoding="utf-8")

replacements = {
    "attachments.Failed(filepath.Base(path), prepErr.Error())": "attachments.Failed(portableAttachmentBase(path), prepErr.Error())",
    "FileName: filepath.Base(attachment.FileName),": "FileName: portableAttachmentBase(attachment.FileName),",
    "name := filepath.Base(strings.TrimSpace(item.FileName))": "name := portableAttachmentBase(item.FileName)",
}
for old, new in replacements.items():
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"attachment basename replacement {old!r}: expected one match, found {count}")
    text = text.replace(old, new, 1)

marker = "// decodePromptAttachments turns composer uploads into prepared attachments."
helper = '''// portableAttachmentBase strips either slash style before applying the host
// platform's final clean-up. Composer payloads can retain a source-platform
// path even when tests or remote clients run on another operating system.
func portableAttachmentBase(name string) string {
    name = strings.TrimSpace(name)
    if index := strings.LastIndexAny(name, `/\\`); index >= 0 {
        name = name[index+1:]
    }
    return filepath.Base(name)
}

'''
if text.count(marker) != 1:
    raise SystemExit("decodePromptAttachments marker was not unique")
text = text.replace(marker, helper + marker, 1)
path.write_text(text, encoding="utf-8")
