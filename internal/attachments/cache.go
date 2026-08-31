package attachments

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Dyu-36/gotack/internal/appconfig"
)

// CacheDir returns the directory where uploaded attachments are persisted.
func CacheDir() string {
	dir := filepath.Join(appconfig.Dir(), "attachments")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// SaveToCache persists raw attachment bytes to the attachments cache dir and returns the absolute path.
func SaveToCache(fileName string, content []byte) (string, error) {
	dir := CacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir attachments dir: %w", err)
	}

	cleanName := filepath.Base(strings.TrimSpace(fileName))
	if cleanName == "" || cleanName == "." {
		cleanName = "attachment.bin"
	}

	hash := sha256.Sum256(content)
	digest := fmt.Sprintf("%x", hash)[:8]

	targetPath := filepath.Join(dir, fmt.Sprintf("%s-%s", digest, cleanName))
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return "", fmt.Errorf("save attachment %s: %w", targetPath, err)
	}

	return targetPath, nil
}
