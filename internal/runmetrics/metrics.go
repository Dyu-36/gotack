package runmetrics

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
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
var labelPattern = regexp.MustCompile(`^[a-zA-Z0-9_./:\-]{0,128}$`)
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
	// Publish only fully written bytes. A second process must never observe a
	// partially written key and silently switch fingerprint identity.
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
	if err := os.MkdirAll(filepath.Dir(writer.path), 0o700); err != nil {
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
	if telemetry == nil {
		return errors.New("telemetry_missing")
	}
	if telemetry.TotalMicros < 0 || telemetry.RetryDelayMicros < 0 {
		return errors.New("telemetry durations must be non-negative")
	}
	for name, duration := range telemetry.SpansMicros {
		if !validSpans[name] || duration < 0 {
			return errors.New("telemetry_span_invalid")
		}
	}
	if !validCacheStatuses[telemetry.CacheStatus] {
		return errors.New("telemetry_cache_status_invalid")
	}
	if telemetry.PrefixChangedReason != "" && !validPrefixReasons[telemetry.PrefixChangedReason] {
		return errors.New("telemetry_prefix_reason_invalid")
	}
	if telemetry.RunID != "" && !idPattern.MatchString(telemetry.RunID) {
		return fmt.Errorf("invalid run_id format")
	}
	if !labelPattern.MatchString(telemetry.Provider) {
		return fmt.Errorf("provider identifier too long")
	}
	if !labelPattern.MatchString(telemetry.Model) {
		return fmt.Errorf("model identifier too long")
	}
	if telemetry.Attempt < 0 || telemetry.RetryCount < 0 || telemetry.StablePrefixBytes < 0 || telemetry.DynamicSuffixBytes < 0 || telemetry.RequestShapeBytes < 0 {
		return errors.New("telemetry_count_invalid")
	}
	for _, count := range []*int64{telemetry.CachedInputTokens, telemetry.UncachedInputTokens} {
		if count != nil && *count < 0 {
			return errors.New("telemetry_usage_invalid")
		}
	}
	for _, digest := range []string{telemetry.StablePrefixHMAC, telemetry.DynamicSuffixHMAC, telemetry.RequestShapeHMAC} {
		if digest == "" {
			continue
		}
		decoded, err := base64.RawURLEncoding.DecodeString(digest)
		if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != digest {
			return errors.New("telemetry_hmac_invalid")
		}
	}
	if !enum(telemetry.FirstSemantic, "", "text", "tool_call", "reasoning") ||
		!enum(telemetry.ReasoningEffort, "", "none", "minimal", "low", "medium", "high", "xhigh", "max") ||
		!enum(telemetry.ServiceTier, "", "auto", "default", "flex", "priority", "scale", "standard") {
		return errors.New("telemetry_label_invalid")
	}
	return nil
}

var validSpans = map[string]bool{
	"ready_wait": true, "mcp_wait": true, "local_preparation": true,
	"model_refresh": true, "skill_scan": true, "tool_build": true,
	"history_load": true, "prompt_prepare": true, "request_encode": true,
	"request_write": true, "request_write_to_first_byte": true,
	"first_byte_to_first_sse": true, "first_sse_to_first_reasoning": true,
	"first_sse_to_first_tool": true, "first_sse_to_first_text": true,
	"stream": true, "summarize": true, "retry_delay": true,
}

func enum(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func redactSensitive(telemetry *crushapi.RunTelemetry) *crushapi.RunTelemetry {
	out := *telemetry
	out.ProviderRequestID = ""
	out.SpansMicros = maps.Clone(telemetry.SpansMicros)
	if telemetry.CachedInputTokens != nil {
		n := *telemetry.CachedInputTokens
		out.CachedInputTokens = &n
	}
	if telemetry.UncachedInputTokens != nil {
		n := *telemetry.UncachedInputTokens
		out.UncachedInputTokens = &n
	}
	return &out
}
