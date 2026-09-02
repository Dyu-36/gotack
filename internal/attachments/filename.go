package attachments

import (
	"path"
	"strings"
)

// BaseName extracts a display name from either slash convention. Uploaded
// names come from the webview and may use the client's path separator rather
// than the host's, so filepath.Base is not sufficient on cross-platform CI.
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
