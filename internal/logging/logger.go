package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// logger.go -- role: logger construction, level handling and file sink.
//
// The destination comes from internal/appconfig. Keep logging cheap: this app
// targets 6 GB machines and must not buffer large amounts of text.

// logFileName is the file the logger writes to under the user log dir.
const logFileName = "gotack.log"

// Setup builds a slog.Logger that writes to dir/gotack.log, with a secondary
// stderr mirror so the developer console is also useful. Level is Info by
// default; passing debug=true sets the level to Debug. The returned logger is
// set as the process default via slog.SetDefault so packages that use
// slog.Default() pick it up.
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
		// AddSource is intentionally off: it costs a runtime.Callers per record
		// and is rarely useful for a desktop host.
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, nil
}
