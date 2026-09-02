package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const logFileName = "gotack.log"

func Setup(dir string, debug bool) (*slog.Logger, error) {
	if dir == "" {
		return nil, fmt.Errorf("logging: empty log dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("logging: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, logFileName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("logging: open %s: %w", path, err)
	}
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	w := io.MultiWriter(f, os.Stderr)
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, nil
}
