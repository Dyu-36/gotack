package attachments

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/Dyu-36/gotack/internal/appconfig"
)

// cache.go -- role: persist uploaded attachment bytes so the agent, and any
// conversion step, has a real file path to work with.
//
// Layout: <configDir>/attachments/<sha8>/<sanitized name>. The digest lives in
// the directory instead of being glued onto the file name, so filepath.Base
// still returns a name the user recognizes in the UI, and shell-hostile
// characters never reach a command line.

// CacheDir returns the directory where uploaded attachments are persisted.
func CacheDir() string {
	dir := filepath.Join(appconfig.Dir(), "attachments")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// SaveToCache persists raw attachment bytes and returns the absolute path.
func SaveToCache(fileName string, content []byte) (string, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256(content))[:8]
	dir := filepath.Join(CacheDir(), digest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir attachments dir: %w", err)
	}

	targetPath := filepath.Join(dir, SanitizeFileName(fileName))
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return "", fmt.Errorf("save attachment %s: %w", targetPath, err)
	}
	return targetPath, nil
}

// SanitizeFileName keeps a readable file name that is still safe to hand to a
// shell: letters (including Vietnamese), digits, dot, dash and underscore
// survive, every other run of characters collapses into one underscore.
func SanitizeFileName(fileName string) string {
	base := filepath.Base(strings.TrimSpace(fileName))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "attachment.bin"
	}
	ext := filepath.Ext(base)
	stem := sanitizeSegment(strings.TrimSuffix(base, ext))
	if stem == "" {
		stem = "attachment"
	}
	return stem + strings.ToLower(sanitizeSegment(ext))
}

func sanitizeSegment(in string) string {
	var sb strings.Builder
	sb.Grow(len(in))
	padded := false
	for _, r := range in {
		switch {
		case r == '.' || r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			sb.WriteRune(r)
			padded = false
		case !padded:
			sb.WriteByte('_')
			padded = true
		}
	}
	return strings.Trim(sb.String(), "_")
}
