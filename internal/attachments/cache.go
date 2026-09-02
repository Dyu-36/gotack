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

func CacheDir() string {
	dir := filepath.Join(appconfig.Dir(), "attachments")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

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

func SanitizeFileName(fileName string) string {
	base := BaseName(fileName)
	if base == "" {
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
