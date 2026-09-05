package runmetrics

import (
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func BenchmarkValidate(b *testing.B) {
	telemetry := &crushapi.RunTelemetry{
		RunID:               "bench-run-001",
		Provider:            "openai",
		Model:               "gpt-4",
		ReasoningEffort:     "high",
		Attempt:             1,
		RetryCount:          0,
		TotalMicros:         100000,
		CacheStatus:         "unreported",
		PrefixChangedReason: "none",
		SpansMicros:         map[string]int64{"stream": 50000},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(telemetry)
	}
}

func BenchmarkRedactSensitive(b *testing.B) {
	telemetry := &crushapi.RunTelemetry{
		RunID:             "bench-run-002",
		Provider:          "openai",
		Model:             "gpt-4",
		ProviderRequestID: "req-abc-123",
		CacheStatus:       "hit",
		SpansMicros:       map[string]int64{"stream": 50000},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = redactSensitive(telemetry)
	}
}
