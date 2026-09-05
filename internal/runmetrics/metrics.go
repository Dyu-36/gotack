package runmetrics

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const keyFileName = "run-metrics.key"

var validPrefixReasons = map[string]bool{
	"git_status": true, "date": true, "mcp": true, "skills": true,
	"context": true, "tool_set": true, "compaction": true, "model_switch": true, "none": true,
}

var validCacheStatuses = map[string]bool{
	"hit": true, "miss": true, "unreported": true,
}

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{0,256}$`)

func EnsureKey(dataDir string) (string, error) {
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
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return EnsureKey(dataDir)
	}
	if err != nil {
		return "", fmt.Errorf("create run metrics key: %w", err)
	}
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
	return path, nil
}

type Writer struct {
	path string
	log  *slog.Logger
	mu   sync.Mutex
}

func New(logDir string, log *slog.Logger) *Writer {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Writer{path: filepath.Join(logDir, "input-pipeline.jsonl"), log: log}
}

func (writer *Writer) Append(telemetry *crushapi.RunTelemetry) {
	if telemetry == nil {
		return
	}
	if err := Validate(telemetry); err != nil {
		writer.log.Warn("runmetrics: rejected invalid telemetry", "err", err)
		return
	}
	sanitized := redactSensitive(telemetry)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		writer.log.Warn("runmetrics: failed to encode telemetry", "err", err)
		return
	}
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(writer.path), 0o755); err != nil {
		writer.log.Warn("runmetrics: failed to create log directory", "err", err)
		return
	}
	file, err := os.OpenFile(writer.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		writer.log.Warn("runmetrics: failed to open telemetry log", "err", err)
		return
	}
	defer file.Close()
	encoded = append(encoded, '\n')
	if _, err := file.Write(encoded); err != nil {
		writer.log.Warn("runmetrics: failed to append telemetry", "err", err)
	}
}

func Validate(telemetry *crushapi.RunTelemetry) error {
	if telemetry.TotalMicros < 0 || telemetry.RetryDelayMicros < 0 {
		return errors.New("telemetry durations must be non-negative")
	}
	for name, duration := range telemetry.SpansMicros {
		if duration < 0 {
			return fmt.Errorf("telemetry span %q must be non-negative", name)
		}
	}
	if !validCacheStatuses[telemetry.CacheStatus] {
		return fmt.Errorf("unknown cache status %q", telemetry.CacheStatus)
	}
	if telemetry.PrefixChangedReason != "" && !validPrefixReasons[telemetry.PrefixChangedReason] {
		return fmt.Errorf("unknown prefix changed reason %q", telemetry.PrefixChangedReason)
	}
	if telemetry.RunID != "" && !idPattern.MatchString(telemetry.RunID) {
		return fmt.Errorf("invalid run_id format")
	}
	if telemetry.Provider != "" && len(telemetry.Provider) > 128 {
		return fmt.Errorf("provider identifier too long")
	}
	if telemetry.Model != "" && len(telemetry.Model) > 128 {
		return fmt.Errorf("model identifier too long")
	}
	return nil
}

func redactSensitive(telemetry *crushapi.RunTelemetry) *crushapi.RunTelemetry {
	out := *telemetry
	out.ProviderRequestID = ""
	return &out
}
