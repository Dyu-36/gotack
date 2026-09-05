package runmetrics

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/stretchr/testify/require"
)

func TestEnsureKeyIsStableAndPrivate(t *testing.T) {
	dir := t.TempDir()
	first, err := EnsureKey(dir)
	require.NoError(t, err)
	second, err := EnsureKey(dir)
	require.NoError(t, err)
	require.Equal(t, first, second)
	content, err := os.ReadFile(first)
	require.NoError(t, err)
	require.Len(t, content, 32)
	info, err := os.Stat(first)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestEnsureKeyEmptyDir(t *testing.T) {
	_, err := EnsureKey("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestWriterRejectsNegativeSpanAndWritesRedactedShape(t *testing.T) {
	dir := t.TempDir()
	writer := New(dir, slog.Default())
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", SpansMicros: map[string]int64{"stream": -1}}))
	writer.Append(&crushapi.RunTelemetry{RunID: "run-1", CacheStatus: "unreported", StablePrefixHMAC: strings.Repeat("A", 43), StablePrefixBytes: 42})
	content, err := os.ReadFile(writer.path)
	require.NoError(t, err)
	require.Contains(t, string(content), `"stable_prefix_hmac":"`+strings.Repeat("A", 43)+`"`)
	require.NotContains(t, string(content), "prompt")
}

func TestValidateCacheStatus(t *testing.T) {
	for _, cs := range []string{"hit", "miss", "unreported"} {
		require.NoError(t, Validate(&crushapi.RunTelemetry{CacheStatus: cs}))
	}
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unknown"}))
}

func TestValidatePrefixChangedReason(t *testing.T) {
	for _, reason := range []string{"git_status", "date", "mcp", "skills", "context", "tool_set", "compaction", "model_switch", "none"} {
		require.NoError(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", PrefixChangedReason: reason}))
	}
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", PrefixChangedReason: "invalid_reason"}))
}

func TestValidateChangeReasons(t *testing.T) {
	require.NoError(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported",
		ChangeReasons: []string{"context", "date", "skills", "todo"}}))
	// Dynamic-only reasons and initial never validate as a primary reason.
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", PrefixChangedReason: "initial"}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", PrefixChangedReason: "todo"}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", ChangeReasons: []string{"unknown_reason"}}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", ChangeReasons: []string{""}}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", ChangeReasons: []string{"date", "date"}}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", ChangeReasons: []string{"skills", "context"}}))
}

func TestValidateNegativeDuration(t *testing.T) {
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", TotalMicros: -1}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", RetryDelayMicros: -1}))
}

func TestValidateRunIDFormat(t *testing.T) {
	require.NoError(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", RunID: "run-abc_123.def"}))
	require.NoError(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", RunID: ""}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", RunID: "has spaces"}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", RunID: string(make([]byte, 300))}))
}

func TestRedactSensitiveFields(t *testing.T) {
	telemetry := &crushapi.RunTelemetry{
		RunID:             "run-1",
		ProviderRequestID: "req-abc-123",
		CacheStatus:       "hit",
	}
	dir := t.TempDir()
	writer := New(dir, slog.Default())
	writer.Append(telemetry)
	content, err := os.ReadFile(writer.path)
	require.NoError(t, err)
	require.NotContains(t, string(content), "req-abc-123")
	require.Contains(t, string(content), `"run_id":"run-1"`)
}

func TestWriterNilSafe(t *testing.T) {
	dir := t.TempDir()
	writer := New(dir, slog.Default())
	writer.Append(nil)
	if _, err := os.Stat(writer.path); err == nil {
		t.Fatal("file should not exist after nil append")
	}
}

func TestWriterNilLogger(t *testing.T) {
	dir := t.TempDir()
	writer := New(dir, nil)
	writer.Append(&crushapi.RunTelemetry{CacheStatus: "unreported"})
}

func TestWriterConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	writer := New(dir, slog.Default())
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			writer.Append(&crushapi.RunTelemetry{
				RunID:       "run-concurrent",
				CacheStatus: "unreported",
			})
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	content, err := os.ReadFile(writer.path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 10)
}

func TestValidateProviderModelTooLong(t *testing.T) {
	long := strings.Repeat("a", 200)
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", Provider: long}))
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", Model: long}))
}

func TestNewWriterCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "log")
	writer := New(dir, slog.Default())
	writer.Append(&crushapi.RunTelemetry{CacheStatus: "unreported"})
	_, err := os.Stat(writer.path)
	require.NoError(t, err)
}

func TestOldTelemetryPayloadCompatibility(t *testing.T) {
	oldPayload := `{"run_id":"old-run","total_us":100000,"cache_status":"unreported","attempt":1}`
	var telemetry crushapi.RunTelemetry
	require.NoError(t, json.Unmarshal([]byte(oldPayload), &telemetry))
	require.Equal(t, "old-run", telemetry.RunID)
	require.Equal(t, int64(100000), telemetry.TotalMicros)
	require.Equal(t, "unreported", telemetry.CacheStatus)

	newPayload, err := json.Marshal(&crushapi.RunTelemetry{
		RunID:       "new-run",
		CacheStatus: "hit",
		TotalMicros: 200000,
	})
	require.NoError(t, err)
	require.Contains(t, string(newPayload), "new-run")
}

func TestValidateEmptyTelemetry(t *testing.T) {
	require.NoError(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported"}))
}

func TestValidateAllSpansNonNegative(t *testing.T) {
	spans := map[string]int64{
		"ready_wait":     0,
		"mcp_wait":       100,
		"prompt_prepare": 200,
		"stream":         300,
		"summarize":      0,
	}
	require.NoError(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", SpansMicros: spans}))

	spans["negative"] = -1
	require.Error(t, Validate(&crushapi.RunTelemetry{CacheStatus: "unreported", SpansMicros: spans}))
}

func TestRedactionPreservesAllSafeFields(t *testing.T) {
	telemetry := &crushapi.RunTelemetry{
		RunID:               "run-safe",
		Provider:            "openai",
		Model:               "gpt-4",
		ReasoningEffort:     "high",
		Attempt:             2,
		RetryCount:          1,
		RetryDelayMicros:    5000,
		SpansMicros:         map[string]int64{"stream": 100000},
		TotalMicros:         200000,
		FirstSemantic:       "text",
		CacheStatus:         "hit",
		ServiceTier:         "priority",
		ProviderRequestID:   "req-secret",
		EstimatedUsage:      true,
		Compacted:           false,
		PrefixChangedReason: "model_switch",
		StablePrefixHMAC:    strings.Repeat("A", 43),
		StablePrefixBytes:   1024,
		DynamicSuffixHMAC:   strings.Repeat("A", 43),
		DynamicSuffixBytes:  512,
		RequestShapeHMAC:    strings.Repeat("A", 43),
		RequestShapeBytes:   2048,
	}
	dir := t.TempDir()
	writer := New(dir, slog.Default())
	writer.Append(telemetry)
	content, err := os.ReadFile(writer.path)
	require.NoError(t, err)
	require.Contains(t, string(content), `"provider":"openai"`)
	require.Contains(t, string(content), `"model":"gpt-4"`)
	require.Contains(t, string(content), `"attempt":2`)
	require.Contains(t, string(content), `"retry_count":1`)
	require.Contains(t, string(content), `"total_us":200000`)
	require.Contains(t, string(content), `"cache_status":"hit"`)
	require.Contains(t, string(content), `"prefix_changed_reason":"model_switch"`)
	require.NotContains(t, string(content), "req-secret")
}

func TestRejectedTelemetryNeverEchoesInput(t *testing.T) {
	const canary = "SYNTHETIC_SECRET_CANARY"
	for _, mutate := range []func(*crushapi.RunTelemetry){
		func(v *crushapi.RunTelemetry) { v.CacheStatus = canary },
		func(v *crushapi.RunTelemetry) { v.PrefixChangedReason = canary },
		func(v *crushapi.RunTelemetry) { v.SpansMicros = map[string]int64{canary: -1} },
		func(v *crushapi.RunTelemetry) { v.StablePrefixHMAC = canary },
		func(v *crushapi.RunTelemetry) { v.ReasoningEffort = canary },
	} {
		var logs bytes.Buffer
		writer := New(t.TempDir(), slog.New(slog.NewJSONHandler(&logs, nil)))
		value := &crushapi.RunTelemetry{CacheStatus: "unreported"}
		mutate(value)
		require.Error(t, Validate(value))
		writer.Append(value)
		require.NotContains(t, logs.String(), canary)
		require.Contains(t, logs.String(), "rejected invalid telemetry")
		_, err := os.Stat(writer.path)
		require.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestRedactionOwnsNestedDataAndPreservesAbsence(t *testing.T) {
	zero := int64(0)
	original := &crushapi.RunTelemetry{CacheStatus: "miss", SpansMicros: map[string]int64{"stream": 7}, CachedInputTokens: &zero}
	copy := redactSensitive(original)
	copy.SpansMicros["stream"] = 9
	*copy.CachedInputTokens = 12
	require.Equal(t, int64(7), original.SpansMicros["stream"])
	require.Zero(t, *original.CachedInputTokens)
	require.Nil(t, copy.UncachedInputTokens)
	encoded, err := json.Marshal(copy)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "uncached_input_tokens")
}
