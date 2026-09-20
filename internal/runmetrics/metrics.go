package runmetrics

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const keyFileName = "run-metrics.key"

var keyMu sync.Mutex

func EnsureKey(dataDir string) (string, error) {
	keyMu.Lock()
	defer keyMu.Unlock()
	if dataDir == "" {
		return "", errors.New("runmetrics: data directory must not be empty")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", fmt.Errorf("create data directory: %w", err)
	}
	path := filepath.Join(dataDir, keyFileName)
	if content, err := os.ReadFile(path); err == nil {
		if len(content) != 32 {
			return "", errors.New("run metrics key must contain exactly 32 bytes")
		}
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read run metrics key: %w", err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate run metrics key: %w", err)
	}
	file, err := os.CreateTemp(dataDir, ".run-metrics-key-*")
	if err != nil {
		return "", fmt.Errorf("create run metrics key: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write run metrics key: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("sync run metrics key: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close run metrics key: %w", err)
	}
	if err := os.Link(file.Name(), path); err != nil {
		if existing, readErr := os.ReadFile(path); readErr == nil && len(existing) == 32 {
			return path, nil
		}
		return "", errors.New("runmetrics: key publication unavailable")
	}
	return path, nil
}
