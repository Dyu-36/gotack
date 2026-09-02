package attachments

import (
	"path"
	"strings"
)

func BaseName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return ""
	}
	base := path.Base(strings.ReplaceAll(clean, `\`, "/"))
	if base == "." || base == "/" {
		return ""
	}
	return base
}
