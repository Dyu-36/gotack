package runmetrics

import (
	"github.com/Dyu-36/gotack/internal/engineapi"
	"testing"
)

func TestProviderAttemptIdentityCannotBeAmbiguous(t *testing.T) {
	tests := []struct {
		name      string
		attempts  []engineapi.ProviderAttemptTelemetry
		wantError bool
	}{
		{"duplicate", []engineapi.ProviderAttemptTelemetry{{ModelCallID: 1, HTTPAttempt: 1, Purpose: "tool_loop"}, {ModelCallID: 1, HTTPAttempt: 1, Purpose: "tool_loop"}}, true},
		{"duplicate_with_different_purpose", []engineapi.ProviderAttemptTelemetry{{ModelCallID: 1, HTTPAttempt: 1, Purpose: "tool_loop"}, {ModelCallID: 1, HTTPAttempt: 1, Purpose: "summarize"}}, true},
		{"two_calls", []engineapi.ProviderAttemptTelemetry{{ModelCallID: 1, HTTPAttempt: 1, Purpose: "tool_loop"}, {ModelCallID: 2, HTTPAttempt: 1, Purpose: "summarize"}}, false},
		{"retry", []engineapi.ProviderAttemptTelemetry{{ModelCallID: 1, HTTPAttempt: 1, Purpose: "tool_loop"}, {ModelCallID: 1, HTTPAttempt: 2, Purpose: "retry"}}, false},
		{"missing_observations", []engineapi.ProviderAttemptTelemetry{{ModelCallID: 1, HTTPAttempt: 1, Purpose: "tool_loop"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&engineapi.RunTelemetry{RunID: "root", CacheStatus: "unreported", ProviderAttempts: tt.attempts})
			if (err != nil) != tt.wantError {
				t.Fatal("provider_attempt_identity_validation_mismatch")
			}
		})
	}
}
